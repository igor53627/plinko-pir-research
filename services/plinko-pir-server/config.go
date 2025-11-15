package main

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultServerPort           = "3000"
	defaultDatabasePath         = "/data/database.bin"
	defaultDatabaseWaitTimeout  = 120 * time.Second
	deprecatedHintPathEnvNotice = "Deprecated hint path env var detected (%s); treating as database path"
)

type Config struct {
	ServerPort          string
	DatabasePath        string
	DatabaseWaitTimeout time.Duration
}

func LoadConfig() Config {
	cfg := Config{
		ServerPort:          defaultServerPort,
		DatabasePath:        defaultDatabasePath,
		DatabaseWaitTimeout: defaultDatabaseWaitTimeout,
	}

	if v := firstNonEmpty(
		os.Getenv("PLINKO_PIR_SERVER_PORT"),
		os.Getenv("SERVER_PORT"),
		os.Getenv("PORT"),
	); v != "" {
		cfg.ServerPort = v
	}

	if v := firstNonEmpty(
		os.Getenv("PLINKO_PIR_DATABASE_PATH"),
		os.Getenv("PLINKO_PIR_DB_PATH"),
		os.Getenv("DATABASE_PATH"),
		os.Getenv("DB_PATH"),
	); v != "" {
		cfg.DatabasePath = v
	} else if v := firstNonEmpty(
		os.Getenv("PLINKO_PIR_HINT_PATH"),
		os.Getenv("HINT_PATH"),
	); v != "" {
		log.Printf(deprecatedHintPathEnvNotice, v)
		cfg.DatabasePath = v
	}

	if v := firstNonEmpty(
		os.Getenv("PLINKO_PIR_DATABASE_TIMEOUT_SECONDS"),
		os.Getenv("PLINKO_PIR_DB_TIMEOUT_SECONDS"),
		os.Getenv("DATABASE_TIMEOUT_SECONDS"),
		os.Getenv("DB_TIMEOUT_SECONDS"),
		os.Getenv("PLINKO_PIR_HINT_TIMEOUT_SECONDS"), // backward compatibility
		os.Getenv("HINT_TIMEOUT_SECONDS"),
	); v != "" {
		if seconds, err := strconv.Atoi(v); err == nil && seconds >= 0 {
			cfg.DatabaseWaitTimeout = time.Duration(seconds) * time.Second
		} else {
			log.Printf("Invalid database timeout value %q, using default %v", v, defaultDatabaseWaitTimeout)
		}
	}

	cfg.ServerPort = strings.TrimSpace(cfg.ServerPort)
	cfg.DatabasePath = strings.TrimSpace(cfg.DatabasePath)

	return cfg
}

func (c Config) ListenAddress() string {
	port := strings.TrimSpace(c.ServerPort)
	if port == "" {
		port = defaultServerPort
	}

	if strings.HasPrefix(port, ":") || strings.Contains(port, ":") {
		return port
	}

	return ":" + port
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
