package store

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testStore เปิดฐานข้อมูลเทสต์ที่ล้างแล้ว ถ้าไม่ตั้ง TEST_DATABASE_URL จะข้ามเทสต์
func testStore(t *testing.T) *Store {
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
	return &Store{Pool: pool}
}

func execOK(t *testing.T, s *Store, sql string, args ...any) {
	t.Helper()
	if _, err := s.Pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func execFails(t *testing.T, s *Store, name, sql string, args ...any) {
	t.Helper()
	if _, err := s.Pool.Exec(context.Background(), sql, args...); err == nil {
		t.Errorf("%s: ต้องถูกปฏิเสธ", name)
	}
}

const insertGame = `INSERT INTO games (title, description_th, developer, price, cover_url) VALUES ($1,'d','dev',$2,'https://x/y.jpg')`
const insertMember = `INSERT INTO members (email, password_hash, display_name) VALUES ($1,'h','n')`

func TestSchemaRejectsBadData(t *testing.T) {
	s := testStore(t)
	execOK(t, s, insertGame, "A", 100)
	execOK(t, s, insertMember, "a@x.test")
	execOK(t, s, `INSERT INTO orders (id, member_id, total) VALUES (1, 1, 100)`)
	execOK(t, s, `INSERT INTO library_items (member_id, game_id, order_id) VALUES (1,1,1)`)
	execOK(t, s, `INSERT INTO reviews (member_id, game_id, rating, text) VALUES (1,1,5,'ดี')`)

	execFails(t, s, "ราคา 0", insertGame, "B", 0)
	execFails(t, s, "อีเมลตัวพิมพ์ต่างกันซ้ำ", insertMember, "A@X.TEST")
	execFails(t, s, "คะแนน 6", `INSERT INTO reviews (member_id, game_id, rating, text) VALUES (1,1,6,'ดี')`)
	execFails(t, s, "รีวิวซ้ำ", `INSERT INTO reviews (member_id, game_id, rating, text) VALUES (1,1,4,'ซ้ำ')`)
	execFails(t, s, "ซื้อเกมซ้ำ", `INSERT INTO library_items (member_id, game_id, order_id) VALUES (1,1,1)`)
}
