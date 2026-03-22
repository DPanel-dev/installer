package install

// Config represents the complete installation configuration
type Config struct {
	// Action: install, upgrade, uninstall
	Action string

	// Language: zh, en (only used in TUI mode)
	Language string

	// Installation type: container, binary
	InstallType string

	// Version: community, pro, dev
	Version string

	// Edition: standard, lite
	Edition string

	// OS base: alpine, debian
	OS string

	// Image registry: hub, aliyun
	ImageRegistry string

	// Container configuration
	ContainerName string
	Port          int
	DataPath      string

	// Docker connection configuration
	DockerConnection *DockerConnection

	// Network configuration
	Proxy string
	DNS   string
}

// DockerConnection represents Docker/Podman connection configuration
type DockerConnection struct {
	// Connection type: local, tcp, ssh
	Type string

	// Local connection
	SockPath string

	// TCP connection
	Host       string
	TLSEnabled bool
	TLSPath    string

	// SSH connection
	SSHUser string
	SSHPass string
	SSHKey  string
}

// NewConfig creates a new configuration with default values
func NewConfig() *Config {
	return &Config{
		Action:        "install",
		Language:      "zh",
		InstallType:   "container",
		Version:       "community",
		Edition:       "lite",
		OS:            "debian",
		ImageRegistry: "hub",
		ContainerName: "dpanel",
		Port:          0, // 0 means random port
		DataPath:      "/home/dpanel",
		DockerConnection: &DockerConnection{
			Type:     "local",
			SockPath: "/var/run/docker.sock",
		},
		Proxy: "",
		DNS:   "",
	}
}
