package handler

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"siangame/mw"
	"siangame/store"
)

var httpURL = regexp.MustCompile(`^https?://\S+$`)

func (h *Handler) AdminStats(w http.ResponseWriter, r *http.Request) {
	st, err := h.Store.Stats(r.Context())
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (h *Handler) AdminMembers(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.ListMembers(r.Context())
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// AdminSetRole เปลี่ยนบทบาทคนอื่น แก้ของตัวเองไม่ได้ กันระบบไม่เหลือ Admin
func (h *Handler) AdminSetRole(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		badID(w)
		return
	}
	if me, _ := mw.MemberID(r.Context()); me == id {
		writeError(w, http.StatusBadRequest, "SELF_ROLE", "เปลี่ยนบทบาทของตัวเองไม่ได้")
		return
	}
	var req struct {
		Role string `json:"role"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.Role != "member" && req.Role != "admin" {
		writeError(w, http.StatusBadRequest, "BAD_ROLE", "บทบาทต้องเป็น member หรือ admin")
		return
	}
	m, err := h.Store.SetRole(r.Context(), id, req.Role)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "MEMBER_NOT_FOUND", "ไม่พบสมาชิกนี้")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// readGame อ่านและตรวจข้อมูลเกม ฟอร์มตรวจชั้นแรก แต่ server ต้องตรวจซ้ำเสมอ
func readGame(w http.ResponseWriter, r *http.Request) (store.GameInput, bool) {
	var in store.GameInput
	if !readJSON(w, r, &in) {
		return in, false
	}
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	in.Developer = strings.TrimSpace(in.Developer)
	in.Cover = strings.TrimSpace(in.Cover)

	bad := func(code, message string) (store.GameInput, bool) {
		writeError(w, http.StatusBadRequest, code, message)
		return in, false
	}
	switch {
	case in.Title == "" || len(in.Title) > 255:
		return bad("BAD_TITLE", "ต้องมีชื่อเกม และยาวไม่เกิน 255 ไบต์")
	case in.Description == "":
		return bad("BAD_DESCRIPTION", "ต้องมีคำอธิบายเกม")
	case in.Developer == "" || len(in.Developer) > 255:
		return bad("BAD_DEVELOPER", "ต้องมีชื่อผู้พัฒนา")
	case in.Price <= 0 || in.Price > 99999999:
		return bad("BAD_PRICE", "ราคาต้องมากกว่า 0")
	case !httpURL.MatchString(in.Cover) || len(in.Cover) > 1000:
		return bad("BAD_COVER", "รูปหลักต้องเป็นลิงก์ที่ขึ้นต้นด้วย http:// หรือ https://")
	case len(in.GenreIDs) == 0:
		return bad("BAD_GENRE", "ต้องเลือกแนวเกมอย่างน้อย 1 แนว")
	}
	if in.ReleaseDate != nil && *in.ReleaseDate == "" {
		in.ReleaseDate = nil
	}
	if in.ReleaseDate != nil {
		if _, err := time.Parse("2006-01-02", *in.ReleaseDate); err != nil {
			return bad("BAD_DATE", "วันวางจำหน่ายต้องอยู่ในรูปแบบ YYYY-MM-DD")
		}
	}
	for _, u := range in.Screenshots {
		if !httpURL.MatchString(u) || len(u) > 1000 {
			return bad("BAD_SCREENSHOT", "ภาพตัวอย่างต้องเป็นลิงก์ที่ขึ้นต้นด้วย http:// หรือ https://")
		}
	}
	return in, true
}

func adminGameError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrBadGenre):
		writeError(w, http.StatusBadRequest, "BAD_GENRE", "มีแนวเกมที่ไม่มีอยู่จริง")
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "GAME_NOT_FOUND", "ไม่พบเกมนี้")
	default:
		internal(w, err)
	}
}

func (h *Handler) AdminListGames(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.AdminListGames(r.Context())
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) AdminCreateGame(w http.ResponseWriter, r *http.Request) {
	in, ok := readGame(w, r)
	if !ok {
		return
	}
	g, err := h.Store.CreateGame(r.Context(), in)
	if err != nil {
		adminGameError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

func (h *Handler) AdminUpdateGame(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		badID(w)
		return
	}
	in, ok := readGame(w, r)
	if !ok {
		return
	}
	g, err := h.Store.UpdateGame(r.Context(), id, in)
	if err != nil {
		adminGameError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, g)
}

// AdminStopSelling คือ DELETE ของเกม: เลิกขาย แต่เกมยังอยู่ในคลังของคนที่ซื้อแล้ว
func (h *Handler) AdminStopSelling(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		badID(w)
		return
	}
	if err := h.Store.StopSelling(r.Context(), id); err != nil {
		adminGameError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) AdminOrders(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.AdminOrders(r.Context())
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) AdminReviews(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.AdminReviews(r.Context())
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) AdminDeleteReview(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		badID(w)
		return
	}
	err := h.Store.AdminDeleteReview(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "REVIEW_NOT_FOUND", "ไม่พบรีวิวนี้")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
