package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("DATABASE_PATH", "")
	t.Setenv("APP_ENV", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Port != 7200 {
		t.Fatalf("Port = %d", cfg.Port)
	}
	if cfg.DatabasePath != "./data/bff.db" {
		t.Fatalf("DatabasePath = %q", cfg.DatabasePath)
	}

}

func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Setenv("PORT", "not-a-port")

	if _, err := Load(); err == nil {
		t.Fatal("expected invalid PORT error")
	}
}
