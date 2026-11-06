package store

import (
	"context"
	"os"
	"testing"
)

func loadSeed(t *testing.T, s *Store) {
	t.Helper()
	seed, err := os.ReadFile("../db/seed.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Pool.Exec(context.Background(), string(seed)); err != nil {
		t.Fatal(err)
	}
}

func scalar(t *testing.T, s *Store, sql string) int {
	t.Helper()
	var n int
	if err := s.Pool.QueryRow(context.Background(), sql).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

func TestSeedConsistent(t *testing.T) {
	s := testStore(t)
	loadSeed(t, s)

	if n := scalar(t, s, `SELECT count(*) FROM games WHERE is_active`); n < 25 {
		t.Errorf("เกมมี %d ต้องอย่างน้อย 25", n)
	}
	if n := scalar(t, s, `SELECT count(*) FROM members WHERE role='admin'`); n < 1 {
		t.Error("ต้องมี Admin อย่างน้อย 1 คน")
	}
	if n := scalar(t, s, `SELECT count(*) FROM orders o WHERE o.total <>
		(SELECT sum(i.price) FROM order_items i WHERE i.order_id = o.id)`); n != 0 {
		t.Errorf("คำสั่งซื้อที่ยอดไม่ตรง %d ใบ", n)
	}
	if n := scalar(t, s, `SELECT count(*) FROM order_items i JOIN orders o ON o.id=i.order_id
		LEFT JOIN library_items l ON l.member_id=o.member_id AND l.game_id=i.game_id
		WHERE l.game_id IS NULL`); n != 0 {
		t.Errorf("รายการซื้อที่ไม่อยู่ในคลัง %d รายการ", n)
	}
	if n := scalar(t, s, `SELECT count(*) FROM games g WHERE NOT EXISTS
		(SELECT 1 FROM game_screenshots x WHERE x.game_id=g.id)`); n != 0 {
		t.Errorf("เกมที่ไม่มีภาพตัวอย่าง %d เกม", n)
	}
	if n := scalar(t, s, `SELECT count(*) FROM games g WHERE NOT EXISTS
		(SELECT 1 FROM game_genres x WHERE x.game_id=g.id)`); n != 0 {
		t.Errorf("เกมที่ไม่มีแนว %d เกม", n)
	}
}
