package config

import "os"

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	NATSURL     string
}

func Load() Config {
	return Config{
		HTTPAddr:    getEnv("APOLLO_HTTP_ADDR", ":8081"),
		DatabaseURL: getEnv("APOLLO_DATABASE_URL", ""),
		NATSURL:     getEnv("APOLLO_NATS_URL", ""),
	}
}

func getEnv(key string, fallback string) string {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}

	return value
}
