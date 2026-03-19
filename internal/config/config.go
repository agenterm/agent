package config

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const DefaultRelayURL = "https://push.agenterm.app"

type Config struct {
	RelayURL string `json:"relay_url"`
	PushKey  string `json:"push_key"`
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agenterm"), nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// ConfigPath returns the path to ~/.agenterm/config.json.
func ConfigPath() (string, error) {
	return configPath()
}

// Load reads config from ~/.agenterm/config.json.
// Returns default config if the file does not exist.
func Load() (*Config, error) {
	cfg := &Config{RelayURL: DefaultRelayURL}

	p, err := configPath()
	if err != nil {
		return cfg, nil
	}

	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if cfg.RelayURL == "" {
		cfg.RelayURL = DefaultRelayURL
	}
	return cfg, nil
}

// Save writes config to ~/.agenterm/config.json, creating the directory if needed.
func (c *Config) Save() error {
	p, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

// Validate checks that the push key is accepted by the relay.
// It sends a test POST /proposals request and treats HTTP 401 as invalid.
func (c *Config) Validate() error {
	url := strings.TrimRight(c.RelayURL, "/") + "/proposals"
	body := `{"type":"status","title":"__validate__"}`
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.PushKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return fmt.Errorf("connecting to relay: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid push key (HTTP 401)")
	}
	return nil
}
