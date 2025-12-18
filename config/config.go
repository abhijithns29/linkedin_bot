package config

import (
	"errors"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	Headless     bool   `yaml:"headless"`
	UserAgent    string `yaml:"user_agent"`
	ProxyURL     string `yaml:"proxy_url"`
	UserDataDir  string `yaml:"user_data_dir"`
	MonitorIndex int    `yaml:"monitor_index"`

	LinkedIn struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"linkedin"`

	Limits struct {
		DailyConnections int `yaml:"daily_connections"`
		DailyMessages    int `yaml:"daily_messages"`
	} `yaml:"limits"`
}

// LoadConfig reads the config file and applies environment variable overrides
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{}

	// Defaults across the board
	cfg.Headless = true
	cfg.Limits.DailyConnections = 20
	cfg.Limits.DailyMessages = 20

	// 1. Read YAML file
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			// If file doesn't exist, we warn but continue if we can rely on env vars?
			// Or strict fail?
			// Let's fail if path provided but not found
			if !os.IsNotExist(err) {
				return nil, err
			}
			// If not exist, just continue with defaults/env
		} else {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
		}
	}

	// 2. Env Overrides
	if v := os.Getenv("LINKEDIN_HEADLESS"); v != "" {
		cfg.Headless = (v == "true" || v == "1")
	}
	if v := os.Getenv("LINKEDIN_USER_AGENT"); v != "" {
		cfg.UserAgent = v
	}
	if v := os.Getenv("LINKEDIN_PROXY"); v != "" {
		cfg.ProxyURL = v
	}
	if v := os.Getenv("LINKEDIN_USER_DATA"); v != "" {
		cfg.UserDataDir = v
	}
	if v := os.Getenv("LINKEDIN_USERNAME"); v != "" {
		cfg.LinkedIn.Username = v
	}
	if v := os.Getenv("LINKEDIN_PASSWORD"); v != "" {
		cfg.LinkedIn.Password = v
	}

	if v := os.Getenv("LINKEDIN_LIMIT_CONNECT"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.Limits.DailyConnections = i
		}
	}

	// 3. Validation
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks for required fields
func (c *Config) Validate() error {
	if c.LinkedIn.Username == "" || c.LinkedIn.Password == "" {
		// If UserDataDir is set, maybe we don't need credentials (session reuse)?
		// But for now let's warn or strict check?
		// We'll allow empty creds IF UserDataDir is set (session might be valid)
		if c.UserDataDir == "" {
			return errors.New("linkedin credentials (username/password) or user_data_dir are required")
		}
	}
	return nil
}
