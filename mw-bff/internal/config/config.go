package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppEnv       string
	Port         int
	DatabasePath string
}

func Load() (Config, error) {
	portRaw := getenv("PORT", "7200")
	port, err := strconv.Atoi(portRaw)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("invalid PORT %q", portRaw)
	}

	databasePath := strings.TrimSpace(
		getenv("DATABASE_PATH", "./data/bff.db"),
	)
	if databasePath == "" {
		return Config{}, fmt.Errorf("DATABASE_PATH is required")
	}

	return Config{
		AppEnv:       getenv("APP_ENV", "development"),
		Port:         port,
		DatabasePath: databasePath,
	}, nil
}

func getenv(key, fallback string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}

	return value
}
