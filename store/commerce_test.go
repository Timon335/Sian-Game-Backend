package store

import (
	"context"
	"testing"
)

// เทสต์นี้บังคับให้ขั้นกลางของ transaction พังด้วย trigger ทำให้พิสูจน์ rollback ได้จริง
func TestPlaceOrderRollsBackOnFailure(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	execOK(t, s, insertGame, "A", 100)
	execOK(t, s, insertMember, "a@x.test")
	execOK(t, s, `INSERT INTO cart_items (member_id, game_id) VALUES (1,1)`)
	execOK(t, s, `CREATE FUNCTION boom() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'พังกลางทาง'; END $$ LANGUAGE plpgsql`)
	execOK(t, s, `CREATE TRIGGER boom BEFORE INSERT ON library_items FOR EACH ROW EXECUTE FUNCTION boom()`)

	if _, err := s.PlaceOrder(ctx, 1); err == nil {
		t.Fatal("ต้องคืน error")
	}
	if n := scalar(t, s, `SELECT count(*) FROM orders`); n != 0 {
		t.Errorf("มีคำสั่งซื้อค้าง %d ใบ", n)
	}
	if n := scalar(t, s, `SELECT count(*) FROM order_items`); n != 0 {
		t.Errorf("มีรายการค้าง %d รายการ", n)
	}
	if n := scalar(t, s, `SELECT count(*) FROM cart_items`); n != 1 {
		t.Errorf("ตะกร้าเดิมต้องอยู่ครบ เหลือ %d", n)
	}
}
