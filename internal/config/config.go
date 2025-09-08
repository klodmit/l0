package config

import (
	"log"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env        string     `yaml:"env" env-default:"development"`
	HttpServer HttpServer `yaml:"http_server"`
	Db         Db         `yaml:"db"`
}

type HttpServer struct {
	Address     string        `yaml:"address"      env:"ADDRESS"`
	Timeout     time.Duration `yaml:"timeout"      env:"TIMEOUT"`
	IdleTimeout time.Duration `yaml:"idle_timeout" env:"IDLE_TIMEOUT"`
}
type Db struct {
	DatabaseUser string `yaml:"database_user" env:"DB_USER"`
	DatabasePass string `yaml:"database_pass" env:"DB_PASS"`
	DatabaseHost string `yaml:"database_host" env:"DB_HOST"`
	DatabasePort string `yaml:"database_port" env:"DB_PORT"`
	DatabaseName string `yaml:"database_name" env:"DB_NAME"`
}

func MustLoad() Config {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		log.Fatal("CONFIG_PATH environment variable not set")
	}
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatalf("config file does not exist: %s", configPath)
	}

	var cfg Config
	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		log.Fatalf("Can't read config: %s", err)
	}
	return cfg
}
