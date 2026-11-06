package handler

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"siangame/store"
)

// ทุกเส้นทางหลังบ้านต้องอยู่ในรายการนี้ ถ้าเพิ่มเส้นทางใหม่ให้เพิ่มที่นี่ด้วย
var adminRoutes = [][2]string{
	{"GET", "/api/admin/stats"},
	{"GET", "/api/admin/games"},
	{"POST", "/api/admin/games"},
	{"GET", "/api/admin/games/1"},
	{"PUT", "/api/admin/games/1"},
	{"DELETE", "/api/admin/games/1"},
	{"GET", "/api/admin/members"},
	{"PUT", "/api/admin/members/1/role"},
	{"GET", "/api/admin/orders"},
	{"GET", "/api/admin/reviews"},
	{"DELETE", "/api/admin/reviews/1"},
}

func TestEveryAdminRouteRejectsMember(t *testing.T) {
	a := newApp(t)
	_, tok := a.addMember("m@x.test", "member")
	for _, r := range adminRoutes {
		if code := wantStatus(t, a.call(r[0], r[1], tok, nil), http.StatusForbidden); code != "FORBIDDEN" {
			t.Errorf("%v code = %q", r, code)
		}
	}
}

func TestEveryAdminRouteRejectsGuest(t *testing.T) {
	a := newApp(t)
	for _, r := range adminRoutes {
		if code := wantStatus(t, a.call(r[0], r[1], "", nil), http.StatusUnauthorized); code != "NO_TOKEN" {
			t.Errorf("%v code = %q", r, code)
		}
	}
}

func validGame() map[string]any {
	return map[string]any{
		"title": "เกมใหม่", "description": "คำอธิบาย", "developer": "ทีม", "releaseDate": "2026-01-15",
		"price": 290, "cover": "https://x.test/c.jpg", "genreIds": []int{1}, "screenshots": []string{"https://x.test/1.jpg", "https://x.test/2.jpg"},
	}
}

func TestCreateGame(t *testing.T) {
	a := newApp(t)
	_, tok := a.addMember("a@x.test", "admin")
	a.exec(`INSERT INTO genres (id,name) VALUES (1,'RPG')`)
	rec := a.call("POST", "/api/admin/games", tok, validGame())
	wantStatus(t, rec, http.StatusCreated)
	g := decode[store.Game](t, rec)
	if g.Title != "เกมใหม่" || g.Price != 290 || len(g.Genres) != 1 || len(g.Screenshots) != 2 || !g.IsActive || *g.ReleaseDate != "2026-01-15" {
		t.Errorf("game = %+v", g)
	}
}

func TestCreateGameValidation(t *testing.T) {
	a := newApp(t)
	_, tok := a.addMember("a@x.test", "admin")
	a.exec(`INSERT INTO genres (id,name) VALUES (1,'RPG')`)
	cases := []struct {
		name  string
		field string
		value any
		code  string
	}{
		{"ราคา 0", "price", 0, "BAD_PRICE"},
		{"ราคาติดลบ", "price", -5, "BAD_PRICE"},
		{"ชื่อว่าง", "title", "  ", "BAD_TITLE"},
		{"คำอธิบายว่าง", "description", "", "BAD_DESCRIPTION"},
		{"ผู้พัฒนาว่าง", "developer", "", "BAD_DEVELOPER"},
		{"รูปไม่ใช่ URL", "cover", "javascript:alert(1)", "BAD_COVER"},
		{"วันที่ผิด", "releaseDate", "15/01/2026", "BAD_DATE"},
		{"ไม่มีแนว", "genreIds", []int{}, "BAD_GENRE"},
		{"แนวไม่มีจริง", "genreIds", []int{99}, "BAD_GENRE"},
		{"ภาพไม่ใช่ URL", "screenshots", []string{"ftp://x"}, "BAD_SCREENSHOT"},
	}
	for _, c := range cases {
		body := validGame()
		body[c.field] = c.value
		rec := a.call("POST", "/api/admin/games", tok, body)
		if code := wantStatus(t, rec, http.StatusBadRequest); code != c.code {
			t.Errorf("%s: code = %q", c.name, code)
		}
	}
	if n := a.count(`SELECT count(*) FROM games`); n != 0 {
		t.Errorf("ต้องไม่มีเกมค้าง พบ %d", n)
	}
}

func TestUpdateGameReplacesGenresAndScreenshots(t *testing.T) {
	a := newApp(t)
	_, tok := a.addMember("a@x.test", "admin")
	a.exec(`INSERT INTO genres (id,name) VALUES (1,'RPG'),(2,'แอ็กชัน')`)
	id := decode[store.Game](t, a.call("POST", "/api/admin/games", tok, validGame())).ID
	body := validGame()
	body["title"] = "ชื่อใหม่"
	body["genreIds"] = []int{2}
	body["screenshots"] = []string{"https://x.test/9.jpg"}
	rec := a.call("PUT", fmt.Sprintf("/api/admin/games/%d", id), tok, body)
	wantStatus(t, rec, http.StatusOK)
	g := decode[store.Game](t, rec)
	if g.Title != "ชื่อใหม่" || g.Genres[0] != "แอ็กชัน" || len(g.Screenshots) != 1 {
		t.Errorf("game = %+v", g)
	}
}

func TestUpdateMissingGame404(t *testing.T) {
	a := newApp(t)
	_, tok := a.addMember("a@x.test", "admin")
	a.exec(`INSERT INTO genres (id,name) VALUES (1,'RPG')`)
	wantStatus(t, a.call("PUT", "/api/admin/games/999", tok, validGame()), http.StatusNotFound)
}

func TestStopSellingKeepsLibraryAndHidesFromList(t *testing.T) {
	a := newApp(t)
	_, admin := a.addMember("a@x.test", "admin")
	_, member := a.addMember("m@x.test", "member")
	g := a.addGame("A", 100)
	addCart(t, a, member, g)
	wantStatus(t, a.call("POST", "/api/me/orders", member, nil), http.StatusCreated)

	wantStatus(t, a.call("DELETE", fmt.Sprintf("/api/admin/games/%d", g), admin, nil), http.StatusNoContent)

	if items := decode[struct{ Items []store.Game }](t, a.call("GET", "/api/games", "", nil)).Items; len(items) != 0 {
		t.Errorf("ต้องหายจากร้าน: %+v", items)
	}
	if items := decode[struct{ Items []store.Game }](t, a.call("GET", "/api/me/library", member, nil)).Items; len(items) != 1 {
		t.Errorf("ต้องอยู่ในคลัง: %+v", items)
	}
	if items := decode[struct{ Items []store.Game }](t, a.call("GET", "/api/admin/games", admin, nil)).Items; len(items) != 1 || items[0].IsActive {
		t.Errorf("หลังบ้านต้องเห็นเกมที่เลิกขาย: %+v", items)
	}
}

func TestRestoreGame(t *testing.T) {
	a := newApp(t)
	_, admin := a.addMember("a@x.test", "admin")
	a.exec(`INSERT INTO genres (id,name) VALUES (1,'RPG')`)
	id := decode[store.Game](t, a.call("POST", "/api/admin/games", admin, validGame())).ID
	wantStatus(t, a.call("DELETE", fmt.Sprintf("/api/admin/games/%d", id), admin, nil), http.StatusNoContent)
	body := validGame()
	body["isActive"] = true
	rec := a.call("PUT", fmt.Sprintf("/api/admin/games/%d", id), admin, body)
	wantStatus(t, rec, http.StatusOK)
	if !decode[store.Game](t, rec).IsActive {
		t.Error("ต้องกลับมาขาย")
	}
}

func TestCannotChangeOwnRole400(t *testing.T) {
	a := newApp(t)
	id, tok := a.addMember("a@x.test", "admin")
	rec := a.call("PUT", fmt.Sprintf("/api/admin/members/%d/role", id), tok, map[string]string{"role": "member"})
	if code := wantStatus(t, rec, http.StatusBadRequest); code != "SELF_ROLE" {
		t.Errorf("code = %q", code)
	}
}

func TestChangeRole(t *testing.T) {
	a := newApp(t)
	_, tok := a.addMember("a@x.test", "admin")
	m, _ := a.addMember("m@x.test", "member")
	path := fmt.Sprintf("/api/admin/members/%d/role", m)
	rec := a.call("PUT", path, tok, map[string]string{"role": "admin"})
	wantStatus(t, rec, http.StatusOK)
	if got := decode[store.Member](t, rec); got.Role != "admin" {
		t.Errorf("role = %q", got.Role)
	}
	if code := wantStatus(t, a.call("PUT", path, tok, map[string]string{"role": "root"}), http.StatusBadRequest); code != "BAD_ROLE" {
		t.Errorf("code = %q", code)
	}
	wantStatus(t, a.call("PUT", "/api/admin/members/999/role", tok, map[string]string{"role": "admin"}), http.StatusNotFound)
}

func TestAdminDeleteAnyReview(t *testing.T) {
	a := newApp(t)
	_, admin := a.addMember("a@x.test", "admin")
	_, member := a.addMember("m@x.test", "member")
	g := a.addGame("A", 100)
	id := decode[store.Review](t, a.call("POST", fmt.Sprintf("/api/games/%d/reviews", g), member, map[string]any{"rating": 5, "text": "ดี"})).ID

	rec := a.call("GET", "/api/admin/reviews", admin, nil)
	wantStatus(t, rec, http.StatusOK)
	list := decode[struct{ Items []store.Review }](t, rec).Items
	if len(list) != 1 || list[0].GameTitle != "A" || list[0].Email != "m@x.test" {
		t.Errorf("list = %+v", list)
	}
	wantStatus(t, a.call("DELETE", fmt.Sprintf("/api/admin/reviews/%d", id), admin, nil), http.StatusNoContent)
	if n := a.count(`SELECT count(*) FROM reviews`); n != 0 {
		t.Errorf("เหลือ %d", n)
	}
	wantStatus(t, a.call("DELETE", fmt.Sprintf("/api/admin/reviews/%d", id), admin, nil), http.StatusNotFound)
}

func TestStatsRevenueMatchesOrders(t *testing.T) {
	a := newApp(t)
	_, admin := a.addMember("a@x.test", "admin")
	_, member := a.addMember("m@x.test", "member")
	g1, g2 := a.addGame("A", 100), a.addGame("B", 250)
	addCart(t, a, member, g1)
	addCart(t, a, member, g2)
	wantStatus(t, a.call("POST", "/api/me/orders", member, nil), http.StatusCreated)

	rec := a.call("GET", "/api/admin/stats", admin, nil)
	wantStatus(t, rec, http.StatusOK)
	s := decode[store.Stats](t, rec)
	if s.Games != 2 || s.Members != 2 || s.Admins != 1 || s.Orders != 1 || s.Revenue != 350 {
		t.Errorf("stats = %+v", s)
	}
	if len(s.TopSelling) != 2 || s.TopSelling[0].SalesCount != 1 || len(s.NewestMembers) != 2 {
		t.Errorf("stats lists = %+v", s)
	}
}

func TestAdminOrdersIncludeMember(t *testing.T) {
	a := newApp(t)
	_, admin := a.addMember("a@x.test", "admin")
	_, member := a.addMember("m@x.test", "member")
	g := a.addGame("A", 100)
	addCart(t, a, member, g)
	wantStatus(t, a.call("POST", "/api/me/orders", member, nil), http.StatusCreated)
	rec := a.call("GET", "/api/admin/orders", admin, nil)
	wantStatus(t, rec, http.StatusOK)
	list := decode[struct{ Items []store.Order }](t, rec).Items
	if len(list) != 1 || list[0].Member == nil || list[0].Member.Email != "m@x.test" || len(list[0].Items) != 1 {
		t.Errorf("orders = %+v", list)
	}
}

func TestAdminListsOmitPasswordHash(t *testing.T) {
	a := newApp(t)
	_, admin := a.addMember("a@x.test", "admin")
	a.exec(`UPDATE members SET password_hash='SECRET-HASH'`)
	for _, path := range []string{"/api/admin/members", "/api/admin/stats", "/api/me"} {
		rec := a.call("GET", path, admin, nil)
		wantStatus(t, rec, http.StatusOK)
		if strings.Contains(rec.Body.String(), "SECRET-HASH") {
			t.Errorf("%s ส่ง hash ออกมา", path)
		}
	}
}
