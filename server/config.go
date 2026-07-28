package server

import (
	"os"

	"github.com/BurntSushi/toml"
)

const DefaultPort = 9080

const (
	defaultLogFileDirectory  = "logs"
	defaultLogFileMaxSizeMB  = 10
	defaultLogFileMaxBackups = 5
)

// Config
type Config struct {
	Port    int           `toml:"port"`
	Logging LoggingConfig `toml:"logging"`
}

// LoggingConfig controls diagnostic logging. Enabled is a master switch;
// Categories lets each logged part of the system be turned on or off
// independently.
type LoggingConfig struct {
	Enabled    bool                `toml:"enabled"`
	Color      bool                `toml:"color"`
	Categories LogCategoriesConfig `toml:"categories"`
	File       LogFileConfig       `toml:"file"`
}

// LogCategoriesConfig toggles individual logged parts of the system.
type LogCategoriesConfig struct {
	Server     bool `toml:"server"`
	Config     bool `toml:"config"`
	Connection bool `toml:"connection"`
	Handshake  bool `toml:"handshake"`
	Messages   bool `toml:"messages"`
	Variables  bool `toml:"variables"`
	Rooms      bool `toml:"rooms"`
	Broadcast  bool `toml:"broadcast"`
	Errors     bool `toml:"errors"`
}

// LogFileConfig controls optional rotating, gzip-compressed file logging.
// When enabled, log lines are appended to "current.log" inside Directory;
// once it grows past MaxSizeMB, it is compressed into a timestamped
// "log-*.gz" file, keeping at most MaxBackups of them.
type LogFileConfig struct {
	Enabled    bool   `toml:"enabled"`
	Directory  string `toml:"directory"`
	MaxSizeMB  int    `toml:"max_size_mb"`
	MaxBackups int    `toml:"max_backups"`
}

func defaultLoggingConfig() LoggingConfig {
	return LoggingConfig{
		Enabled: true,
		Color:   true,
		Categories: LogCategoriesConfig{
			Server:     true,
			Config:     true,
			Connection: true,
			Handshake:  true,
			Messages:   true,
			Variables:  true,
			Rooms:      true,
			Broadcast:  true,
			Errors:     true,
		},
		File: LogFileConfig{
			Enabled:    false,
			Directory:  defaultLogFileDirectory,
			MaxSizeMB:  defaultLogFileMaxSizeMB,
			MaxBackups: defaultLogFileMaxBackups,
		},
	}
}

// Loads TOML config file. Safe fallback.
func LoadConfig(path string) (Config, error) {
	cfg := Config{Port: DefaultPort, Logging: defaultLoggingConfig()}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, err
	}

	if cfg.Port == 0 {
		cfg.Port = DefaultPort
	}
	if cfg.Logging.File.Directory == "" {
		cfg.Logging.File.Directory = defaultLogFileDirectory
	}
	if cfg.Logging.File.MaxSizeMB == 0 {
		cfg.Logging.File.MaxSizeMB = defaultLogFileMaxSizeMB
	}

	return cfg, nil
}
