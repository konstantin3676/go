package app

import (
	"net"
	"net/url"
	"os"
	"strconv"
)

type config struct {
	databaseDSN   string
	redisAddr     string
	redisPassword string
	redisDB       int
}

func configFromEnv() config {
	redisDB, err := strconv.Atoi(envOrDefault("REDIS_DB", "0"))
	if err != nil || redisDB < 0 {
		redisDB = 0
	}
	return config{
		databaseDSN:   databaseDSN(),
		redisAddr:     envOrDefault("REDIS_ADDR", "localhost:6379"),
		redisPassword: os.Getenv("REDIS_PASSWORD"),
		redisDB:       redisDB,
	}
}

func databaseDSN() string {
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		return dsn
	}
	dsn := url.URL{
		Scheme: "postgres",
		User: url.UserPassword(
			envOrDefault("DB_USER", "postgres"),
			envOrDefault("DB_PASS", "postgres"),
		),
		Host:     net.JoinHostPort(envOrDefault("DB_HOST", "localhost"), envOrDefault("DB_PORT", "5432")),
		Path:     "/" + envOrDefault("DB_NAME", "cashapp"),
		RawQuery: "sslmode=disable",
	}
	return dsn.String()
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
