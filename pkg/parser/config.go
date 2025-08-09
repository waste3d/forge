package parser

// Config — это корневая структура, представляющая весь файл forge.yaml
type Config struct {
	Version   int             `yaml:"version"`
	AppName   string          `yaml:"appName"`
	Services  []ServiceConfig `yaml:"services"`
	Databases []DBConfig      `yaml:"databases"`
}

// ServiceConfig описывает один сервис, например, бэкенд или фронтенд
type ServiceConfig struct {
	Name         string   `yaml:"name"`
	Type         string   `yaml:"type"` // example: go, node, python, etc..
	Repo         string   `yaml:"repo,omitempty"`
	Path         string   `yaml:"path,omitempty"`
	Port         int      `yaml:"port"`
	InternalPort int      `yaml:"internalPort,omitempty"`
	DependsOn    []string `yaml:"dependsOn,omitempty"`
	Env          []string `yaml:"env,omitempty"`
}

// DBConfig описывает одну базу данных
type DBConfig struct {
	Name         string   `yaml:"name"`
	Type         string   `yaml:"type"` // example: "postgres", "redis", "mongo", etc..
	Version      string   `yaml:"version"`
	Port         int      `yaml:"port"`
	InternalPort int      `yaml:"internalPort"`
	DependsOn    []string `yaml:"dependsOn,omitempty"`
	Env          []string `yaml:"env,omitempty"`
}
