package handler

import (
	"net/http"

	"siangame/mw"
)

// Routes ผูกทุกเส้นทางของ API ไว้ที่เดียว
func (h *Handler) Routes() http.Handler {
	// login = ต้องมีตั๋ว
	login := func(next http.HandlerFunc) http.HandlerFunc { return mw.RequireLogin(h.Tickets, next) }

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/register", h.Register)
	mux.HandleFunc("POST /api/auth/login", h.Login)
	mux.HandleFunc("GET /api/genres", h.ListGenres)
	mux.HandleFunc("GET /api/games", h.ListGames)
	mux.HandleFunc("GET /api/games/{id}", h.GetGame)

	mux.HandleFunc("GET /api/me", login(h.Me))
	mux.HandleFunc("PUT /api/me", login(h.UpdateMe))
	mux.HandleFunc("PUT /api/me/password", login(h.ChangePassword))
	return mux
}
