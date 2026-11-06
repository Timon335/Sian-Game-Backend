package handler

import (
	"context"
	"errors"
	"net/http"

	"siangame/mw"
	"siangame/store"
)

// commerceError แปลงข้อผิดพลาดของการซื้อเป็นคำตอบที่หน้าเว็บอ่านได้
func commerceError(w http.ResponseWriter, err error) {
	var inactive *store.ErrGameInactive
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "GAME_NOT_FOUND", "ไม่พบเกมนี้")
	case errors.Is(err, store.ErrCartEmpty):
		writeError(w, http.StatusBadRequest, "CART_EMPTY", "ยังไม่มีเกมในตะกร้า")
	case errors.Is(err, store.ErrAlreadyOwned):
		writeError(w, http.StatusConflict, "ALREADY_OWNED", "คุณมีเกมนี้ในคลังแล้ว")
	case errors.As(err, &inactive):
		writeError(w, http.StatusConflict, "GAME_INACTIVE", inactive.Error())
	default:
		internal(w, err)
	}
}

// gameList ตอบรายการเกมของสมาชิกที่ถือตั๋วอยู่
func gameList(load func(context.Context, int64) ([]store.Game, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := mw.MemberID(r.Context())
		items, err := load(r.Context(), id)
		if err != nil {
			internal(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

// gameAction ทำงานกับเกมตามเลขใน URL แล้วตอบ 204
func gameAction(do func(context.Context, int64, int64) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gameID, ok := pathID(r, "gameId")
		if !ok {
			badID(w)
			return
		}
		id, _ := mw.MemberID(r.Context())
		if err := do(r.Context(), id, gameID); err != nil {
			commerceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// PlaceOrder สร้างคำสั่งซื้อจากตะกร้าของคนที่ถือตั๋ว ไม่อ่าน body เลย
func (h *Handler) PlaceOrder(w http.ResponseWriter, r *http.Request) {
	id, _ := mw.MemberID(r.Context())
	o, err := h.Store.PlaceOrder(r.Context(), id)
	if err != nil {
		commerceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, o)
}

func (h *Handler) ListOrders(w http.ResponseWriter, r *http.Request) {
	id, _ := mw.MemberID(r.Context())
	items, err := h.Store.OrdersOf(r.Context(), id)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	orderID, ok := pathID(r, "id")
	if !ok {
		badID(w)
		return
	}
	id, _ := mw.MemberID(r.Context())
	o, err := h.Store.OrderOf(r.Context(), id, orderID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "ORDER_NOT_FOUND", "ไม่พบคำสั่งซื้อนี้")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, o)
}
