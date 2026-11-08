package db

import "testing"

func TestGetCfgPrefersDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://primary.example/hoc_spend?sslmode=require")
	t.Setenv("DB_URL", "postgres://fallback.example/hoc_spend")

	conf, err := getCfg()
	if err != nil {
		t.Fatalf("getCfg() error = %v", err)
	}
	if conf.URL != "postgres://primary.example/hoc_spend?sslmode=require" {
		t.Fatalf("config URL = %q, want DATABASE_URL", conf.URL)
	}
}

func TestGetCfgUsesDBURLFallback(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_URL", "postgres://fallback.example/hoc_spend?sslmode=require")

	conf, err := getCfg()
	if err != nil {
		t.Fatalf("getCfg() error = %v", err)
	}
	if conf.URL != "postgres://fallback.example/hoc_spend?sslmode=require" {
		t.Fatalf("config URL = %q, want DB_URL", conf.URL)
	}
}

func TestGetCfgRejectsMissingURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_URL", "")

	if _, err := getCfg(); err == nil {
		t.Fatal("getCfg() error = nil, want missing URL error")
	}
}
