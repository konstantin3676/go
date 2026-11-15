package app

import (
	"net/url"
	"testing"
)

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"DATABASE_URL", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASS", "DB_NAME",
		"REDIS_ADDR", "REDIS_PASSWORD", "REDIS_DB",
	} {
		t.Setenv(name, "")
	}
}

func TestConfigDefaults(t *testing.T) {
	clearConfigEnv(t)
	got := configFromEnv()
	want := config{
		databaseDSN: "postgres://postgres:postgres@localhost:5432/cashapp?sslmode=disable",
		redisAddr:   "localhost:6379",
	}
	if got != want {
		t.Fatalf("config = %+v, want %+v", got, want)
	}
}

func TestDatabaseURLTakesPrecedence(t *testing.T) {
	clearConfigEnv(t)
	want := "postgres://custom:secret@db:5433/ledger?sslmode=require"
	t.Setenv("DATABASE_URL", want)
	t.Setenv("DB_HOST", "ignored")
	if got := configFromEnv().databaseDSN; got != want {
		t.Fatalf("DSN = %q, want %q", got, want)
	}
}

func TestConfigFromEnvironment(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DB_HOST", "::1")
	t.Setenv("DB_PORT", "5433")
	t.Setenv("DB_USER", "user@name")
	t.Setenv("DB_PASS", "p@ss:/?# word")
	t.Setenv("DB_NAME", "ledger db")
	t.Setenv("REDIS_ADDR", "redis:6380")
	t.Setenv("REDIS_PASSWORD", "cache secret")
	t.Setenv("REDIS_DB", "2")
	cfg := configFromEnv()
	dsn, err := url.Parse(cfg.databaseDSN)
	if err != nil {
		t.Fatal(err)
	}
	password, _ := dsn.User.Password()
	if dsn.Host != "[::1]:5433" || dsn.User.Username() != "user@name" || password != "p@ss:/?# word" || dsn.Path != "/ledger db" || dsn.Query().Get("sslmode") != "disable" {
		t.Fatal("database configuration was not encoded correctly")
	}
	if cfg.redisAddr != "redis:6380" || cfg.redisPassword != "cache secret" || cfg.redisDB != 2 {
		t.Fatal("Redis configuration was not read correctly")
	}
}

func TestInvalidRedisDBFallsBackToZero(t *testing.T) {
	clearConfigEnv(t)
	for _, value := range []string{"", "bad", "-1", "999999999999999999999999"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("REDIS_DB", value)
			if got := configFromEnv().redisDB; got != 0 {
				t.Fatalf("Redis DB = %d, want 0", got)
			}
		})
	}
}
