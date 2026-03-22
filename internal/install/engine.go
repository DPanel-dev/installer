package install

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Engine handles the installation process
type Engine struct {
	Config *Config
}

// NewEngine creates a new installation engine
func NewEngine(cfg *Config) *Engine {
	return &Engine{Config: cfg}
}

// Run executes the installation based on the configured action
func (e *Engine) Run() error {
	slog.Info("Starting installation engine",
		"action", e.Config.Action,
		"installType", e.Config.InstallType,
		"version", e.Config.Version,
	)

	switch e.Config.Action {
	case "install":
		return e.install()
	case "upgrade":
		return e.upgrade()
	case "uninstall":
		return e.uninstall()
	default:
		return fmt.Errorf("unknown action: %s", e.Config.Action)
	}
}

// install performs the installation
func (e *Engine) install() error {
	slog.Info("Running installation")

	// Environment check
	if err := e.checkEnvironment(); err != nil {
		return fmt.Errorf("environment check failed: %w", err)
	}

	// Build and execute command
	if e.Config.InstallType == "container" {
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
	if e.Config.InstallType == "container" {
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
	if e.Config.InstallType == "container" {
		return e.uninstallContainer()
	}
	return e.uninstallBinary()
}

// checkEnvironment validates the installation environment
func (e *Engine) checkEnvironment() error {
	slog.Info("Checking environment")

	// Check Docker/Podman availability
	if e.Config.InstallType == "container" {
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

	// Check if docker command exists
	dockerExists := false
	if _, err := exec.LookPath("docker"); err == nil {
		dockerExists = true
		slog.Info("Docker command found")
	}

	podmanExists := false
	if _, err := exec.LookPath("podman"); err == nil {
		podmanExists = true
		slog.Info("Podman command found")
	}

	if !dockerExists && !podmanExists {
		return fmt.Errorf("neither docker nor podman found in PATH")
	}

	// Test docker service if it's the configured type
	if e.Config.DockerConnection.Type == "local" {
		// Try docker first
		if dockerExists {
			slog.Info("Testing Docker service")
			if err := e.testDockerService("docker"); err != nil {
				slog.Warn("Docker service test failed", "error", err)
				// Try podman as fallback
				if podmanExists {
					slog.Info("Trying Podman service")
					if err := e.testDockerService("podman"); err != nil {
						return fmt.Errorf("neither Docker nor Podman service is available: %w", err)
					}
					slog.Info("Podman service is available")
					return nil
				}
				return fmt.Errorf("Docker service is not available: %w", err)
			}
			slog.Info("Docker service is available")
			return nil
		} else if podmanExists {
			// Only podman available
			slog.Info("Testing Podman service")
			if err := e.testDockerService("podman"); err != nil {
				return fmt.Errorf("Podman service is not available: %w", err)
			}
			slog.Info("Podman service is available")
			return nil
		}
	}

	return nil
}

// testDockerService tests if docker/podman service is actually running
func (e *Engine) testDockerService(runtime string) error {
	// Test with docker ps command with timeout
	cmd := exec.Command(runtime, "ps")

	// Create a timeout channel
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	// Wait for command or timeout
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("%s ps failed: %w", runtime, err)
		}
		return nil
	case <-time.After(5 * time.Second):
		// Timeout - try to kill the process
		_ = cmd.Process.Kill()
		return fmt.Errorf("%s ps timed out - service may not be running", runtime)
	}
}

// checkDockerConnection verifies Docker connection
func (e *Engine) checkDockerConnection() error {
	conn := e.Config.DockerConnection
	slog.Info("Checking docker connection", "type", conn.Type)

	switch conn.Type {
	case "local":
		return e.checkLocalConnection(conn)
	case "tcp":
		return e.checkTCPConnection(conn)
	case "ssh":
		return e.checkSSHConnection(conn)
	default:
		return fmt.Errorf("unknown docker connection type: %s", conn.Type)
	}
}

// checkLocalConnection checks local socket file
func (e *Engine) checkLocalConnection(conn *DockerConnection) error {
	sockPath := conn.SockPath
	if sockPath == "" {
		sockPath = "/var/run/docker.sock"
	}
	conn.SockPath = sockPath

	slog.Info("Checking local docker sock", "path", sockPath)

	// Check if socket file exists
	if _, err := os.Stat(sockPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("docker socket file not found: %s", sockPath)
		}
		return fmt.Errorf("cannot access docker socket: %w", err)
	}

	return nil
}

// checkTCPConnection checks TCP connectivity
func (e *Engine) checkTCPConnection(conn *DockerConnection) error {
	if conn.Host == "" {
		return fmt.Errorf("docker host is required for TCP connection")
	}

	slog.Info("Checking TCP connection", "host", conn.Host)

	// Test TCP connection by trying to execute docker command with -H flag
	runtime := e.getDockerRuntime()
	testCmd := exec.Command(runtime, "-H", conn.Host, "ps")
	output, err := testCmd.CombinedOutput()

	if err != nil {
		slog.Error("TCP connection test failed", "error", err, "output", string(output))
		return fmt.Errorf("TCP connection to Docker daemon failed: %w", err)
	}

	slog.Info("TCP connection successful", "host", conn.Host)
	return nil
}

// checkSSHConnection checks SSH connectivity
func (e *Engine) checkSSHConnection(conn *DockerConnection) error {
	if conn.Host == "" {
		return fmt.Errorf("docker host is required for SSH connection")
	}
	if conn.SSHUser == "" {
		return fmt.Errorf("SSH username is required for SSH connection")
	}

	slog.Info("Checking SSH connection", "host", conn.Host, "user", conn.SSHUser)

	// Test SSH connection by trying to execute docker command via SSH
	// Build SSH command: ssh user@host docker ps
	var sshCmd []string
	sshCmd = append(sshCmd, "ssh")

	// Add SSH options
	if conn.SSHKey != "" {
		sshCmd = append(sshCmd, "-i", conn.SSHKey)
	}
	sshCmd = append(sshCmd, "-o", "StrictHostKeyChecking=no")
	sshCmd = append(sshCmd, "-o", "UserKnownHostsFile=/dev/null")

	// Add host and user
	sshCmd = append(sshCmd, fmt.Sprintf("%s@%s", conn.SSHUser, conn.Host))

	// Add docker command
	sshCmd = append(sshCmd, "docker", "ps")

	// Execute SSH command
	cmd := exec.Command(sshCmd[0], sshCmd[1:]...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		slog.Error("SSH connection test failed", "error", err, "output", string(output))
		return fmt.Errorf("SSH connection to Docker daemon failed: %w", err)
	}

	slog.Info("SSH connection successful", "host", conn.Host)
	return nil
}

// installContainer installs DPanel as a container
func (e *Engine) installContainer() error {
	slog.Info("Starting container installation")

	// Log configuration
	e.logInstallationConfig()

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

	// Log detailed installation steps
	e.logInstallationSteps()

	if err := e.executeCommand(cmd); err != nil {
		slog.Error("Container installation failed", "error", err)
		return fmt.Errorf("container installation failed: %w", err)
	}

	slog.Info("Container installation completed successfully")

	// Save success log
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
	slog.Info("OS", "os", cfg.OS)
	slog.Info("Registry", "registry", cfg.ImageRegistry)
	slog.Info("Container Name", "name", cfg.ContainerName)
	slog.Info("Port", "port", cfg.Port)
	slog.Info("Data Path", "path", cfg.DataPath)
	if cfg.DockerConnection != nil {
		slog.Info("Docker Connection", "type", cfg.DockerConnection.Type)
	}
	if cfg.Proxy != "" {
		slog.Info("Proxy", "proxy", cfg.Proxy)
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
	if e.Config.DockerConnection != nil {
		fmt.Fprintf(file, "Docker Connection: %s\n", e.Config.DockerConnection.Type)
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

	// Step 1: Download binary
	slog.Info("Step 1: Downloading DPanel binary")
	if err := e.downloadBinary(); err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}

	// Step 2: Verify checksum (optional)
	slog.Info("Step 2: Verifying binary checksum")
	// TODO: Implement checksum verification

	// Step 3: Install to system path
	slog.Info("Step 3: Installing binary to system path")
	if err := e.installBinaryToPath(); err != nil {
		return fmt.Errorf("failed to install binary: %w", err)
	}

	// Step 4: Create service file
	slog.Info("Step 4: Creating service file")
	if err := e.createServiceFile(); err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	// Step 5: Start service
	slog.Info("Step 5: Starting service")
	if err := e.startBinaryService(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	slog.Info("Binary installation completed successfully")
	e.saveBinaryInstallResult(true, "")
	return nil
}

// buildDockerCommand builds the docker/podman run command
func (e *Engine) buildDockerCommand() (string, error) {
	cfg := e.Config

	// Determine if using podman
	usePodman := false
	if _, err := exec.LookPath("docker"); err != nil {
		if _, err := exec.LookPath("podman"); err == nil {
			usePodman = true
		}
	}

	runtime := "docker"
	if usePodman {
		runtime = "podman"
	}

	// Build image name
	image := e.buildImageName()

	// Build command parts
	var parts []string
	parts = append(parts, runtime, "run", "-d")
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
	if cfg.Proxy != "" {
		parts = append(parts, "-e", fmt.Sprintf("HTTP_PROXY=%s", cfg.Proxy))
		parts = append(parts, "-e", fmt.Sprintf("HTTPS_PROXY=%s", cfg.Proxy))
	}

	// DNS
	if cfg.DNS != "" {
		parts = append(parts, "--dns", cfg.DNS)
	}

	// Ports
	if cfg.Edition == "standard" {
		parts = append(parts, "-p", "80:80", "-p", "443:443")
	}
	if cfg.Port > 0 {
		parts = append(parts, "-p", fmt.Sprintf("%d:8080", cfg.Port))
	} else {
		// Random port
		parts = append(parts, "-p", "8080")
	}

	// Volumes
	parts = append(parts, "-v", fmt.Sprintf("%s:/var/run/docker.sock", cfg.DockerConnection.SockPath))
	parts = append(parts, "-v", fmt.Sprintf("%s:/dpanel", cfg.DataPath))

	// Image
	parts = append(parts, image)

	return strings.Join(parts, " "), nil
}

// buildImageName builds the Docker image name
func (e *Engine) buildImageName() string {
	cfg := e.Config
	var image string

	// Build base image name
	switch cfg.Version {
	case "community":
		if cfg.Edition == "lite" {
			image = "dpanel/dpanel:lite"
		} else {
			image = "dpanel/dpanel:latest"
		}
	case "pro":
		if cfg.Edition == "lite" {
			image = "dpanel/dpanel-pe:lite"
		} else {
			image = "dpanel/dpanel-pe:latest"
		}
	case "dev":
		if cfg.Edition == "lite" {
			image = "dpanel/dpanel:beta-lite"
		} else {
			image = "dpanel/dpanel:beta"
		}
	}

	// Add registry prefix
	if cfg.ImageRegistry == "aliyun" {
		image = "registry.cn-hangzhou.aliyuncs.com/" + image
	}

	return image
}

// executeCommand executes a shell command
func (e *Engine) executeCommand(cmd string) error {
	slog.Info("Executing command", "cmd", cmd)

	// Execute using sh -c
	parts := []string{"sh", "-c", cmd}
	command := exec.Command(parts[0], parts[1:]...)

	// Connect output to terminal for user feedback
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	if err := command.Run(); err != nil {
		return fmt.Errorf("command execution failed: %w", err)
	}

	slog.Info("Command executed successfully")
	return nil
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

	runtime := e.getDockerRuntime()

	// Step 1: Stop current container
	slog.Info("Step 1: Stopping current container")
	if err := e.stopContainer(runtime, e.Config.ContainerName); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	// Step 2: Pull new image
	slog.Info("Step 2: Pulling new image")
	image := e.buildImageName()
	if err := e.pullImage(runtime, image); err != nil {
		return fmt.Errorf("failed to pull new image: %w", err)
	}

	// Step 3: Remove old container
	slog.Info("Step 3: Removing old container")
	if err := e.removeContainer(runtime, e.Config.ContainerName); err != nil {
		return fmt.Errorf("failed to remove old container: %w", err)
	}

	// Step 4: Create and start new container
	slog.Info("Step 4: Creating and starting new container")
	cmd, err := e.buildDockerCommand()
	if err != nil {
		return fmt.Errorf("failed to build docker command: %w", err)
	}

	// Save upgrade log
	if err := e.saveUpgradeLog(cmd); err != nil {
		slog.Warn("Failed to save upgrade log", "error", err)
	}

	if err := e.executeCommand(cmd); err != nil {
		slog.Error("Container upgrade failed", "error", err)
		// Attempt rollback
		_ = e.rollbackUpgrade(runtime, image)
		return fmt.Errorf("container upgrade failed: %w", err)
	}

	slog.Info("Container upgrade completed successfully")
	e.saveUpgradeResult(true, "")
	return nil
}

// upgradeBinary upgrades the binary installation
func (e *Engine) upgradeBinary() error {
	slog.Info("Starting binary upgrade")
	// TODO: Implement binary upgrade logic
	return fmt.Errorf("binary upgrade is not implemented yet")
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

	runtime := e.getDockerRuntime()

	// Step 1: Check if container exists
	slog.Info("Step 1: Checking if container exists")
	containerExists, err := e.checkContainerExists(runtime, e.Config.ContainerName)
	if err != nil {
		return fmt.Errorf("failed to check container existence: %w", err)
	}

	if !containerExists {
		return fmt.Errorf("container %s does not exist", e.Config.ContainerName)
	}

	// Step 2: Stop container if running
	slog.Info("Step 2: Stopping container")
	if err := e.stopContainer(runtime, e.Config.ContainerName); err != nil {
		// Container might already be stopped, continue
		slog.Warn("Failed to stop container (may already be stopped)", "error", err)
	}

	// Step 3: Remove container
	slog.Info("Step 3: Removing container")
	if err := e.removeContainer(runtime, e.Config.ContainerName); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	// Step 4: Optional - Remove image
	// TODO: Add confirmation prompt for image removal
	// slog.Info("Step 4: Removing image")
	// image := e.buildImageName()
	// if err := e.removeImage(runtime, image); err != nil {
	// 	slog.Warn("Failed to remove image", "error", err)
	// }

	// Step 5: Optional - Remove data volumes
	// TODO: Add confirmation prompt for data removal
	// slog.Info("Step 5: Cleaning up data volumes")
	// if err := e.cleanupDataVolumes(); err != nil {
	// 	slog.Warn("Failed to cleanup data volumes", "error", err)
	// }

	slog.Info("Container uninstallation completed successfully")
	e.saveUninstallResult(true, "")
	return nil
}

// uninstallBinary uninstalls the binary installation
func (e *Engine) uninstallBinary() error {
	slog.Info("Starting binary uninstallation")
	// TODO: Implement binary uninstall logic
	return fmt.Errorf("binary uninstall is not implemented yet")
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
	// Determine install path based on OS
	installPath := e.getBinaryInstallPath()
	slog.Info("Installing binary to", "path", installPath)

	// TODO: Implement installation logic
	// Copy binary to install path
	// Set executable permissions
	return fmt.Errorf("binary installation to path not implemented yet")
}

// getBinaryInstallPath returns the installation path for the binary
func (e *Engine) getBinaryInstallPath() string {
	// Determine install path based on OS
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
