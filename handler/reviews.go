package handler

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"siangame/mw"
	"siangame/store"
)

const maxReviewLength = 2000

// readReview อ่านและตรวจข้อมูลรีวิว กติกานี้ต้องอยู่ที่ server เพราะฟอร์มถูกข้ามได้
func readReview(w http.ResponseWriter, r *http.Request) (int, string, bool) {
	var req struct {
		Rating int    `json:"rating"`
		Text   string `json:"text"`
	}
	if !readJSON(w, r, &req) {
		return 0, "", false
	}
	text := strings.TrimSpace(req.Text)
	switch {
	case req.Rating < 1 || req.Rating > 5:
		writeError(w, http.StatusBadRequest, "BAD_RATING", "คะแนนต้องอยู่ระหว่าง 1 ถึง 5 ดาว")
		return 0, "", false
	case text == "" || utf8.RuneCountInString(text) > maxReviewLength || hasNUL(text):
		writeError(w, http.StatusBadRequest, "BAD_TEXT", "กรุณาเขียนรีวิว และยาวไม่เกิน 2,000 ตัวอักษร")
		return 0, "", false
	}
	return req.Rating, text, true
}

func reviewError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrDuplicateReview):
		writeError(w, http.StatusConflict, "DUPLICATE_REVIEW", "คุณรีวิวเกมนี้แล้ว แก้ไขรีวิวเดิมได้")
	case errors.Is(err, store.ErrNotOwner):
		writeError(w, http.StatusForbidden, "NOT_OWNER", "แก้ไขได้เฉพาะรีวิวของตัวเอง")
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "REVIEW_NOT_FOUND", "ไม่พบรีวิวนี้")
	default:
		internal(w, err)
	}
}

func (h *Handler) ListReviews(w http.ResponseWriter, r *http.Request) {
	gameID, ok := pathID(r, "id")
	if !ok {
		badID(w)
		return
	}
	items, err := h.Store.ListReviews(r.Context(), gameID)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) CreateReview(w http.ResponseWriter, r *http.Request) {
	gameID, ok := pathID(r, "id")
	if !ok {
		badID(w)
		return
	}
	rating, text, ok := readReview(w, r)
	if !ok {
		return
	}
	memberID, _ := mw.MemberID(r.Context())
	review, err := h.Store.CreateReview(r.Context(), memberID, gameID, rating, text)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "GAME_NOT_FOUND", "ไม่พบเกมนี้")
		return
	}
	if err != nil {
		reviewError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, review)
}

func (h *Handler) UpdateReview(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		badID(w)
		return
	}
	rating, text, ok := readReview(w, r)
	if !ok {
		return
	}
	memberID, _ := mw.MemberID(r.Context())
	review, err := h.Store.UpdateReview(r.Context(), id, memberID, rating, text)
	if err != nil {
		reviewError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, review)
}

func (h *Handler) DeleteReview(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		badID(w)
		return
	}
	memberID, _ := mw.MemberID(r.Context())
	if err := h.Store.DeleteReview(r.Context(), id, memberID); err != nil {
		reviewError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
