package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"siangame/auth"
	"siangame/store"
)

// app คือ server ตัวจริงทั้งชุดบนฐานข้อมูลเทสต์ที่ล้างแล้ว
type app struct {
	t       *testing.T
	store   *store.Store
	tickets *auth.Issuer
	routes  http.Handler
}

func newApp(t *testing.T) *app {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("ไม่ได้ตั้ง TEST_DATABASE_URL")
	}
	if strings.Contains(url, "supabase") {
		t.Fatal("ห้ามรันเทสต์กับฐานข้อมูลบน Supabase")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema, err := os.ReadFile("../db/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(schema)); err != nil {
		t.Fatal(err)
	}
	st := &store.Store{Pool: pool}
	tickets := &auth.Issuer{Secret: []byte("test-secret-test-secret-test-secret"), TTL: time.Hour}
	return &app{t: t, store: st, tickets: tickets, routes: (&Handler{Store: st, Tickets: tickets}).Routes()}
}

func (a *app) exec(sql string, args ...any) {
	a.t.Helper()
	if _, err := a.store.Pool.Exec(context.Background(), sql, args...); err != nil {
		a.t.Fatalf("%s: %v", sql, err)
	}
}

// addGame เพิ่มเกมแล้วคืน id
func (a *app) addGame(title string, price int) int64 {
	a.t.Helper()
	var id int64
	err := a.store.Pool.QueryRow(context.Background(),
		`INSERT INTO games (title, description_th, developer, price, cover_url) VALUES ($1,'คำอธิบาย','dev',$2,'https://x.test/c.jpg') RETURNING id`,
		title, price).Scan(&id)
	if err != nil {
		a.t.Fatal(err)
	}
	return id
}

// addMember เพิ่มสมาชิกแล้วคืน id กับ token
func (a *app) addMember(email, role string) (int64, string) {
	a.t.Helper()
	var id int64
	err := a.store.Pool.QueryRow(context.Background(),
		`INSERT INTO members (email, password_hash, display_name, role) VALUES ($1,'x',$1,$2) RETURNING id`,
		email, role).Scan(&id)
	if err != nil {
		a.t.Fatal(err)
	}
	token, err := a.tickets.Issue(id, role)
	if err != nil {
		a.t.Fatal(err)
	}
	return id, token
}

func (a *app) count(sql string, args ...any) int {
	a.t.Helper()
	var n int
	if err := a.store.Pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		a.t.Fatalf("%s: %v", sql, err)
	}
	return n
}

// call ยิงคำขอเข้า server แล้วคืนผลลัพธ์
func (a *app) call(method, path, token string, body any) *httptest.ResponseRecorder {
	a.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if s, ok := body.(string); ok {
			buf.WriteString(s)
		} else if err := json.NewEncoder(&buf).Encode(body); err != nil {
			a.t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	a.routes.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("อ่าน JSON ไม่ได้: %q", rec.Body.String())
	}
	return v
}

// wantStatus ตรวจ status และคืน error code ถ้าเป็น error แบบ JSON
func wantStatus(t *testing.T, rec *httptest.ResponseRecorder, status int) string {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d ต้องการ %d: %s", rec.Code, status, rec.Body.String())
	}
	if status < 400 {
		return ""
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("error ต้องเป็น JSON แต่ได้ %q", ct)
	}
	e := decode[struct {
		Error struct{ Code, Message string } `json:"error"`
	}](t, rec)
	if e.Error.Message == "" {
		t.Error("error ต้องมีข้อความ")
	}
	return e.Error.Code
}
