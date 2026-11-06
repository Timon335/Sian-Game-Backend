package handler

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"siangame/store"
)

func addCart(t *testing.T, a *app, tok string, game int64) {
	t.Helper()
	wantStatus(t, a.call("PUT", fmt.Sprintf("/api/me/cart/%d", game), tok, nil), http.StatusNoContent)
}

func TestCartAddListRemove(t *testing.T) {
	a := newApp(t)
	g := a.addGame("A", 100)
	_, tok := a.addMember("a@x.test", "member")
	addCart(t, a, tok, g)
	addCart(t, a, tok, g) // เพิ่มซ้ำต้องไม่พัง
	rec := a.call("GET", "/api/me/cart", tok, nil)
	wantStatus(t, rec, http.StatusOK)
	if items := decode[struct{ Items []store.Game }](t, rec).Items; len(items) != 1 || items[0].Title != "A" {
		t.Errorf("items = %+v", items)
	}
	wantStatus(t, a.call("DELETE", fmt.Sprintf("/api/me/cart/%d", g), tok, nil), http.StatusNoContent)
	if n := a.count(`SELECT count(*) FROM cart_items`); n != 0 {
		t.Errorf("เหลือ %d รายการ", n)
	}
}

func TestCartMissingGame404(t *testing.T) {
	a := newApp(t)
	_, tok := a.addMember("a@x.test", "member")
	if code := wantStatus(t, a.call("PUT", "/api/me/cart/999", tok, nil), http.StatusNotFound); code != "GAME_NOT_FOUND" {
		t.Errorf("code = %q", code)
	}
}

func TestCartOtherMemberIsolated(t *testing.T) {
	a := newApp(t)
	g := a.addGame("A", 100)
	_, tok1 := a.addMember("a@x.test", "member")
	_, tok2 := a.addMember("b@x.test", "member")
	addCart(t, a, tok1, g)
	rec := a.call("GET", "/api/me/cart", tok2, nil)
	if items := decode[struct{ Items []store.Game }](t, rec).Items; len(items) != 0 {
		t.Errorf("เห็นตะกร้าคนอื่น: %+v", items)
	}
}

func TestPlaceOrderMovesCartToLibrary(t *testing.T) {
	a := newApp(t)
	g1, g2 := a.addGame("A", 100), a.addGame("B", 250)
	m, tok := a.addMember("a@x.test", "member")
	addCart(t, a, tok, g1)
	addCart(t, a, tok, g2)

	rec := a.call("POST", "/api/me/orders", tok, nil)
	wantStatus(t, rec, http.StatusCreated)
	o := decode[store.Order](t, rec)
	if o.Total != 350 || len(o.Items) != 2 {
		t.Errorf("order = %+v", o)
	}
	if n := a.count(`SELECT count(*) FROM cart_items WHERE member_id=$1`, m); n != 0 {
		t.Errorf("ตะกร้าเหลือ %d", n)
	}
	rec = a.call("GET", "/api/me/library", tok, nil)
	wantStatus(t, rec, http.StatusOK)
	if items := decode[struct{ Items []store.Game }](t, rec).Items; len(items) != 2 {
		t.Errorf("คลัง = %+v", items)
	}
}

func TestPlaceOrderKeepsPurchasePrice(t *testing.T) {
	a := newApp(t)
	g := a.addGame("A", 100)
	_, tok := a.addMember("a@x.test", "member")
	addCart(t, a, tok, g)
	wantStatus(t, a.call("POST", "/api/me/orders", tok, nil), http.StatusCreated)
	a.exec(`UPDATE games SET price=999 WHERE id=$1`, g)
	rec := a.call("GET", "/api/me/orders", tok, nil)
	wantStatus(t, rec, http.StatusOK)
	o := decode[struct{ Items []store.Order }](t, rec).Items[0]
	if o.Total != 100 || o.Items[0].Price != 100 {
		t.Errorf("ราคาเปลี่ยนตาม: %+v", o)
	}
}

func TestPlaceOrderEmptyCart400(t *testing.T) {
	a := newApp(t)
	_, tok := a.addMember("a@x.test", "member")
	if code := wantStatus(t, a.call("POST", "/api/me/orders", tok, nil), http.StatusBadRequest); code != "CART_EMPTY" {
		t.Errorf("code = %q", code)
	}
}

func TestPlaceOrderTwiceConcurrently(t *testing.T) {
	a := newApp(t)
	g := a.addGame("A", 100)
	_, tok := a.addMember("a@x.test", "member")
	addCart(t, a, tok, g)

	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = a.call("POST", "/api/me/orders", tok, nil).Code
		}()
	}
	wg.Wait()
	if n := a.count(`SELECT count(*) FROM orders`); n != 1 {
		t.Errorf("คำสั่งซื้อ %d ใบ status=%v", n, codes)
	}
	if n := a.count(`SELECT count(*) FROM library_items`); n != 1 {
		t.Errorf("คลัง %d รายการ", n)
	}
	for _, c := range codes {
		if c != http.StatusCreated && c != http.StatusBadRequest && c != http.StatusConflict {
			t.Errorf("status แปลก %v", codes)
		}
	}
}

func TestPlaceOrderInactiveGame409NamesTitle(t *testing.T) {
	a := newApp(t)
	g := a.addGame("เกมเลิกขาย", 100)
	m, tok := a.addMember("a@x.test", "member")
	addCart(t, a, tok, g)
	a.exec(`UPDATE games SET is_active=false WHERE id=$1`, g)

	rec := a.call("POST", "/api/me/orders", tok, nil)
	if code := wantStatus(t, rec, http.StatusConflict); code != "GAME_INACTIVE" {
		t.Errorf("code = %q", code)
	}
	if !strings.Contains(rec.Body.String(), "เกมเลิกขาย") {
		t.Errorf("ต้องบอกชื่อเกม: %s", rec.Body.String())
	}
	if n := a.count(`SELECT count(*) FROM cart_items WHERE member_id=$1`, m); n != 1 {
		t.Error("ตะกร้าเดิมต้องยังอยู่")
	}
	if n := a.count(`SELECT count(*) FROM orders`); n != 0 {
		t.Error("ต้องไม่มีคำสั่งซื้อค้าง")
	}
}

func TestAddOwnedGameToCart409(t *testing.T) {
	a := newApp(t)
	g := a.addGame("A", 100)
	_, tok := a.addMember("a@x.test", "member")
	addCart(t, a, tok, g)
	wantStatus(t, a.call("POST", "/api/me/orders", tok, nil), http.StatusCreated)
	if code := wantStatus(t, a.call("PUT", fmt.Sprintf("/api/me/cart/%d", g), tok, nil), http.StatusConflict); code != "ALREADY_OWNED" {
		t.Errorf("code = %q", code)
	}
}

func TestAddInactiveGameToCart409(t *testing.T) {
	a := newApp(t)
	g := a.addGame("A", 100)
	_, tok := a.addMember("a@x.test", "member")
	a.exec(`UPDATE games SET is_active=false WHERE id=$1`, g)
	if code := wantStatus(t, a.call("PUT", fmt.Sprintf("/api/me/cart/%d", g), tok, nil), http.StatusConflict); code != "GAME_INACTIVE" {
		t.Errorf("code = %q", code)
	}
}

func TestOrderPriceIgnoresBody(t *testing.T) {
	a := newApp(t)
	g := a.addGame("A", 100)
	_, tok := a.addMember("a@x.test", "member")
	addCart(t, a, tok, g)
	rec := a.call("POST", "/api/me/orders", tok, `{"total":1,"price":1,"cardNumber":"4242"}`)
	wantStatus(t, rec, http.StatusCreated)
	if o := decode[store.Order](t, rec); o.Total != 100 {
		t.Errorf("ยอด = %v", o.Total)
	}
}

func TestOrderOfOtherMember404(t *testing.T) {
	a := newApp(t)
	g := a.addGame("A", 100)
	_, tok1 := a.addMember("a@x.test", "member")
	_, tok2 := a.addMember("b@x.test", "member")
	addCart(t, a, tok1, g)
	rec := a.call("POST", "/api/me/orders", tok1, nil)
	id := decode[store.Order](t, rec).ID
	wantStatus(t, a.call("GET", fmt.Sprintf("/api/me/orders/%d", id), tok1, nil), http.StatusOK)
	if code := wantStatus(t, a.call("GET", fmt.Sprintf("/api/me/orders/%d", id), tok2, nil), http.StatusNotFound); code != "ORDER_NOT_FOUND" {
		t.Errorf("code = %q", code)
	}
}

func TestLibraryKeepsStoppedGame(t *testing.T) {
	a := newApp(t)
	g := a.addGame("A", 100)
	_, tok := a.addMember("a@x.test", "member")
	addCart(t, a, tok, g)
	wantStatus(t, a.call("POST", "/api/me/orders", tok, nil), http.StatusCreated)
	a.exec(`UPDATE games SET is_active=false WHERE id=$1`, g)
	rec := a.call("GET", "/api/me/library", tok, nil)
	if items := decode[struct{ Items []store.Game }](t, rec).Items; len(items) != 1 {
		t.Errorf("เกมต้องยังอยู่ในคลัง: %+v", items)
	}
}

func TestWishlistAddListRemove(t *testing.T) {
	a := newApp(t)
	g := a.addGame("A", 100)
	_, tok := a.addMember("a@x.test", "member")
	path := fmt.Sprintf("/api/me/wishlist/%d", g)
	wantStatus(t, a.call("PUT", path, tok, nil), http.StatusNoContent)
	wantStatus(t, a.call("PUT", path, tok, nil), http.StatusNoContent)
	rec := a.call("GET", "/api/me/wishlist", tok, nil)
	if items := decode[struct{ Items []store.Game }](t, rec).Items; len(items) != 1 {
		t.Errorf("items = %+v", items)
	}
	wantStatus(t, a.call("DELETE", path, tok, nil), http.StatusNoContent)
	if n := a.count(`SELECT count(*) FROM wishlist_items`); n != 0 {
		t.Errorf("เหลือ %d", n)
	}
}

func TestMemberRoutesRequireToken(t *testing.T) {
	a := newApp(t)
	for _, r := range [][2]string{{"GET", "/api/me/cart"}, {"PUT", "/api/me/cart/1"}, {"POST", "/api/me/orders"}, {"GET", "/api/me/orders"}, {"GET", "/api/me/library"}, {"GET", "/api/me/wishlist"}} {
		if code := wantStatus(t, a.call(r[0], r[1], "", nil), http.StatusUnauthorized); code != "NO_TOKEN" {
			t.Errorf("%v: code = %q", r, code)
		}
	}
}
