package handler

import (
	"fmt"
	"net/http"
	"testing"

	"siangame/store"
)

func TestListGamesHidesInactive(t *testing.T) {
	a := newApp(t)
	a.addGame("ขายอยู่", 100)
	off := a.addGame("เลิกขาย", 100)
	a.exec(`UPDATE games SET is_active=false WHERE id=$1`, off)

	rec := a.call("GET", "/api/games", "", nil)
	wantStatus(t, rec, http.StatusOK)
	got := decode[struct{ Items []store.Game }](t, rec)
	if len(got.Items) != 1 || got.Items[0].Title != "ขายอยู่" {
		t.Errorf("items = %+v", got.Items)
	}
}

func TestListGamesIncludesStats(t *testing.T) {
	a := newApp(t)
	g := a.addGame("A", 100)
	m, _ := a.addMember("a@x.test", "member")
	m2, _ := a.addMember("b@x.test", "member")
	a.exec(`INSERT INTO reviews (member_id, game_id, rating, text) VALUES ($1,$2,5,'ดี'),($3,$2,4,'ดี')`, m, g, m2)
	a.exec(`INSERT INTO orders (id, member_id, total) VALUES (1,$1,100)`, m)
	a.exec(`INSERT INTO order_items VALUES (1,$1,100)`, g)
	a.exec(`INSERT INTO game_screenshots VALUES ($1,'https://x.test/1.jpg',0)`, g)
	a.exec(`INSERT INTO genres (id,name) VALUES (1,'RPG')`)
	a.exec(`INSERT INTO game_genres VALUES ($1,1)`, g)

	rec := a.call("GET", "/api/games", "", nil)
	wantStatus(t, rec, http.StatusOK)
	it := decode[struct{ Items []store.Game }](t, rec).Items[0]
	if it.RatingAverage != 4.5 || it.RatingCount != 2 || it.SalesCount != 1 {
		t.Errorf("สถิติผิด: %+v", it)
	}
	if len(it.Genres) != 1 || it.Genres[0] != "RPG" || len(it.Screenshots) != 1 {
		t.Errorf("แนวหรือภาพผิด: %+v", it)
	}
}

func TestGetGameInactiveStillReturns(t *testing.T) {
	a := newApp(t)
	g := a.addGame("เลิกขาย", 100)
	a.exec(`UPDATE games SET is_active=false WHERE id=$1`, g)
	rec := a.call("GET", fmt.Sprintf("/api/games/%d", g), "", nil)
	wantStatus(t, rec, http.StatusOK)
	if decode[store.Game](t, rec).IsActive {
		t.Error("isActive ต้องเป็น false")
	}
}

func TestGetGameMissing404JSON(t *testing.T) {
	a := newApp(t)
	if code := wantStatus(t, a.call("GET", "/api/games/999", "", nil), http.StatusNotFound); code != "GAME_NOT_FOUND" {
		t.Errorf("code = %q", code)
	}
}

func TestBadIDReturns400(t *testing.T) {
	a := newApp(t)
	if code := wantStatus(t, a.call("GET", "/api/games/abc", "", nil), http.StatusBadRequest); code != "BAD_ID" {
		t.Errorf("code = %q", code)
	}
}

func TestGenresList(t *testing.T) {
	a := newApp(t)
	a.exec(`INSERT INTO genres (name) VALUES ('RPG')`)
	rec := a.call("GET", "/api/genres", "", nil)
	wantStatus(t, rec, http.StatusOK)
	if got := decode[struct{ Items []store.Genre }](t, rec); len(got.Items) != 1 || got.Items[0].Name != "RPG" {
		t.Errorf("items = %+v", got.Items)
	}
}
