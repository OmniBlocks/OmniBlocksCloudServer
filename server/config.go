package server

import (
	"os"

	"github.com/BurntSushi/toml"
)

const DefaultPort = 9080

// Config
type Config struct {
	Port int `toml:"port"`
}

// Loads TOML config file. Safe fallback.
func LoadConfig(path string) (Config, error) {
	cfg := Config{Port: DefaultPort}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, err
	}

	if cfg.Port == 0 {
		cfg.Port = DefaultPort
	}

	return cfg, nil
}
