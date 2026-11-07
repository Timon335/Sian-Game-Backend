package handler

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"siangame/store"
)

func TestRegisterRejectsBadEmailFormats(t *testing.T) {
	a := newApp(t)
	for _, email := range []string{"@", "a b@ c", "a@", "@x.test", "a@b", "a\u0000@x.test"} {
		rec := a.call("POST", "/api/auth/register", "", map[string]string{"email": email, "password": "password1", "displayName": "เอ"})
		if code := wantStatus(t, rec, http.StatusBadRequest); code != "BAD_EMAIL" {
			t.Errorf("%q: code = %q", email, code)
		}
	}
}

func TestPasswordOnlySpacesRejected(t *testing.T) {
	a := newApp(t)
	rec := a.call("POST", "/api/auth/register", "", map[string]string{"email": "a@x.test", "password": strings.Repeat(" ", 8), "displayName": "เอ"})
	if code := wantStatus(t, rec, http.StatusBadRequest); code != "BAD_PASSWORD" {
		t.Errorf("code = %q", code)
	}
}

func TestNulCharacterRejectedNot500(t *testing.T) {
	a := newApp(t)
	g := a.addGame("A", 100)
	_, tok := a.addMember("a@x.test", "member")

	rec := a.call("POST", fmt.Sprintf("/api/games/%d/reviews", g), tok, map[string]any{"rating": 5, "text": "ดี\u0000"})
	if code := wantStatus(t, rec, http.StatusBadRequest); code != "BAD_TEXT" {
		t.Errorf("review code = %q", code)
	}
	rec = a.call("PUT", "/api/me", tok, map[string]string{"displayName": "ชื่อ\u0000"})
	if code := wantStatus(t, rec, http.StatusBadRequest); code != "BAD_NAME" {
		t.Errorf("name code = %q", code)
	}
	rec = a.call("POST", "/api/auth/login", "", map[string]string{"email": "a\u0000@x.test", "password": "password1"})
	if code := wantStatus(t, rec, http.StatusUnauthorized); code != "BAD_CREDENTIALS" {
		t.Errorf("login code = %q", code)
	}
}

func TestGameInputOddValuesRejectedNot500(t *testing.T) {
	a := newApp(t)
	_, tok := a.addMember("a@x.test", "admin")
	a.exec(`INSERT INTO genres (id,name) VALUES (1,'RPG')`)
	cases := []struct {
		name  string
		field string
		value any
		code  string
	}{
		{"เลขแนวเกินช่วง", "genreIds", []int64{99999999999}, "BAD_GENRE"},
		{"เลขแนวติดลบ", "genreIds", []int64{-1}, "BAD_GENRE"},
		{"ปี 0", "releaseDate", "0000-01-01", "BAD_DATE"},
		{"ชื่อมี NUL", "title", "เกม\u0000", "BAD_TITLE"},
		{"คำอธิบายมี NUL", "description", "ดี\u0000", "BAD_DESCRIPTION"},
		{"ผู้พัฒนามี NUL", "developer", "ทีม\u0000", "BAD_DEVELOPER"},
	}
	for _, c := range cases {
		body := validGame()
		body[c.field] = c.value
		rec := a.call("POST", "/api/admin/games", tok, body)
		if code := wantStatus(t, rec, http.StatusBadRequest); code != c.code {
			t.Errorf("%s: code = %q", c.name, code)
		}
	}
}

func TestDeletedMemberTokenGets401(t *testing.T) {
	a := newApp(t)
	g := a.addGame("A", 100)
	id, tok := a.addMember("gone@x.test", "member")
	a.exec(`DELETE FROM members WHERE id=$1`, id)

	calls := []struct {
		method, path string
		body         any
	}{
		{"GET", "/api/me", nil},
		{"PUT", "/api/me", map[string]string{"displayName": "ใหม่"}},
		{"PUT", fmt.Sprintf("/api/me/cart/%d", g), nil},
		{"PUT", fmt.Sprintf("/api/me/wishlist/%d", g), nil},
		{"GET", "/api/me/orders", nil},
		{"POST", "/api/me/orders", nil},
		{"POST", fmt.Sprintf("/api/games/%d/reviews", g), map[string]any{"rating": 5, "text": "ดี"}},
	}
	for _, c := range calls {
		rec := a.call(c.method, c.path, tok, c.body)
		if code := wantStatus(t, rec, http.StatusUnauthorized); code != "MEMBER_GONE" {
			t.Errorf("%s %s: code = %q", c.method, c.path, code)
		}
	}
}

func TestPlaceOrderRemovesBoughtGameFromWishlist(t *testing.T) {
	a := newApp(t)
	bought, other := a.addGame("ซื้อแล้ว", 100), a.addGame("ยังอยากได้", 200)
	_, tok := a.addMember("a@x.test", "member")
	wantStatus(t, a.call("PUT", fmt.Sprintf("/api/me/wishlist/%d", bought), tok, nil), http.StatusNoContent)
	wantStatus(t, a.call("PUT", fmt.Sprintf("/api/me/wishlist/%d", other), tok, nil), http.StatusNoContent)
	addCart(t, a, tok, bought)
	wantStatus(t, a.call("POST", "/api/me/orders", tok, nil), http.StatusCreated)

	items := decode[struct{ Items []store.Game }](t, a.call("GET", "/api/me/wishlist", tok, nil)).Items
	if len(items) != 1 || items[0].Title != "ยังอยากได้" {
		t.Errorf("wishlist = %+v", items)
	}
}
