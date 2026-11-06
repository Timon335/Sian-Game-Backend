package handler

import (
	"fmt"
	"net/http"
	"testing"

	"siangame/store"
)

func postReview(a *app, game int64, tok string, rating int, text string) *reviewResult {
	rec := a.call("POST", fmt.Sprintf("/api/games/%d/reviews", game), tok, map[string]any{"rating": rating, "text": text})
	return &reviewResult{rec.Code, rec.Body.String()}
}

type reviewResult struct {
	code int
	body string
}

func TestReviewWithoutPurchaseAllowed(t *testing.T) {
	a := newApp(t)
	g := a.addGame("A", 100)
	_, tok := a.addMember("a@x.test", "member")
	rec := a.call("POST", fmt.Sprintf("/api/games/%d/reviews", g), tok, map[string]any{"rating": 5, "text": "  สนุกมาก  "})
	wantStatus(t, rec, http.StatusCreated)
	r := decode[store.Review](t, rec)
	if r.Text != "สนุกมาก" || r.Rating != 5 || r.DisplayName == "" {
		t.Errorf("review = %+v", r)
	}
}

func TestReviewNeedsLogin(t *testing.T) {
	a := newApp(t)
	g := a.addGame("A", 100)
	wantStatus(t, a.call("POST", fmt.Sprintf("/api/games/%d/reviews", g), "", map[string]any{"rating": 5, "text": "ดี"}), http.StatusUnauthorized)
}

func TestDuplicateReview409(t *testing.T) {
	a := newApp(t)
	g := a.addGame("A", 100)
	_, tok := a.addMember("a@x.test", "member")
	postReview(a, g, tok, 5, "ดี")
	rec := a.call("POST", fmt.Sprintf("/api/games/%d/reviews", g), tok, map[string]any{"rating": 4, "text": "ซ้ำ"})
	if code := wantStatus(t, rec, http.StatusConflict); code != "DUPLICATE_REVIEW" {
		t.Errorf("code = %q", code)
	}
}

func TestRating6Rejected400(t *testing.T) {
	a := newApp(t)
	g := a.addGame("A", 100)
	_, tok := a.addMember("a@x.test", "member")
	for _, rating := range []int{0, 6} {
		rec := a.call("POST", fmt.Sprintf("/api/games/%d/reviews", g), tok, map[string]any{"rating": rating, "text": "ดี"})
		if code := wantStatus(t, rec, http.StatusBadRequest); code != "BAD_RATING" {
			t.Errorf("rating %d: code = %q", rating, code)
		}
	}
}

func TestBlankTextRejected400(t *testing.T) {
	a := newApp(t)
	g := a.addGame("A", 100)
	_, tok := a.addMember("a@x.test", "member")
	rec := a.call("POST", fmt.Sprintf("/api/games/%d/reviews", g), tok, map[string]any{"rating": 5, "text": "   "})
	if code := wantStatus(t, rec, http.StatusBadRequest); code != "BAD_TEXT" {
		t.Errorf("code = %q", code)
	}
}

func TestReviewMissingGame404(t *testing.T) {
	a := newApp(t)
	_, tok := a.addMember("a@x.test", "member")
	rec := a.call("POST", "/api/games/999/reviews", tok, map[string]any{"rating": 5, "text": "ดี"})
	if code := wantStatus(t, rec, http.StatusNotFound); code != "GAME_NOT_FOUND" {
		t.Errorf("code = %q", code)
	}
}

func TestEditOwnReview(t *testing.T) {
	a := newApp(t)
	g := a.addGame("A", 100)
	_, tok := a.addMember("a@x.test", "member")
	rec := a.call("POST", fmt.Sprintf("/api/games/%d/reviews", g), tok, map[string]any{"rating": 5, "text": "ดี"})
	id := decode[store.Review](t, rec).ID
	rec = a.call("PUT", fmt.Sprintf("/api/reviews/%d", id), tok, map[string]any{"rating": 3, "text": "เฉย ๆ"})
	wantStatus(t, rec, http.StatusOK)
	if r := decode[store.Review](t, rec); r.Rating != 3 || r.Text != "เฉย ๆ" {
		t.Errorf("review = %+v", r)
	}
}

func TestEditOtherReview403(t *testing.T) {
	a := newApp(t)
	g := a.addGame("A", 100)
	_, tok1 := a.addMember("a@x.test", "member")
	_, tok2 := a.addMember("b@x.test", "member")
	id := decode[store.Review](t, a.call("POST", fmt.Sprintf("/api/games/%d/reviews", g), tok1, map[string]any{"rating": 5, "text": "ดี"})).ID

	if code := wantStatus(t, a.call("PUT", fmt.Sprintf("/api/reviews/%d", id), tok2, map[string]any{"rating": 1, "text": "แก้"}), http.StatusForbidden); code != "NOT_OWNER" {
		t.Errorf("code = %q", code)
	}
	if code := wantStatus(t, a.call("DELETE", fmt.Sprintf("/api/reviews/%d", id), tok2, nil), http.StatusForbidden); code != "NOT_OWNER" {
		t.Errorf("code = %q", code)
	}
	if n := a.count(`SELECT count(*) FROM reviews WHERE rating=5`); n != 1 {
		t.Error("รีวิวต้องไม่เปลี่ยน")
	}
}

func TestDeleteOwnReview(t *testing.T) {
	a := newApp(t)
	g := a.addGame("A", 100)
	_, tok := a.addMember("a@x.test", "member")
	id := decode[store.Review](t, a.call("POST", fmt.Sprintf("/api/games/%d/reviews", g), tok, map[string]any{"rating": 5, "text": "ดี"})).ID
	wantStatus(t, a.call("DELETE", fmt.Sprintf("/api/reviews/%d", id), tok, nil), http.StatusNoContent)
	if n := a.count(`SELECT count(*) FROM reviews`); n != 0 {
		t.Errorf("เหลือ %d", n)
	}
	wantStatus(t, a.call("DELETE", fmt.Sprintf("/api/reviews/%d", id), tok, nil), http.StatusNotFound)
}

func TestReviewsListAndRatingAverage(t *testing.T) {
	a := newApp(t)
	g := a.addGame("A", 100)
	_, tok1 := a.addMember("a@x.test", "member")
	_, tok2 := a.addMember("b@x.test", "member")
	postReview(a, g, tok1, 5, "ดี")
	postReview(a, g, tok2, 2, "ไม่ชอบ")

	rec := a.call("GET", fmt.Sprintf("/api/games/%d/reviews", g), "", nil)
	wantStatus(t, rec, http.StatusOK)
	if items := decode[struct{ Items []store.Review }](t, rec).Items; len(items) != 2 {
		t.Errorf("items = %+v", items)
	}
	rec = a.call("GET", fmt.Sprintf("/api/games/%d", g), "", nil)
	if got := decode[store.Game](t, rec); got.RatingAverage != 3.5 || got.RatingCount != 2 {
		t.Errorf("game = %+v", got)
	}
}
