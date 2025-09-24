package config

import (
	"log"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

const configPath = "./config/config.yaml"

type Config struct {
	Env        string     `yaml:"env" env:"ENV" env-default:"local" env-requered:"true"`
	Service    Service    `yaml:"service"`
	HTTPServer HTTPServer `yaml:"http-server"`
}

type Service struct {
	WorkerCount uint16 `yaml:"worker_count" env-default:"4"`
	FilesAtTime uint16 `yaml:"files_at_time" env-default:"20"`
	StorageDir  string `yaml:"storage_dir" env-default:"storage/"`
	DbDir       string `yaml:"db_dir" env-default:"database/"`
}

type HTTPServer struct {
	Address     string        `yaml:"address" env-default:":8080"`
	Timeout     time.Duration `yaml:"timeout" env-default:"10s"`
	IdleTimeout time.Duration `yaml:"idle_timeout" env-default:"60s"`
}

func NewConfigFile() *Config {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatalf("cannot find config file by path: %s", configPath)
	}

	var cfg Config

	err := cleanenv.ReadConfig(configPath, cfg)
	if err != nil {
		log.Fatalf("failed to read config file: %s", err)
	}

	return &cfg
}
