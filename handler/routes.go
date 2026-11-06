package handler

import (
	"net/http"

	"siangame/mw"
)

// Routes ผูกทุกเส้นทางของ API ไว้ที่เดียว
func (h *Handler) Routes() http.Handler {
	// login = ต้องมีตั๋ว
	login := func(next http.HandlerFunc) http.HandlerFunc { return mw.RequireLogin(h.Tickets, next) }

	// admin = ต้องมีตั๋วและบทบาทเป็น admin ตรวจตั๋วก่อนเสมอแล้วค่อยดูบทบาท
	admin := func(next http.HandlerFunc) http.HandlerFunc { return login(mw.RequireRole("admin", next)) }

	mux := http.NewServeMux()
	// หลังบ้าน: ทุกเส้นทางใต้ /api/admin ผ่าน admin(...) ไม่มีข้อยกเว้น
	mux.HandleFunc("GET /api/admin/stats", admin(h.AdminStats))
	mux.HandleFunc("GET /api/admin/games", admin(h.AdminListGames))
	mux.HandleFunc("POST /api/admin/games", admin(h.AdminCreateGame))
	mux.HandleFunc("GET /api/admin/games/{id}", admin(h.GetGame))
	mux.HandleFunc("PUT /api/admin/games/{id}", admin(h.AdminUpdateGame))
	mux.HandleFunc("DELETE /api/admin/games/{id}", admin(h.AdminStopSelling))
	mux.HandleFunc("GET /api/admin/members", admin(h.AdminMembers))
	mux.HandleFunc("PUT /api/admin/members/{id}/role", admin(h.AdminSetRole))
	mux.HandleFunc("GET /api/admin/orders", admin(h.AdminOrders))
	mux.HandleFunc("GET /api/admin/reviews", admin(h.AdminReviews))
	mux.HandleFunc("DELETE /api/admin/reviews/{id}", admin(h.AdminDeleteReview))

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
