package main

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
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

func TestConnectErrorHidesPassword(t *testing.T) {
	cfg, err := poolConfig("postgresql://user:SECRETPW@127.0.0.1:1/db?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	err = pingDatabase(context.Background(), pool)
	if err == nil {
		t.Fatal("ต่อพอร์ตที่ไม่มี server ต้องคืน error")
	}
	if strings.Contains(err.Error(), "SECRETPW") {
		t.Errorf("error ห้ามมีรหัสผ่าน: %v", err)
	}
	if !strings.Contains(err.Error(), "ต่อฐานข้อมูลไม่ติด") {
		t.Errorf("error ต้องบอกเรื่องการเชื่อมต่อ: %v", err)
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
