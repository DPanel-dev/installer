package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func NormalizeHost(host string) string {
	if host == "" {
		return ""
	}
	if strings.HasPrefix(host, "unix://") || strings.HasPrefix(host, "npipe://") {
		return host
	}
	return "unix://" + host
}

func SockPathFromHost(host string) string {
	switch {
	case strings.HasPrefix(host, "unix://"):
		return strings.TrimPrefix(host, "unix://")
	case strings.HasPrefix(host, "npipe://"):
		return host
	default:
		return ""
	}
}

func dockerHosts() []string {
	hosts := make([]string, 0, 6)
	seen := make(map[string]struct{})
	addHost := func(host string) {
		if host == "" {
			return
		}
		if _, ok := seen[host]; ok {
			return
		}
		seen[host] = struct{}{}
		hosts = append(hosts, host)
	}

	if host := os.Getenv("CONTAINER_HOST"); host != "" {
		addHost(host)
	}
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		addHost(host)
	}

	switch runtime.GOOS {
	case "linux":
		addHost("unix:///var/run/docker.sock")
		if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
			addHost("unix://" + filepath.Join(dir, "docker.sock"))
		}
		if uid := os.Getuid(); uid > 0 {
			addHost(fmt.Sprintf("unix:///run/user/%d/docker.sock", uid))
		}
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			addHost("unix://" + filepath.Join(home, ".docker", "run", "docker.sock"))
		}
	case "windows":
		addHost("npipe:////./pipe/docker_engine")
	}

	return hosts
}

func podmanHosts() []string {
	hosts := make([]string, 0, 6)
	seen := make(map[string]struct{})
	addHost := func(host string) {
		if host == "" {
			return
		}
		if _, ok := seen[host]; ok {
			return
		}
		seen[host] = struct{}{}
		hosts = append(hosts, host)
	}

	if host := os.Getenv("CONTAINER_HOST"); host != "" {
		addHost(host)
	}
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		addHost(host)
	}

	switch runtime.GOOS {
	case "linux":
		if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
			addHost("unix://" + filepath.Join(dir, "podman", "podman.sock"))
		}
		if uid := os.Getuid(); uid > 0 {
			addHost(fmt.Sprintf("unix:///run/user/%d/podman/podman.sock", uid))
		}
		addHost("unix:///run/podman/podman.sock")
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			addHost("unix://" + filepath.Join(home, ".local", "share", "containers", "podman", "machine", "podman.sock"))
			addHost("unix://" + filepath.Join(home, ".local", "share", "containers", "podman", "machine", "default", "podman.sock"))
		}
	}

	return hosts
}
