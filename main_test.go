package main

import (
	"strings"
	"testing"
)

func TestEnvValueTrimsPastedWhitespace(t *testing.T) {
	t.Setenv("SOME_VALUE", "  value-with-newline\r\n")
	if got := envValue("SOME_VALUE"); got != "value-with-newline" {
		t.Errorf("ได้ %q", got)
	}
}

func TestPoolConfigErrorHidesPassword(t *testing.T) {
	_, err := poolConfig("postgresql://user:SECRET@host:5432/db?sslmode=wrong")
	if err == nil {
		t.Fatal("ค่าที่ผิดต้องคืน error")
	}
	if strings.Contains(err.Error(), "SECRET") {
		t.Errorf("error ห้ามมีรหัสผ่าน: %v", err)
	}
}

func TestPoolConfigBadURLHidesPassword(t *testing.T) {
	_, err := poolConfig("postgresql://user:SEC%ZZRET@host:5432/db")
	if err == nil {
		t.Fatal("URL ที่ผิดต้องคืน error")
	}
	if strings.Contains(err.Error(), "SEC") {
		t.Errorf("error ห้ามมีรหัสผ่าน: %v", err)
	}
}

func TestPoolConfigAcceptsSessionPoolerURL(t *testing.T) {
	cfg, err := poolConfig(envValueOf("postgresql://postgres.ref:pw@aws-0-ap-southeast-1.pooler.supabase.com:5432/postgres?sslmode=require\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnConfig.Host != "aws-0-ap-southeast-1.pooler.supabase.com" {
		t.Errorf("host = %q", cfg.ConnConfig.Host)
	}
}
