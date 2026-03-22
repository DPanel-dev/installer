package install

type Config struct {
	Action        string // install, upgrade, uninstall
	Language      string // zh, en
	InstallType   string // container, binary
	Version       string // community, pro, dev
	Edition       string // standard, lite
	OS            string // alpine, debian
	ImageRegistry string // aliyun, hub
	ContainerName string
	Port          int
	DataPath      string
	DockerSock    string
	Proxy         string
	DNS           string
}

type Engine struct {
	Config *Config
}

func NewEngine(cfg *Config) *Engine {
	return &Engine{Config: cfg}
}

func (e *Engine) Run() error {
	// TODO: Implement installation logic based on Config
	return nil
}
