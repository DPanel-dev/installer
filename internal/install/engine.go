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
	// TODO: Implement upgrade logic
	return fmt.Errorf("upgrade feature is not implemented yet")
}

// uninstall performs the uninstallation
func (e *Engine) uninstall() error {
	slog.Info("Running uninstall")
	// TODO: Implement uninstall logic
	return fmt.Errorf("uninstall feature is not implemented yet")
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
	// TODO: Implement actual TCP connectivity check
	slog.Info("TCP connection check not implemented yet", "host", conn.Host)
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
	// TODO: Implement actual SSH connectivity check
	slog.Info("SSH connection check not implemented yet", "host", conn.Host)
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
	// TODO: Implement binary installation
	return fmt.Errorf("binary installation is not implemented yet")
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
