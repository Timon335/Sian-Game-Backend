package handler

import "net/http"

// Routes ผูกทุกเส้นทางของ API ไว้ที่เดียว
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/genres", h.ListGenres)
	mux.HandleFunc("GET /api/games", h.ListGames)
	mux.HandleFunc("GET /api/games/{id}", h.GetGame)
	return mux
}
