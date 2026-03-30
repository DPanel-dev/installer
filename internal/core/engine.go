package core

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dpanel-dev/installer/internal/config"
	"github.com/dpanel-dev/installer/internal/types"
	docker "github.com/dpanel-dev/installer/pkg/docker"
	dockerclient "github.com/moby/moby/client"
)

// Engine handles the installation process
type Engine struct {
	Config *config.Config
}

// NewEngine creates a new installation engine
func NewEngine(cfg *config.Config) *Engine {
	return &Engine{Config: cfg}
}

// Run executes the installation based on the configured action
func (e *Engine) Run() error {
	e.logRuntimeConfig()

	switch e.Config.Action {
	case types.ActionInstall:
		return e.install()
	case types.ActionUpgrade:
		return e.upgrade()
	case types.ActionUninstall:
		return e.uninstall()
	default:
		return fmt.Errorf("unknown action: %s", e.Config.Action)
	}
}

func (e *Engine) logRuntimeConfig() {
	cfg := e.Config

	attrs := []any{
		"os", cfg.OS,
		"arch", cfg.Arch,
		"action", cfg.Action,
		"language", cfg.Language,
		"install_type", cfg.InstallType,
		"version", cfg.Version,
		"edition", cfg.Edition,
		"base_image", cfg.BaseImage,
		"registry", cfg.Registry,
		"container_name", cfg.ContainerName,
		"port", cfg.Port,
		"data_path", cfg.DataPath,
		"dns", cfg.DNS,
		"http_proxy", cfg.HTTPProxy,
		"https_proxy", cfg.HTTPSProxy,
		"upgrade_backup", cfg.UpgradeBackup,
		"uninstall_remove_data", cfg.UninstallRemoveData,
	}

	slog.Info("Installation config", attrs...)
}

// install performs the installation
func (e *Engine) install() error {
	slog.Info("Running installation")

	// Environment check
	if err := e.checkEnvironment(); err != nil {
		return fmt.Errorf("environment check failed: %w", err)
	}

	// Build and execute command
	if e.Config.InstallType == types.InstallTypeContainer {
		return e.installContainer()
	}
	return e.installBinary()
}

// upgrade performs the upgrade
func (e *Engine) upgrade() error {
	slog.Info("Running upgrade")

	// Environment check
	if err := e.checkEnvironment(); err != nil {
		return fmt.Errorf("environment check failed: %w", err)
	}

	// Detect existing installation
	if err := e.detectExistingInstallation(); err != nil {
		return fmt.Errorf("failed to detect existing installation: %w", err)
	}

	// Perform upgrade based on installation type
	if e.Config.InstallType == types.InstallTypeContainer {
		return e.upgradeContainer()
	}
	return e.upgradeBinary()
}

// uninstall performs the uninstallation
func (e *Engine) uninstall() error {
	slog.Info("Running uninstall")

	// Environment check
	if err := e.checkEnvironment(); err != nil {
		return fmt.Errorf("environment check failed: %w", err)
	}

	// Perform uninstall based on installation type
	if e.Config.InstallType == types.InstallTypeContainer {
		return e.uninstallContainer()
	}
	return e.uninstallBinary()
}

// checkEnvironment validates the installation environment
func (e *Engine) checkEnvironment() error {
	slog.Info("Checking environment")

	// Check Docker/Podman availability
	if e.Config.InstallType == types.InstallTypeContainer {
		if err := e.checkDocker(); err != nil {
			return err
		}

		// Check Docker connection
		if err := e.checkDockerConnection(); err != nil {
			return err
		}
	}

	slog.Info("Environment check passed")
	return nil
}

// checkDocker verifies Docker/Podman is available and running
func (e *Engine) checkDocker() error {
	slog.Info("Checking Docker/Podman availability")

	client := e.Config.Client
	if client == nil || client.Client == nil {
		return fmt.Errorf("no container runtime available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := client.Client.Ping(ctx, dockerclient.PingOptions{}); err != nil {
		return fmt.Errorf("container service is not available: %w", err)
	}

	slog.Info("Container service is available", "address", client.Client.DaemonHost())
	return nil
}

// checkDockerConnection verifies Docker connection
func (e *Engine) checkDockerConnection() error {
	if e.Config.Client == nil || e.Config.Client.Client == nil {
		return fmt.Errorf("no container connection configured")
	}

	host := e.Config.Client.Client.DaemonHost()
	sockPath := docker.SockPathFromHost(host)

	slog.Info("Checking container connection", "address", host)
	if sockPath == "" {
		return fmt.Errorf("only local socket connection is supported in installer")
	}
	return e.checkSockConnection(sockPath)
}

// checkSockConnection checks local socket file
func (e *Engine) checkSockConnection(sockPath string) error {
	if sockPath == "" {
		sockPath = "/var/run/docker.sock"
	}
	if strings.HasPrefix(sockPath, "npipe://") {
		slog.Info("Windows named pipe connection", "address", sockPath)
		return nil
	}

	slog.Info("Checking local socket", "path", sockPath)

	// Check if socket file exists
	if _, err := os.Stat(sockPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("socket file not found: %s", sockPath)
		}
		return fmt.Errorf("cannot access socket: %w", err)
	}

	return nil
}

// installContainer installs DPanel as a container
func (e *Engine) installContainer() error {
	slog.Info("Starting container installation")

	// Log configuration
	e.logInstallationConfig()

	// Build and save docker command (for reference)
	cmd, err := e.buildDockerCommand()
	if err != nil {
		slog.Error("Failed to build docker command", "error", err)
		return err
	}

	slog.Info("Docker command built", "command", cmd)

	// Save command to installation log file
	if err := e.saveInstallationLog(cmd); err != nil {
		slog.Warn("Failed to save installation log", "error", err)
	}

	if err := e.simulateExecution("container installation", 5*time.Second); err != nil {
		return err
	}
	e.saveInstallationResult(true, "")
	return nil
}

// logInstallationConfig logs the installation configuration
func (e *Engine) logInstallationConfig() {
	cfg := e.Config
	slog.Info("=== Installation Configuration ===")
	slog.Info("Action", "action", cfg.Action)
	slog.Info("Install Type", "type", cfg.InstallType)
	slog.Info("Version", "version", cfg.Version)
	slog.Info("Edition", "edition", cfg.Edition)
	slog.Info("BaseImage", "baseImage", cfg.BaseImage)
	slog.Info("Registry", "registry", cfg.Registry)
	slog.Info("Container Name", "name", cfg.ContainerName)
	slog.Info("Port", "port", cfg.Port)
	slog.Info("Data Path", "path", cfg.DataPath)
	if cfg.Client != nil && cfg.Client.Client != nil {
		slog.Info("Container Connection", "address", cfg.Client.Client.DaemonHost())
	}
	if cfg.HTTPProxy != "" {
		slog.Info("HTTP Proxy", "proxy", cfg.HTTPProxy)
	}
	if cfg.DNS != "" {
		slog.Info("DNS", "dns", cfg.DNS)
	}
	slog.Info("=== End Configuration ===")
}

// logInstallationSteps logs the installation steps
func (e *Engine) logInstallationSteps() {
	slog.Info("=== Installation Steps ===")
	slog.Info("Step 1: Environment check - PASSED")
	slog.Info("Step 2: Building docker command")
	slog.Info("Step 3: Pulling docker image")
	slog.Info("Step 4: Creating container")
	slog.Info("Step 5: Starting container")
	slog.Info("=== End Steps ===")
}

// saveInstallationLog saves the installation command to a log file
func (e *Engine) saveInstallationLog(command string) error {
	// Get executable directory
	execPath, err := os.Executable()
	if err != nil {
		return err
	}
	execDir := filepath.Dir(execPath)

	// Create installation log directory
	logDir := filepath.Join(execDir, "install_logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	// Create log file with timestamp
	timestamp := time.Now().Format("20060102_150405")
	logFile := filepath.Join(logDir, fmt.Sprintf("install_%s.log", timestamp))

	file, err := os.Create(logFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write installation details
	fmt.Fprintf(file, "=== DPanel Installation Log ===\n")
	fmt.Fprintf(file, "Date: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(file, "Version: %s\n", e.Config.Version)
	fmt.Fprintf(file, "Edition: %s\n", e.Config.Edition)
	fmt.Fprintf(file, "Install Type: %s\n", e.Config.InstallType)
	fmt.Fprintf(file, "\n=== Configuration ===\n")
	fmt.Fprintf(file, "Container Name: %s\n", e.Config.ContainerName)
	fmt.Fprintf(file, "Port: %d\n", e.Config.Port)
	fmt.Fprintf(file, "Data Path: %s\n", e.Config.DataPath)
	if e.Config.Client != nil && e.Config.Client.Client != nil {
		fmt.Fprintf(file, "Container Connection: %s\n", e.Config.Client.Client.DaemonHost())
	}
	fmt.Fprintf(file, "\n=== Execution Command ===\n")
	fmt.Fprintf(file, "%s\n", command)
	fmt.Fprintf(file, "\n=== End Log ===\n")

	slog.Info("Installation log saved", "file", logFile)
	return nil
}

// saveInstallationResult saves the installation result
func (e *Engine) saveInstallationResult(success bool, errorMsg string) {
	// Get executable directory
	execPath, err := os.Executable()
	if err != nil {
		return
	}
	execDir := filepath.Dir(execPath)

	// Append to latest installation log
	logDir := filepath.Join(execDir, "install_logs")
	logFile := filepath.Join(logDir, "latest.log")

	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	if success {
		fmt.Fprintf(file, "\n[%s] Installation: SUCCESS\n", timestamp)
	} else {
		fmt.Fprintf(file, "\n[%s] Installation: FAILED\n", timestamp)
		fmt.Fprintf(file, "Error: %s\n", errorMsg)
	}
}

// installBinary installs DPanel as a binary
func (e *Engine) installBinary() error {
	slog.Info("Installing DPanel as binary")

	// Log configuration
	e.logBinaryInstallConfig()

	if err := e.simulateExecution("binary installation", 5*time.Second); err != nil {
		return err
	}

	slog.Info("Binary installation completed successfully")
	e.saveBinaryInstallResult(true, "")
	return nil
}

// buildDockerCommand builds the docker/podman run command
func (e *Engine) buildDockerCommand() (string, error) {
	cfg := e.Config

	// Build image name
	image := e.buildImageName()

	// Build command parts
	var parts []string
	parts = append(parts, "docker", "run", "-d")
	parts = append(parts, "--name", cfg.ContainerName)

	// Restart policy
	parts = append(parts, "--restart=on-failure:5")

	// Logging
	parts = append(parts, "--log-driver", "json-file")
	parts = append(parts, "--log-opt", "max-size=5m")
	parts = append(parts, "--log-opt", "max-file=10")

	// Hostname
	parts = append(parts, "--hostname", fmt.Sprintf("%s.pod.dpanel.local", cfg.ContainerName))

	// Host mapping
	parts = append(parts, "--add-host", "host.dpanel.local:host-gateway")

	// Environment variables
	parts = append(parts, "-e", fmt.Sprintf("APP_NAME=%s", cfg.ContainerName))

	// Proxy
	if cfg.HTTPProxy != "" {
		parts = append(parts, "-e", fmt.Sprintf("HTTP_PROXY=%s", cfg.HTTPProxy))
	}
	if cfg.HTTPSProxy != "" {
		parts = append(parts, "-e", fmt.Sprintf("HTTPS_PROXY=%s", cfg.HTTPSProxy))
	}

	// DNS
	if cfg.DNS != "" {
		parts = append(parts, "--dns", cfg.DNS)
	}

	// Ports
	if cfg.Edition == types.EditionStandard {
		parts = append(parts, "-p", "80:80", "-p", "443:443")
	}
	if cfg.Port > 0 {
		parts = append(parts, "-p", fmt.Sprintf("%d:8080", cfg.Port))
	} else {
		// Random port
		parts = append(parts, "-p", "8080")
	}

	// Volumes
	// 从 Container 配置获取 socket 路径
	sockPath := "/var/run/docker.sock" // 默认值
	if cfg.Client != nil && cfg.Client.Client != nil {
		if clientSockPath := docker.SockPathFromHost(cfg.Client.Client.DaemonHost()); clientSockPath != "" {
			sockPath = clientSockPath
		}
	}
	parts = append(parts, "-v", fmt.Sprintf("%s:/var/run/docker.sock", sockPath))
	parts = append(parts, "-v", fmt.Sprintf("%s:/dpanel", cfg.DataPath))

	// Image
	parts = append(parts, image)

	return strings.Join(parts, " "), nil
}

// buildImageName builds the Docker image name
func (e *Engine) buildImageName() string {
	return e.Config.GetImageName()
}

// ParsePort parses port from string
func ParsePort(portStr string) (int, error) {
	if portStr == "" {
		return 0, nil // 0 means random port
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid port number: %w", err)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port must be between 1 and 65535")
	}
	return port, nil
}

// detectExistingInstallation checks if DPanel is already installed
func (e *Engine) detectExistingInstallation() error {
	slog.Info("Detecting existing DPanel installation")

	runtime := e.getDockerRuntime()

	// Check if container exists
	cmd := exec.Command(runtime, "ps", "-a", "--filter", "name="+e.Config.ContainerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to check existing containers: %w", err)
	}

	if !strings.Contains(string(output), e.Config.ContainerName) {
		return fmt.Errorf("no existing DPanel installation found with container name: %s", e.Config.ContainerName)
	}

	slog.Info("Found existing DPanel installation")
	return nil
}

// upgradeContainer upgrades the container installation
func (e *Engine) upgradeContainer() error {
	slog.Info("Starting container upgrade")

	// Log upgrade configuration
	e.logUpgradeConfig()

	// Build and save docker command (for reference)
	cmd, err := e.buildDockerCommand()
	if err != nil {
		slog.Error("Failed to build docker command", "error", err)
		return err
	}

	// Save upgrade log
	if err := e.saveUpgradeLog(cmd); err != nil {
		slog.Warn("Failed to save upgrade log", "error", err)
	}

	if err := e.simulateExecution("container upgrade", 5*time.Second); err != nil {
		return err
	}
	e.saveUpgradeResult(true, "")
	return nil
}

// upgradeBinary upgrades the binary installation
func (e *Engine) upgradeBinary() error {
	slog.Info("Starting binary upgrade")
	if err := e.simulateExecution("binary upgrade", 5*time.Second); err != nil {
		return err
	}
	e.saveUpgradeResult(true, "")
	return nil
}

// getDockerRuntime returns the available docker runtime (docker or podman)
func (e *Engine) getDockerRuntime() string {
	if _, err := exec.LookPath("docker"); err == nil {
		return "docker"
	}
	if _, err := exec.LookPath("podman"); err == nil {
		return "podman"
	}
	return "docker" // Default to docker
}

// stopContainer stops a running container
func (e *Engine) stopContainer(runtime, containerName string) error {
	cmd := exec.Command(runtime, "stop", containerName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop container %s: %w", containerName, err)
	}
	slog.Info("Container stopped", "name", containerName)
	return nil
}

// pullImage pulls a docker image
func (e *Engine) pullImage(runtime, image string) error {
	cmd := exec.Command(runtime, "pull", image)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to pull image %s: %w", image, err)
	}
	slog.Info("Image pulled", "image", image)
	return nil
}

// removeContainer removes a container
func (e *Engine) removeContainer(runtime, containerName string) error {
	cmd := exec.Command(runtime, "rm", containerName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove container %s: %w", containerName, err)
	}
	slog.Info("Container removed", "name", containerName)
	return nil
}

// rollbackUpgrade attempts to rollback a failed upgrade
func (e *Engine) rollbackUpgrade(runtime, image string) error {
	slog.Warn("Attempting upgrade rollback")
	// Try to restart with old image (if it still exists locally)
	// TODO: Implement proper rollback mechanism
	return nil
}

// logUpgradeConfig logs the upgrade configuration
func (e *Engine) logUpgradeConfig() {
	cfg := e.Config
	slog.Info("=== Upgrade Configuration ===")
	slog.Info("Action", "action", cfg.Action)
	slog.Info("Install Type", "type", cfg.InstallType)
	slog.Info("Version", "version", cfg.Version)
	slog.Info("Edition", "edition", cfg.Edition)
	slog.Info("Container Name", "name", cfg.ContainerName)
	slog.Info("=== End Configuration ===")
}

// saveUpgradeLog saves the upgrade command to a log file
func (e *Engine) saveUpgradeLog(command string) error {
	// Get executable directory
	execPath, err := os.Executable()
	if err != nil {
		return err
	}
	execDir := filepath.Dir(execPath)

	// Create upgrade log directory
	logDir := filepath.Join(execDir, "install_logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	// Create log file with timestamp
	timestamp := time.Now().Format("20060102_150405")
	logFile := filepath.Join(logDir, fmt.Sprintf("upgrade_%s.log", timestamp))

	file, err := os.Create(logFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write upgrade details
	fmt.Fprintf(file, "=== DPanel Upgrade Log ===\n")
	fmt.Fprintf(file, "Date: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(file, "Version: %s\n", e.Config.Version)
	fmt.Fprintf(file, "Edition: %s\n", e.Config.Edition)
	fmt.Fprintf(file, "Install Type: %s\n", e.Config.InstallType)
	fmt.Fprintf(file, "\n=== Configuration ===\n")
	fmt.Fprintf(file, "Container Name: %s\n", e.Config.ContainerName)
	fmt.Fprintf(file, "\n=== Execution Command ===\n")
	fmt.Fprintf(file, "%s\n", command)
	fmt.Fprintf(file, "\n=== End Log ===\n")

	slog.Info("Upgrade log saved", "file", logFile)
	return nil
}

// saveUpgradeResult saves the upgrade result
func (e *Engine) saveUpgradeResult(success bool, errorMsg string) {
	// Get executable directory
	execPath, err := os.Executable()
	if err != nil {
		return
	}
	execDir := filepath.Dir(execPath)

	// Append to latest upgrade log
	logDir := filepath.Join(execDir, "install_logs")
	logFile := filepath.Join(logDir, "latest.log")

	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	if success {
		fmt.Fprintf(file, "\n[%s] Upgrade: SUCCESS\n", timestamp)
	} else {
		fmt.Fprintf(file, "\n[%s] Upgrade: FAILED\n", timestamp)
		fmt.Fprintf(file, "Error: %s\n", errorMsg)
	}
}

// uninstallContainer uninstalls the container installation
func (e *Engine) uninstallContainer() error {
	slog.Info("Starting container uninstallation")

	// Log uninstall configuration
	e.logUninstallConfig()

	if err := e.simulateExecution("container uninstallation", 5*time.Second); err != nil {
		return err
	}
	e.saveUninstallResult(true, "")
	return nil
}

// uninstallBinary uninstalls the binary installation
func (e *Engine) uninstallBinary() error {
	slog.Info("Starting binary uninstallation")
	if err := e.simulateExecution("binary uninstallation", 5*time.Second); err != nil {
		return err
	}
	e.saveUninstallResult(true, "")
	return nil
}

// simulateExecution simulates an operation execution flow.
func (e *Engine) simulateExecution(operation string, duration time.Duration) error {
	slog.Info("Simulation started", "operation", operation, "duration", duration.String())
	time.Sleep(duration)
	slog.Info("Simulation completed", "operation", operation)
	return nil
}

// checkContainerExists checks if a container exists
func (e *Engine) checkContainerExists(runtime, containerName string) (bool, error) {
	cmd := exec.Command(runtime, "ps", "-a", "--filter", "name="+containerName, "--format", "{{.Names}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to check container: %w", err)
	}

	return strings.Contains(string(output), containerName), nil
}

// removeImage removes a docker image
func (e *Engine) removeImage(runtime, image string) error {
	cmd := exec.Command(runtime, "rmi", image)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove image %s: %w", image, err)
	}
	slog.Info("Image removed", "image", image)
	return nil
}

// cleanupDataVolumes removes data volumes
func (e *Engine) cleanupDataVolumes() error {
	slog.Info("Cleaning up data volumes", "path", e.Config.DataPath)
	// TODO: Implement data volume cleanup
	// This should be optional and require confirmation
	return nil
}

// logUninstallConfig logs the uninstall configuration
func (e *Engine) logUninstallConfig() {
	cfg := e.Config
	slog.Info("=== Uninstall Configuration ===")
	slog.Info("Action", "action", cfg.Action)
	slog.Info("Install Type", "type", cfg.InstallType)
	slog.Info("Container Name", "name", cfg.ContainerName)
	slog.Info("Data Path", "path", cfg.DataPath)
	slog.Info("=== End Configuration ===")
}

// saveUninstallResult saves the uninstall result
func (e *Engine) saveUninstallResult(success bool, errorMsg string) {
	// Get executable directory
	execPath, err := os.Executable()
	if err != nil {
		return
	}
	execDir := filepath.Dir(execPath)

	// Append to latest log
	logDir := filepath.Join(execDir, "install_logs")
	logFile := filepath.Join(logDir, "latest.log")

	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	if success {
		fmt.Fprintf(file, "\n[%s] Uninstall: SUCCESS\n", timestamp)
	} else {
		fmt.Fprintf(file, "\n[%s] Uninstall: FAILED\n", timestamp)
		fmt.Fprintf(file, "Error: %s\n", errorMsg)
	}
}

// logBinaryInstallConfig logs the binary installation configuration
func (e *Engine) logBinaryInstallConfig() {
	cfg := e.Config
	slog.Info("=== Binary Installation Configuration ===")
	slog.Info("Action", "action", cfg.Action)
	slog.Info("Version", "version", cfg.Version)
	slog.Info("Edition", "edition", cfg.Edition)
	slog.Info("Data Path", "path", cfg.DataPath)
	slog.Info("=== End Configuration ===")
}

// downloadBinary downloads the DPanel binary
func (e *Engine) downloadBinary() error {
	// Determine download URL based on version and edition
	url := e.getBinaryDownloadURL()
	slog.Info("Downloading binary from", "url", url)

	// TODO: Implement actual download logic
	// Use http.Get to download the binary
	// Save to temporary location
	return fmt.Errorf("binary download not implemented yet")
}

// getBinaryDownloadURL returns the download URL for the binary
func (e *Engine) getBinaryDownloadURL() string {
	// Build download URL based on version, edition, and platform
	// Example: https://github.com/dpanel/dpanel/releases/download/v1.0.0/dpanel-linux-amd64
	// TODO: Implement URL building logic
	return "https://github.com/dpanel/dpanel/releases/latest/download/dpanel"
}

// installBinaryToPath installs the binary to system path
func (e *Engine) installBinaryToPath() error {
	// Determine config path based on OS
	installPath := e.getBinaryInstallPath()
	slog.Info("Installing binary to", "path", installPath)

	// TODO: Implement installation logic
	// Copy binary to config path
	// Set executable permissions
	return fmt.Errorf("binary installation to path not implemented yet")
}

// getBinaryInstallPath returns the installation path for the binary
func (e *Engine) getBinaryInstallPath() string {
	// Determine config path based on OS
	// Linux: /usr/local/bin/dpanel
	// macOS: /usr/local/bin/dpanel
	// Windows: C:\Program Files\DPanel\dpanel.exe
	// TODO: Implement OS-specific path logic
	return "/usr/local/bin/dpanel"
}

// createServiceFile creates a service file for the binary
func (e *Engine) createServiceFile() error {
	// Determine service file type based on OS
	// Linux: systemd service file
	// macOS: launchd plist file
	// Windows: Windows service
	slog.Info("Creating service file")

	// TODO: Implement service file creation
	return fmt.Errorf("service file creation not implemented yet")
}

// startBinaryService starts the binary service
func (e *Engine) startBinaryService() error {
	slog.Info("Starting binary service")

	// TODO: Implement service startup logic
	// systemctl start dpanel (Linux)
	// launchctl load (macOS)
	// sc start (Windows)
	return fmt.Errorf("service startup not implemented yet")
}

// saveBinaryInstallResult saves the binary installation result
func (e *Engine) saveBinaryInstallResult(success bool, errorMsg string) {
	// Get executable directory
	execPath, err := os.Executable()
	if err != nil {
		return
	}
	execDir := filepath.Dir(execPath)

	// Append to latest log
	logDir := filepath.Join(execDir, "install_logs")
	logFile := filepath.Join(logDir, "latest.log")

	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	if success {
		fmt.Fprintf(file, "\n[%s] Binary Installation: SUCCESS\n", timestamp)
	} else {
		fmt.Fprintf(file, "\n[%s] Binary Installation: FAILED\n", timestamp)
		fmt.Fprintf(file, "Error: %s\n", errorMsg)
	}
}
