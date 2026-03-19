package hook

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrAlreadyInstalled is returned when the hook is already present in settings.
	ErrAlreadyInstalled = errors.New("already installed")
	// ErrNotInstalled is returned when the hook is not found in settings.
	ErrNotInstalled = errors.New("not installed")
)

const hookMarker = "agenterm gate"

// GeminiSettingsPath returns the default path to Gemini CLI's settings.json.
func GeminiSettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".gemini", "settings.json"), nil
}

// buildGeminiHookEntry creates a BeforeTool hook entry for Gemini CLI.
func buildGeminiHookEntry(binaryPath string) map[string]interface{} {
	return map[string]interface{}{
		"matcher": "*",
		"hooks": []interface{}{
			map[string]interface{}{
				"name":    "agenterm-gate",
				"type":    "command",
				"command": binaryPath + " gate",
				"timeout": 120000, // Gemini timeout is in ms
			},
		},
	}
}

// SettingsPath returns the default path to Claude Code's settings.json.
func SettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

// readSettings reads and parses settings.json at the given path.
// Returns an empty map if the file does not exist.
func readSettings(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]interface{}{}, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return settings, nil
}

// writeSettings writes settings back to the given path with indentation.
func writeSettings(path string, settings map[string]interface{}) error {
	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// getHooksMap returns the "hooks" object from settings, creating it if needed.
func getHooksMap(settings map[string]interface{}) map[string]interface{} {
	if h, ok := settings["hooks"].(map[string]interface{}); ok {
		return h
	}
	h := map[string]interface{}{}
	settings["hooks"] = h
	return h
}

// getHookEntries returns the array for a given hook event name.
func getHookEntries(hooks map[string]interface{}, eventName string) []interface{} {
	if arr, ok := hooks[eventName].([]interface{}); ok {
		return arr
	}
	return nil
}

// containsMarker checks if any hook entry in the array contains the agenterm gate command.
func containsMarker(entries []interface{}) bool {
	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), hookMarker) {
			return true
		}
	}
	return false
}

// removeMarkerEntries removes hook entries that contain the agenterm gate command.
// Returns the filtered array and the count of removed entries.
func removeMarkerEntries(entries []interface{}) ([]interface{}, int) {
	var result []interface{}
	removed := 0
	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			result = append(result, entry)
			continue
		}
		if strings.Contains(string(data), hookMarker) {
			removed++
		} else {
			result = append(result, entry)
		}
	}
	return result, removed
}

// buildHookEntry creates a PermissionRequest hook entry for the given binary path.
func buildHookEntry(binaryPath string) map[string]interface{} {
	return map[string]interface{}{
		"matcher": "",
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": binaryPath + " gate",
				"timeout": 120,
			},
		},
	}
}

// Install adds the agenterm gate hook to Claude Code settings.
// It installs a PermissionRequest hook and removes any legacy PreToolUse hooks.
// binaryPath is the absolute path to the agenterm binary.
// settingsPath can be empty to use the default path.
func Install(binaryPath, settingsPath string) error {
	if settingsPath == "" {
		var err error
		settingsPath, err = SettingsPath()
		if err != nil {
			return err
		}
	}

	settings, err := readSettings(settingsPath)
	if err != nil {
		return err
	}

	hooks := getHooksMap(settings)

	// Check if already installed in PermissionRequest.
	prEntries := getHookEntries(hooks, "PermissionRequest")
	if containsMarker(prEntries) {
		return ErrAlreadyInstalled
	}

	// Clean up legacy PreToolUse hook if present.
	ptuEntries := getHookEntries(hooks, "PreToolUse")
	if len(ptuEntries) > 0 {
		filtered, removed := removeMarkerEntries(ptuEntries)
		if removed > 0 {
			if len(filtered) == 0 {
				delete(hooks, "PreToolUse")
			} else {
				hooks["PreToolUse"] = filtered
			}
		}
	}

	// Add PermissionRequest hook entry.
	entry := buildHookEntry(binaryPath)
	prEntries = append(prEntries, entry)
	hooks["PermissionRequest"] = prEntries

	return writeSettings(settingsPath, settings)
}

// Uninstall removes all agenterm gate hooks from Claude Code settings.
// settingsPath can be empty to use the default path.
func Uninstall(settingsPath string) error {
	if settingsPath == "" {
		var err error
		settingsPath, err = SettingsPath()
		if err != nil {
			return err
		}
	}
	return uninstallFromSettings(settingsPath, "PermissionRequest", "PreToolUse")
}

// InstallGemini adds the agenterm gate hook to Gemini CLI settings.
// binaryPath is the absolute path to the agenterm binary.
// settingsPath can be empty to use the default path.
func InstallGemini(binaryPath, settingsPath string) error {
	if settingsPath == "" {
		var err error
		settingsPath, err = GeminiSettingsPath()
		if err != nil {
			return err
		}
	}

	settings, err := readSettings(settingsPath)
	if err != nil {
		return err
	}

	hooks := getHooksMap(settings)

	// Check if already installed in BeforeTool.
	btEntries := getHookEntries(hooks, "BeforeTool")
	if containsMarker(btEntries) {
		return ErrAlreadyInstalled
	}

	// Add BeforeTool hook entry.
	entry := buildGeminiHookEntry(binaryPath)
	btEntries = append(btEntries, entry)
	hooks["BeforeTool"] = btEntries

	return writeSettings(settingsPath, settings)
}

// UninstallGemini removes all agenterm gate hooks from Gemini CLI settings.
// settingsPath can be empty to use the default path.
func UninstallGemini(settingsPath string) error {
	if settingsPath == "" {
		var err error
		settingsPath, err = GeminiSettingsPath()
		if err != nil {
			return err
		}
	}
	return uninstallFromSettings(settingsPath, "BeforeTool")
}

// uninstallFromSettings removes agenterm gate hooks for the given event names.
func uninstallFromSettings(settingsPath string, eventNames ...string) error {
	settings, err := readSettings(settingsPath)
	if err != nil {
		return err
	}

	hooks := getHooksMap(settings)
	totalRemoved := 0

	for _, eventName := range eventNames {
		entries := getHookEntries(hooks, eventName)
		if len(entries) == 0 {
			continue
		}
		filtered, removed := removeMarkerEntries(entries)
		totalRemoved += removed
		if len(filtered) == 0 {
			delete(hooks, eventName)
		} else {
			hooks[eventName] = filtered
		}
	}

	if totalRemoved == 0 {
		return ErrNotInstalled
	}

	// Clean up empty hooks object.
	if len(hooks) == 0 {
		delete(settings, "hooks")
	}

	return writeSettings(settingsPath, settings)
}
