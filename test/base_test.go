package test

import (
	"MacArthurGo/base"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig_Valid(t *testing.T) {
	old := base.Config
	t.Cleanup(func() { base.Config = old })

	path := writeTestConfig(t, `{
		"address": "ws://127.0.0.1:3001",
		"updateInterval": 60,
		"plugins": {
			"repeat": {"probability": 0.4, "commonProbability": 0.01},
			"picSearch": {"ascii2d": {"timeoutSeconds": 90}}
		}
	}`)
	if err := base.LoadConfig(path); err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if base.Config.Address != "ws://127.0.0.1:3001" {
		t.Fatalf("Config.Address = %q", base.Config.Address)
	}
	if base.Config.ConfigPath != path {
		t.Fatalf("Config.ConfigPath = %q, want %q", base.Config.ConfigPath, path)
	}
	if base.Config.StartTime == 0 {
		t.Fatal("Config.StartTime was not initialized")
	}
}

func TestLoadConfig_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "address", content: `{"address":"http://127.0.0.1"}`, want: "ws:// or wss://"},
		{name: "probability", content: `{"address":"ws://127.0.0.1","plugins":{"repeat":{"probability":1.1}}}`, want: "probability"},
		{name: "trailing value", content: `{"address":"ws://127.0.0.1"} {}`, want: "multiple JSON values"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			old := base.Config
			defer func() { base.Config = old }()
			err := base.LoadConfig(writeTestConfig(t, test.content))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("LoadConfig() error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestConfiguration_UpdateConfig(t *testing.T) {
	old := base.Config
	t.Cleanup(func() { base.Config = old })

	path := writeTestConfig(t, `{"address":"ws://127.0.0.1"}`)
	if err := base.LoadConfig(path); err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	base.Config.BannedList = []int64{10001, 10002}
	if err := base.Config.UpdateConfig(); err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("saved config is invalid JSON: %v", err)
	}
	if _, exists := saved["ConfigPath"]; exists {
		t.Fatal("runtime ConfigPath was serialized")
	}
	banned, ok := saved["bannedList"].([]any)
	if !ok || len(banned) != 2 {
		t.Fatalf("saved bannedList = %#v", saved["bannedList"])
	}
}

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
