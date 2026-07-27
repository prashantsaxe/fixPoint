package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

var VerboseLogging bool

type Config struct {
	OpenRouterAPIKey string `json:"openrouter_api_key"`
	Model            string `json:"model"`
}

func getConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("could not find user config dir: %w", err)
	}

	fixpointDir := filepath.Join(configDir, "fixpoint")
	if err := os.MkdirAll(fixpointDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	return filepath.Join(fixpointDir, "config.json"), nil
}

func LoadConfig() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if file doesn't exist
			return &Config{
				Model: "google/gemini-2.5-flash",
			}, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults if empty
	if cfg.Model == "" {
		cfg.Model = "google/gemini-2.5-flash"
	}

	return &cfg, nil
}

func SaveConfig(cfg *Config) error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
