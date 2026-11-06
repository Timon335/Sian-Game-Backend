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
	mux.HandleFunc("GET /api/games/{id}/reviews", h.ListReviews)

	mux.HandleFunc("POST /api/games/{id}/reviews", login(h.CreateReview))
	mux.HandleFunc("PUT /api/reviews/{id}", login(h.UpdateReview))
	mux.HandleFunc("DELETE /api/reviews/{id}", login(h.DeleteReview))
	mux.HandleFunc("GET /api/me", login(h.Me))
	mux.HandleFunc("PUT /api/me", login(h.UpdateMe))
	mux.HandleFunc("PUT /api/me/password", login(h.ChangePassword))
	mux.HandleFunc("GET /api/me/cart", login(gameList(h.Store.CartItems)))
	mux.HandleFunc("PUT /api/me/cart/{gameId}", login(gameAction(h.Store.AddToCart)))
	mux.HandleFunc("DELETE /api/me/cart/{gameId}", login(gameAction(h.Store.RemoveFromCart)))
	mux.HandleFunc("GET /api/me/wishlist", login(gameList(h.Store.Wishlist)))
	mux.HandleFunc("PUT /api/me/wishlist/{gameId}", login(gameAction(h.Store.AddWishlist)))
	mux.HandleFunc("DELETE /api/me/wishlist/{gameId}", login(gameAction(h.Store.RemoveWishlist)))
	mux.HandleFunc("GET /api/me/library", login(gameList(h.Store.Library)))
	mux.HandleFunc("GET /api/me/orders", login(h.ListOrders))
	mux.HandleFunc("POST /api/me/orders", login(h.PlaceOrder))
	mux.HandleFunc("GET /api/me/orders/{id}", login(h.GetOrder))
	return mux
}
