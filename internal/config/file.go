package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

// Имена флагов пути к конфиг-файлу, общие для сервера и агента.
const (
	flagConfig      = "c"
	flagConfigLong  = "config"
	configUsageHint = "path to JSON config file"
)

// resolveConfigPath возвращает путь к конфиг-файлу с учётом приоритета env > flag.
// flagValue — значение флага -c/-config (пусто, если флаг не задан).
// Если заданы оба источника и пути различаются, пишет предупреждение в лог:
// значение флага молча проигрывать env не должно.
func resolveConfigPath(flagValue string) string {
	if v, ok := os.LookupEnv("CONFIG"); ok {
		if flagValue != "" && flagValue != v {
			log.Printf("конфиг-файл: путь из флага -c/-config (%q) проигнорирован в пользу env CONFIG (%q)", flagValue, v)
		}
		return v
	}
	return flagValue
}

// loadConfigFile читает JSON-файл по пути path и распаковывает его в dst.
// Пустой path означает, что файл не задан — функция ничего не делает.
func loadConfigFile(path string, dst any) error {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("чтение конфиг-файла %q: %w", path, err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("разбор конфиг-файла %q: %w", path, err)
	}
	return nil
}

// parseDurationSeconds переводит строку-длительность вида "1s" в целые секунды.
// Используется для интервалов в конфиг-файле, тогда как флаги/env задают секунды числом.
func parseDurationSeconds(s string) (int, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("разбор длительности %q: %w", s, err)
	}
	return int(d.Seconds()), nil
}

// setFlags возвращает множество имён флагов, заданных пользователем явно.
// Нужно, чтобы значения из конфиг-файла перекрывали дефолты, но не явные флаги.
func setFlags(fs *flag.FlagSet) map[string]bool {
	set := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		set[f.Name] = true
	})
	return set
}
