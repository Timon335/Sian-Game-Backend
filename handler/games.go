package handler

import (
	"errors"
	"net/http"

	"siangame/store"
)

func (h *Handler) ListGames(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.ListGames(r.Context())
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) GetGame(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		badID(w)
		return
	}
	g, err := h.Store.GetGame(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "GAME_NOT_FOUND", "ไม่พบเกมนี้")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (h *Handler) ListGenres(w http.ResponseWriter, r *http.Request) {
	items, err := h.Store.ListGenres(r.Context())
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
