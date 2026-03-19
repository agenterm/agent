package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	// TC-C01: Save then Load returns same data
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := &Config{RelayURL: "https://example.com", PushKey: "my-key"}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.RelayURL != cfg.RelayURL || loaded.PushKey != cfg.PushKey {
		t.Fatalf("Load() = %+v, want %+v", loaded, cfg)
	}
}

func TestLoadDefault(t *testing.T) {
	// TC-C02: file does not exist → default config
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.RelayURL != DefaultRelayURL {
		t.Fatalf("RelayURL = %q, want %q", cfg.RelayURL, DefaultRelayURL)
	}
	if cfg.PushKey != "" {
		t.Fatalf("PushKey = %q, want empty", cfg.PushKey)
	}
}

func TestSaveCreatesDir(t *testing.T) {
	// TC-C03: Save auto-creates ~/.agenterm directory
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".agenterm")
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("directory should not exist before Save")
	}

	cfg := &Config{RelayURL: "https://example.com", PushKey: "k"}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}
}

func TestConfigPathSuffix(t *testing.T) {
	// TC-C04: ConfigPath ends with .agenterm/config.json
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	p, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() error: %v", err)
	}
	if !strings.HasSuffix(p, filepath.Join(".agenterm", "config.json")) {
		t.Fatalf("ConfigPath() = %q, want suffix .agenterm/config.json", p)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	// TC-C05: invalid JSON file → Load returns error
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".agenterm")
	os.MkdirAll(dir, 0o700)
	os.WriteFile(filepath.Join(dir, "config.json"), []byte("{invalid json"), 0o600)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should return error for invalid JSON")
	}
}
