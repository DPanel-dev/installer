package core

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/dpanel-dev/installer/internal/types"
	dockerpkg "github.com/dpanel-dev/installer/pkg/docker"
	containerapi "github.com/moby/moby/api/types/container"
	networkapi "github.com/moby/moby/api/types/network"
	dockerclient "github.com/moby/moby/client"
)

func (e *Engine) backupContainer() error {
	if !e.Config.UpgradeBackup {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cli := e.Config.Client.Client

	containerID, err := e.findContainerID(ctx, cli, e.Config.ContainerName)
	if err != nil {
		return err
	}
	if containerID == "" {
		return nil
	}

	backupName := fmt.Sprintf("%s-backup-%s", e.Config.ContainerName, time.Now().Format("20060102150405"))
	if _, err := cli.ContainerStop(ctx, containerID, dockerclient.ContainerStopOptions{}); err != nil {
		return err
	}
	if _, err := cli.ContainerRename(ctx, containerID, dockerclient.ContainerRenameOptions{NewName: backupName}); err != nil {
		return err
	}

	slog.Info("Container backup created", "source", e.Config.ContainerName, "backup", backupName)
	return nil
}

func (e *Engine) installContainer() error {
	if err := os.MkdirAll(e.Config.DataPath, 0755); err != nil {
		return fmt.Errorf("create data path failed: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cli := e.Config.Client.Client

	if err := e.pullImage(ctx, cli, e.Config.GetImageName()); err != nil {
		return err
	}

	containerID, err := e.findContainerID(ctx, cli, e.Config.ContainerName)
	if err != nil {
		return err
	}
	if containerID != "" {
		if _, err := cli.ContainerRemove(ctx, containerID, dockerclient.ContainerRemoveOptions{Force: true}); err != nil {
			return err
		}
	}

	createOpts, err := e.containerCreateOptions()
	if err != nil {
		return err
	}

	created, err := cli.ContainerCreate(ctx, createOpts)
	if err != nil {
		return err
	}

	if _, err := cli.ContainerStart(ctx, created.ID, dockerclient.ContainerStartOptions{}); err != nil {
		return err
	}

	slog.Info("Container installed", "container_id", created.ID, "container_name", e.Config.ContainerName)
	return nil
}

func (e *Engine) uninstallContainer() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cli := e.Config.Client.Client

	containerID, err := e.findContainerID(ctx, cli, e.Config.ContainerName)
	if err != nil {
		return err
	}
	if containerID != "" {
		if _, err := cli.ContainerRemove(ctx, containerID, dockerclient.ContainerRemoveOptions{Force: true}); err != nil {
			return err
		}
	}

	if e.Config.UninstallRemoveData && e.Config.DataPath != "" {
		if err := os.RemoveAll(e.Config.DataPath); err != nil {
			return fmt.Errorf("remove data path failed: %w", err)
		}
	}

	return nil
}

func (e *Engine) findContainerID(ctx context.Context, cli *dockerclient.Client, name string) (string, error) {
	result, err := cli.ContainerList(ctx, dockerclient.ContainerListOptions{All: true})
	if err != nil {
		return "", fmt.Errorf("list containers failed: %w", err)
	}

	for _, item := range result.Items {
		for _, n := range item.Names {
			if strings.TrimPrefix(n, "/") == name {
				return item.ID, nil
			}
		}
	}

	return "", nil
}

func (e *Engine) pullImage(ctx context.Context, cli *dockerclient.Client, image string) error {
	resp, err := cli.ImagePull(ctx, image, dockerclient.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("pull image failed: %w", err)
	}
	defer resp.Close()

	if err := resp.Wait(ctx); err != nil {
		return fmt.Errorf("wait image pull failed: %w", err)
	}
	return nil
}

func (e *Engine) containerCreateOptions() (dockerclient.ContainerCreateOptions, error) {
	exposedPorts := networkapi.PortSet{}
	portBindings := networkapi.PortMap{}

	addPortBinding := func(hostPort, containerPort string) error {
		port, err := networkapi.ParsePort(containerPort + "/tcp")
		if err != nil {
			return err
		}
		exposedPorts[port] = struct{}{}
		portBindings[port] = []networkapi.PortBinding{{HostIP: netip.Addr{}, HostPort: hostPort}}
		return nil
	}

	if e.Config.Edition == types.EditionStandard {
		if err := addPortBinding("80", "80"); err != nil {
			return dockerclient.ContainerCreateOptions{}, err
		}
		if err := addPortBinding("443", "443"); err != nil {
			return dockerclient.ContainerCreateOptions{}, err
		}
	}
	if e.Config.Port > 0 {
		if err := addPortBinding(fmt.Sprintf("%d", e.Config.Port), "8080"); err != nil {
			return dockerclient.ContainerCreateOptions{}, err
		}
	} else {
		port, err := networkapi.ParsePort("8080/tcp")
		if err != nil {
			return dockerclient.ContainerCreateOptions{}, err
		}
		exposedPorts[port] = struct{}{}
	}

	env := []string{fmt.Sprintf("APP_NAME=%s", e.Config.ContainerName)}
	if e.Config.HTTPProxy != "" {
		env = append(env, fmt.Sprintf("HTTP_PROXY=%s", e.Config.HTTPProxy))
	}
	if e.Config.HTTPSProxy != "" {
		env = append(env, fmt.Sprintf("HTTPS_PROXY=%s", e.Config.HTTPSProxy))
	}

	hostConfig := &containerapi.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/var/run/docker.sock", e.currentSockPath()),
			fmt.Sprintf("%s:/dpanel", e.Config.DataPath),
		},
		PortBindings: portBindings,
		RestartPolicy: containerapi.RestartPolicy{
			Name: "always",
		},
		LogConfig: containerapi.LogConfig{
			Type: "json-file",
			Config: map[string]string{
				"max-size": "5m",
				"max-file": "10",
			},
		},
	}
	if e.Config.DNS != "" {
		addr, err := netip.ParseAddr(e.Config.DNS)
		if err != nil {
			return dockerclient.ContainerCreateOptions{}, fmt.Errorf("invalid dns address: %w", err)
		}
		hostConfig.DNS = []netip.Addr{addr}
	}
	if !e.isPodmanClient() {
		hostConfig.ExtraHosts = []string{"host.dpanel.local:host-gateway"}
	}

	return dockerclient.ContainerCreateOptions{
		Config: &containerapi.Config{
			Image:        e.Config.GetImageName(),
			Hostname:     fmt.Sprintf("%s.pod.dpanel.local", e.Config.ContainerName),
			Env:          env,
			ExposedPorts: networkapi.PortSet(exposedPorts),
		},
		HostConfig: hostConfig,
		Name:       e.Config.ContainerName,
	}, nil
}

func (e *Engine) isPodmanClient() bool {
	if e.Config.Client == nil || e.Config.Client.Client == nil {
		return false
	}
	return strings.Contains(e.Config.Client.Client.DaemonHost(), "podman")
}

func (e *Engine) currentSockPath() string {
	if e.Config.Client != nil && e.Config.Client.Client != nil {
		if sockPath := dockerpkg.SockPathFromHost(e.Config.Client.Client.DaemonHost()); sockPath != "" {
			return sockPath
		}
	}
	return "/var/run/docker.sock"
}
