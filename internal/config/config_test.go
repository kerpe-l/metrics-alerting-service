package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTempConfig создаёт временный JSON-файл конфигурации и возвращает путь к нему.
func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("не удалось записать конфиг: %v", err)
	}
	return path
}

func TestServerConfigPrecedence(t *testing.T) {
	file := `{"address":"file:1111","store_interval":"5s","store_file":"/file.db","restore":false}`

	tests := []struct {
		name    string
		args    []string
		env     map[string]string
		wantAddr string
	}{
		{
			name:     "дефолт, когда нет ни файла, ни флага, ни env",
			args:     nil,
			wantAddr: "localhost:8080",
		},
		{
			name:     "файл перекрывает дефолт",
			args:     []string{"-c", writeTempConfig(t, file)},
			wantAddr: "file:1111",
		},
		{
			name:     "флаг перекрывает файл",
			args:     []string{"-c", writeTempConfig(t, file), "-a", "flag:2222"},
			wantAddr: "flag:2222",
		},
		{
			name:     "env перекрывает флаг и файл",
			args:     []string{"-c", writeTempConfig(t, file), "-a", "flag:2222"},
			env:      map[string]string{"ADDRESS": "env:3333"},
			wantAddr: "env:3333",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			cfg, err := parseServerConfig(tc.args)
			if err != nil {
				t.Fatalf("parseServerConfig: %v", err)
			}
			if cfg.Address != tc.wantAddr {
				t.Errorf("Address = %q, хотим %q", cfg.Address, tc.wantAddr)
			}
		})
	}
}

func TestServerConfigDurationFromFile(t *testing.T) {
	path := writeTempConfig(t, `{"store_interval":"5s"}`)
	cfg, err := parseServerConfig([]string{"-c", path})
	if err != nil {
		t.Fatalf("parseServerConfig: %v", err)
	}
	if cfg.StoreInterval != 5 {
		t.Errorf("StoreInterval = %d, хотим 5", cfg.StoreInterval)
	}
}

func TestServerConfigFileErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string // содержимое файла; пусто = несуществующий путь
		missing bool
	}{
		{name: "несуществующий файл", missing: true},
		{name: "битый JSON", content: `{ broken`},
		{name: "невалидная длительность", content: `{"store_interval":"five"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "missing.json")
			if !tc.missing {
				path = writeTempConfig(t, tc.content)
			}
			if _, err := parseServerConfig([]string{"-c", path}); err == nil {
				t.Error("ожидали ошибку, получили nil")
			}
		})
	}
}

func TestConfigPathEnvBeatsFlag(t *testing.T) {
	envFile := writeTempConfig(t, `{"address":"from-env-file:1"}`)
	flagFile := writeTempConfig(t, `{"address":"from-flag-file:2"}`)
	t.Setenv("CONFIG", envFile)

	cfg, err := parseServerConfig([]string{"-c", flagFile})
	if err != nil {
		t.Fatalf("parseServerConfig: %v", err)
	}
	if cfg.Address != "from-env-file:1" {
		t.Errorf("Address = %q, хотим из CONFIG-файла", cfg.Address)
	}
}

func TestAgentConfigPrecedence(t *testing.T) {
	file := `{"address":"file:1111","report_interval":"30s","poll_interval":"7s","rate_limit":5}`

	tests := []struct {
		name     string
		args     []string
		env      map[string]string
		wantAddr string
		wantPoll int
	}{
		{
			name:     "дефолт",
			wantAddr: "localhost:8080",
			wantPoll: 2,
		},
		{
			name:     "файл перекрывает дефолт",
			args:     []string{"-c", writeTempConfig(t, file)},
			wantAddr: "file:1111",
			wantPoll: 7,
		},
		{
			name:     "флаг перекрывает файл",
			args:     []string{"-c", writeTempConfig(t, file), "-p", "9"},
			wantAddr: "file:1111",
			wantPoll: 9,
		},
		{
			name:     "env перекрывает всё",
			args:     []string{"-c", writeTempConfig(t, file), "-p", "9"},
			env:      map[string]string{"POLL_INTERVAL": "11"},
			wantAddr: "file:1111",
			wantPoll: 11,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			cfg, err := parseAgentConfig(tc.args)
			if err != nil {
				t.Fatalf("parseAgentConfig: %v", err)
			}
			if cfg.Address != tc.wantAddr {
				t.Errorf("Address = %q, хотим %q", cfg.Address, tc.wantAddr)
			}
			if cfg.PollInterval != tc.wantPoll {
				t.Errorf("PollInterval = %d, хотим %d", cfg.PollInterval, tc.wantPoll)
			}
		})
	}
}

func TestAgentConfigValidatesIntervals(t *testing.T) {
	path := writeTempConfig(t, `{"poll_interval":"0s"}`)
	if _, err := parseAgentConfig([]string{"-c", path}); err == nil {
		t.Error("ожидали ошибку валидации при poll_interval=0")
	}
}
