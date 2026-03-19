package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func tempSettingsPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, ".claude", "settings.json")
}

func writeJSON(t *testing.T, path string, v interface{}) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readJSON(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestInstall_Fresh(t *testing.T) {
	path := tempSettingsPath(t)

	if err := Install("/usr/local/bin/agenterm", path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	settings := readJSON(t, path)
	hooks := settings["hooks"].(map[string]interface{})
	pr := hooks["PermissionRequest"].([]interface{})
	if len(pr) != 1 {
		t.Fatalf("expected 1 PermissionRequest entry, got %d", len(pr))
	}

	entry := pr[0].(map[string]interface{})
	if entry["matcher"] != "" {
		t.Fatalf("expected empty matcher, got %q", entry["matcher"])
	}

	hooksList := entry["hooks"].([]interface{})
	hookEntry := hooksList[0].(map[string]interface{})
	if hookEntry["command"] != "/usr/local/bin/agenterm gate" {
		t.Fatalf("wrong command: %v", hookEntry["command"])
	}
	if hookEntry["timeout"] != float64(120) {
		t.Fatalf("wrong timeout: %v", hookEntry["timeout"])
	}
}

func TestInstall_AlreadyInstalled(t *testing.T) {
	path := tempSettingsPath(t)

	if err := Install("/usr/local/bin/agenterm", path); err != nil {
		t.Fatalf("first install failed: %v", err)
	}

	err := Install("/usr/local/bin/agenterm", path)
	if err == nil {
		t.Fatal("expected error on second install")
	}
	if err.Error() != "already installed" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInstall_PreservesExistingSettings(t *testing.T) {
	path := tempSettingsPath(t)
	writeJSON(t, path, map[string]interface{}{
		"theme": "dark",
		"hooks": map[string]interface{}{
			"PostToolUse": []interface{}{
				map[string]interface{}{"matcher": "", "hooks": []interface{}{}},
			},
		},
	})

	if err := Install("/usr/local/bin/agenterm", path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	settings := readJSON(t, path)
	if settings["theme"] != "dark" {
		t.Fatal("theme setting was lost")
	}
	hooks := settings["hooks"].(map[string]interface{})
	if hooks["PostToolUse"] == nil {
		t.Fatal("PostToolUse hook was lost")
	}
	if hooks["PermissionRequest"] == nil {
		t.Fatal("PermissionRequest not added")
	}
}

func TestInstall_CleansUpPreToolUse(t *testing.T) {
	path := tempSettingsPath(t)
	writeJSON(t, path, map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "",
					"hooks": []interface{}{
						map[string]interface{}{
							"type":    "command",
							"command": "/old/path/agenterm gate",
						},
					},
				},
				map[string]interface{}{
					"matcher": "*.py",
					"hooks": []interface{}{
						map[string]interface{}{
							"type":    "command",
							"command": "other-tool check",
						},
					},
				},
			},
		},
	})

	if err := Install("/usr/local/bin/agenterm", path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	settings := readJSON(t, path)
	hooks := settings["hooks"].(map[string]interface{})

	// PreToolUse should still exist with only the non-agenterm entry.
	ptu := hooks["PreToolUse"].([]interface{})
	if len(ptu) != 1 {
		t.Fatalf("expected 1 PreToolUse entry remaining, got %d", len(ptu))
	}

	// PermissionRequest should be installed.
	pr := hooks["PermissionRequest"].([]interface{})
	if len(pr) != 1 {
		t.Fatalf("expected 1 PermissionRequest entry, got %d", len(pr))
	}
}

func TestInstall_CleansUpPreToolUse_RemovesEmptyKey(t *testing.T) {
	path := tempSettingsPath(t)
	writeJSON(t, path, map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "",
					"hooks": []interface{}{
						map[string]interface{}{
							"type":    "command",
							"command": "/old/path/agenterm gate",
						},
					},
				},
			},
		},
	})

	if err := Install("/usr/local/bin/agenterm", path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	settings := readJSON(t, path)
	hooks := settings["hooks"].(map[string]interface{})

	if hooks["PreToolUse"] != nil {
		t.Fatal("expected PreToolUse key to be removed when empty")
	}
}

func TestUninstall_Basic(t *testing.T) {
	path := tempSettingsPath(t)

	// Install first.
	if err := Install("/usr/local/bin/agenterm", path); err != nil {
		t.Fatal(err)
	}

	if err := Uninstall(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	settings := readJSON(t, path)
	if settings["hooks"] != nil {
		t.Fatal("expected hooks to be removed entirely")
	}
}

func TestUninstall_NotInstalled(t *testing.T) {
	path := tempSettingsPath(t)
	writeJSON(t, path, map[string]interface{}{})

	err := Uninstall(path)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "not installed" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUninstall_PreservesOtherHooks(t *testing.T) {
	path := tempSettingsPath(t)
	writeJSON(t, path, map[string]interface{}{
		"hooks": map[string]interface{}{
			"PermissionRequest": []interface{}{
				map[string]interface{}{
					"matcher": "",
					"hooks": []interface{}{
						map[string]interface{}{"type": "command", "command": "/usr/local/bin/agenterm gate"},
					},
				},
				map[string]interface{}{
					"matcher": "*.sh",
					"hooks": []interface{}{
						map[string]interface{}{"type": "command", "command": "other-tool"},
					},
				},
			},
		},
	})

	if err := Uninstall(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	settings := readJSON(t, path)
	hooks := settings["hooks"].(map[string]interface{})
	pr := hooks["PermissionRequest"].([]interface{})
	if len(pr) != 1 {
		t.Fatalf("expected 1 remaining entry, got %d", len(pr))
	}
}

func TestUninstall_BothEventTypes(t *testing.T) {
	path := tempSettingsPath(t)
	writeJSON(t, path, map[string]interface{}{
		"hooks": map[string]interface{}{
			"PermissionRequest": []interface{}{
				map[string]interface{}{
					"matcher": "",
					"hooks": []interface{}{
						map[string]interface{}{"type": "command", "command": "/usr/local/bin/agenterm gate"},
					},
				},
			},
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "",
					"hooks": []interface{}{
						map[string]interface{}{"type": "command", "command": "/old/agenterm gate"},
					},
				},
			},
		},
	})

	if err := Uninstall(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	settings := readJSON(t, path)
	if settings["hooks"] != nil {
		t.Fatal("expected hooks to be removed entirely")
	}
}

func TestInstall_NoFileExists(t *testing.T) {
	path := tempSettingsPath(t)

	// File doesn't exist yet — should create it.
	if err := Install("/usr/local/bin/agenterm", path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected settings file to be created")
	}
}
