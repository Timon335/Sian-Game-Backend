package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	// ErrCartEmpty หมายถึงไม่มีเกมในตะกร้า
	ErrCartEmpty = errors.New("cart empty")
	// ErrAlreadyOwned หมายถึงมีเกมนี้ในคลังแล้ว
	ErrAlreadyOwned = errors.New("already owned")
)

// ErrGameInactive บอกชื่อเกมที่เลิกขายแล้ว
type ErrGameInactive struct {
	Titles []string
}

func (e *ErrGameInactive) Error() string {
	return "เกมเลิกจำหน่ายแล้ว: " + strings.Join(e.Titles, ", ")
}

// OrderItem คือเกมหนึ่งรายการในคำสั่งซื้อ พร้อมราคา ณ วันที่ซื้อ
type OrderItem struct {
	GameID int64   `json:"gameId"`
	Title  string  `json:"title"`
	Cover  string  `json:"cover"`
	Price  float64 `json:"price"`
}

// OrderMember คือเจ้าของคำสั่งซื้อ ใช้เฉพาะหน้าหลังบ้าน
type OrderMember struct {
	ID          int64  `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}

// Order คือคำสั่งซื้อหนึ่งใบ
type Order struct {
	ID        int64        `json:"id"`
	Total     float64      `json:"total"`
	CreatedAt time.Time    `json:"createdAt"`
	Items     []OrderItem  `json:"items"`
	Member    *OrderMember `json:"member,omitempty"`
}

// ---------- ตะกร้า ----------

// CartItems คืนเกมในตะกร้า รวมเกมที่เพิ่งเลิกขายเพื่อให้หน้าเว็บแจ้งผู้ใช้ได้
func (s *Store) CartItems(ctx context.Context, memberID int64) ([]Game, error) {
	return s.queryGames(ctx, gameSelect+` JOIN cart_items c ON c.game_id = g.id
		WHERE c.member_id = $1 ORDER BY c.added_at, g.id`, memberID)
}

// AddToCart ใส่เกมลงตะกร้า เกมเดิมซ้ำไม่ถือว่าผิด
func (s *Store) AddToCart(ctx context.Context, memberID, gameID int64) error {
	var title string
	var active, owned bool
	err := s.Pool.QueryRow(ctx,
		`SELECT g.title, g.is_active,
		        EXISTS (SELECT 1 FROM library_items l WHERE l.member_id = $1 AND l.game_id = g.id)
		 FROM games g WHERE g.id = $2`, memberID, gameID).Scan(&title, &active, &owned)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !active {
		return &ErrGameInactive{Titles: []string{title}}
	}
	if owned {
		return ErrAlreadyOwned
	}
	_, err = s.Pool.Exec(ctx,
		`INSERT INTO cart_items (member_id, game_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, memberID, gameID)
	return err
}

// RemoveFromCart เอาเกมออกจากตะกร้า
func (s *Store) RemoveFromCart(ctx context.Context, memberID, gameID int64) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM cart_items WHERE member_id = $1 AND game_id = $2`, memberID, gameID)
	return err
}

// ---------- คำสั่งซื้อ ----------

// PlaceOrder ย้ายตะกร้าเป็นคำสั่งซื้อและคลังเกมในก้อนเดียว สำเร็จทั้งหมดหรือไม่เกิดขึ้นเลย
// ราคาอ่านจากฐานข้อมูลเท่านั้น ไม่เชื่อค่าจากหน้าเว็บ
func (s *Store) PlaceOrder(ctx context.Context, memberID int64) (Order, error) {
	var o Order
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return o, err
	}
	defer tx.Rollback(ctx) // ถ้า commit แล้วบรรทัดนี้ไม่ทำอะไร จึงปลอดภัยเสมอ

	rows, err := tx.Query(ctx,
		`SELECT g.id, g.title, g.cover_url, g.price::float8, g.is_active,
		        EXISTS (SELECT 1 FROM library_items l WHERE l.member_id = $1 AND l.game_id = g.id)
		 FROM cart_items c JOIN games g ON g.id = c.game_id
		 WHERE c.member_id = $1 ORDER BY c.added_at, g.id`, memberID)
	if err != nil {
		return o, err
	}
	var inactive []string
	var owned bool
	for rows.Next() {
		var it OrderItem
		var active, isOwned bool
		if err := rows.Scan(&it.GameID, &it.Title, &it.Cover, &it.Price, &active, &isOwned); err != nil {
			rows.Close()
			return o, err
		}
		if !active {
			inactive = append(inactive, it.Title)
		}
		owned = owned || isOwned
		o.Items = append(o.Items, it)
		o.Total += it.Price
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return o, err
	}
	switch {
	case len(o.Items) == 0:
		return o, ErrCartEmpty
	case len(inactive) > 0:
		return o, &ErrGameInactive{Titles: inactive}
	case owned:
		return o, ErrAlreadyOwned
	}

	err = tx.QueryRow(ctx,
		`INSERT INTO orders (member_id, total) VALUES ($1,$2) RETURNING id, created_at`, memberID, o.Total).
		Scan(&o.ID, &o.CreatedAt)
	if err != nil {
		return o, err
	}
	for _, it := range o.Items {
		if _, err := tx.Exec(ctx, `INSERT INTO order_items (order_id, game_id, price) VALUES ($1,$2,$3)`, o.ID, it.GameID, it.Price); err != nil {
			return o, err
		}
		_, err := tx.Exec(ctx, `INSERT INTO library_items (member_id, game_id, order_id) VALUES ($1,$2,$3)`, memberID, it.GameID, o.ID)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return o, ErrAlreadyOwned // กดจ่ายซ้ำพร้อมกัน PK ของคลังเกมจะกันไว้
		}
		if err != nil {
			return o, err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM cart_items WHERE member_id = $1`, memberID); err != nil {
		return o, err
	}
	// เกมที่ซื้อแล้วไม่ต้องอยู่ในรายการอยากได้อีก
	_, err = tx.Exec(ctx,
		`DELETE FROM wishlist_items WHERE member_id = $1
		 AND game_id IN (SELECT game_id FROM order_items WHERE order_id = $2)`, memberID, o.ID)
	if err != nil {
		return o, err
	}
	return o, tx.Commit(ctx)
}

// attachItems เติมรายการเกมให้คำสั่งซื้อหลายใบในคำขอเดียว
func (s *Store) attachItems(ctx context.Context, orders []Order) error {
	if len(orders) == 0 {
		return nil
	}
	ids := make([]int64, len(orders))
	index := map[int64]int{}
	for i, o := range orders {
		ids[i] = o.ID
		index[o.ID] = i
	}
	rows, err := s.Pool.Query(ctx,
		`SELECT i.order_id, i.game_id, g.title, g.cover_url, i.price::float8
		 FROM order_items i JOIN games g ON g.id = i.game_id
		 WHERE i.order_id = ANY($1) ORDER BY i.order_id, g.title`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var orderID int64
		var it OrderItem
		if err := rows.Scan(&orderID, &it.GameID, &it.Title, &it.Cover, &it.Price); err != nil {
			return err
		}
		o := &orders[index[orderID]]
		o.Items = append(o.Items, it)
	}
	return rows.Err()
}

// OrdersOf คืนคำสั่งซื้อของสมาชิกคนนี้ ใหม่สุดก่อน
func (s *Store) OrdersOf(ctx context.Context, memberID int64) ([]Order, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, total::float8, created_at FROM orders WHERE member_id = $1 ORDER BY created_at DESC, id DESC`, memberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Order{}
	for rows.Next() {
		o := Order{Items: []OrderItem{}}
		if err := rows.Scan(&o.ID, &o.Total, &o.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, s.attachItems(ctx, out)
}

// OrderOf คืนคำสั่งซื้อใบเดียว เฉพาะของสมาชิกคนนี้ ของคนอื่นถือว่าไม่พบ
func (s *Store) OrderOf(ctx context.Context, memberID, orderID int64) (Order, error) {
	o := Order{Items: []OrderItem{}}
	err := s.Pool.QueryRow(ctx,
		`SELECT id, total::float8, created_at FROM orders WHERE id = $1 AND member_id = $2`, orderID, memberID).
		Scan(&o.ID, &o.Total, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return o, ErrNotFound
	}
	if err != nil {
		return o, err
	}
	list := []Order{o}
	err = s.attachItems(ctx, list)
	return list[0], err
}

// ---------- คลังเกมและรายการอยากได้ ----------

// Library คืนเกมที่สมาชิกซื้อแล้ว รวมเกมที่เลิกขายภายหลัง
func (s *Store) Library(ctx context.Context, memberID int64) ([]Game, error) {
	return s.queryGames(ctx, gameSelect+` JOIN library_items l ON l.game_id = g.id
		WHERE l.member_id = $1 ORDER BY l.acquired_at DESC, g.id`, memberID)
}

// Wishlist คืนเกมที่สมาชิกอยากได้
func (s *Store) Wishlist(ctx context.Context, memberID int64) ([]Game, error) {
	return s.queryGames(ctx, gameSelect+` JOIN wishlist_items w ON w.game_id = g.id
		WHERE w.member_id = $1 ORDER BY w.added_at DESC, g.id`, memberID)
}

// AddWishlist เพิ่มเกมในรายการอยากได้ เกมเดิมซ้ำไม่ถือว่าผิด
func (s *Store) AddWishlist(ctx context.Context, memberID, gameID int64) error {
	tag, err := s.Pool.Exec(ctx,
		`INSERT INTO wishlist_items (member_id, game_id)
		 SELECT $1, id FROM games WHERE id = $2 ON CONFLICT DO NOTHING`, memberID, gameID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := s.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM games WHERE id = $1)`, gameID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
	}
	return nil
}

// RemoveWishlist เอาเกมออกจากรายการอยากได้
func (s *Store) RemoveWishlist(ctx context.Context, memberID, gameID int64) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM wishlist_items WHERE member_id = $1 AND game_id = $2`, memberID, gameID)
	return err
}
