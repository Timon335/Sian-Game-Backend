package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrBadGenre หมายถึงมีเลขแนวเกมที่ไม่มีอยู่จริง
var ErrBadGenre = errors.New("bad genre")

// TopGame คือเกมขายดีในหน้าภาพรวม
type TopGame struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	Cover      string `json:"cover"`
	SalesCount int64  `json:"salesCount"`
}

// Stats รวมตัวเลขของทั้งเว็บ ให้หน้าภาพรวมยิงครั้งเดียวจบ
type Stats struct {
	Games         int64     `json:"games"`
	Members       int64     `json:"members"`
	Admins        int64     `json:"admins"`
	Orders        int64     `json:"orders"`
	Revenue       float64   `json:"revenue"`
	TopSelling    []TopGame `json:"topSelling"`
	NewestMembers []Member  `json:"newestMembers"`
}

// Stats คำนวณตัวเลขจากฐานข้อมูลจริง
func (s *Store) Stats(ctx context.Context) (Stats, error) {
	st := Stats{TopSelling: []TopGame{}, NewestMembers: []Member{}}
	err := s.Pool.QueryRow(ctx,
		`SELECT (SELECT count(*) FROM games WHERE is_active),
		        (SELECT count(*) FROM members),
		        (SELECT count(*) FROM members WHERE role = 'admin'),
		        (SELECT count(*) FROM orders),
		        (SELECT COALESCE(sum(total), 0)::float8 FROM orders)`).
		Scan(&st.Games, &st.Members, &st.Admins, &st.Orders, &st.Revenue)
	if err != nil {
		return st, err
	}

	rows, err := s.Pool.Query(ctx,
		`SELECT g.id, g.title, g.cover_url, count(*) AS sales
		 FROM order_items i JOIN games g ON g.id = i.game_id
		 GROUP BY g.id ORDER BY sales DESC, g.id LIMIT 5`)
	if err != nil {
		return st, err
	}
	for rows.Next() {
		var t TopGame
		if err := rows.Scan(&t.ID, &t.Title, &t.Cover, &t.SalesCount); err != nil {
			rows.Close()
			return st, err
		}
		st.TopSelling = append(st.TopSelling, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return st, err
	}

	st.NewestMembers, err = s.queryMembers(ctx, memberSelect+` ORDER BY created_at DESC, id DESC LIMIT 5`)
	return st, err
}

func (s *Store) queryMembers(ctx context.Context, sql string) ([]Member, error) {
	rows, err := s.Pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Member{}
	for rows.Next() {
		m, err := scanMember(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListMembers คืนสมาชิกทั้งหมด
func (s *Store) ListMembers(ctx context.Context) ([]Member, error) {
	return s.queryMembers(ctx, memberSelect+` ORDER BY created_at DESC, id DESC`)
}

// SetRole เปลี่ยนบทบาทสมาชิก
func (s *Store) SetRole(ctx context.Context, id int64, role string) (Member, error) {
	return scanMember(s.Pool.QueryRow(ctx,
		`UPDATE members SET role = $2, updated_at = now() WHERE id = $1
		 RETURNING id, email, display_name, role, created_at, password_hash`, id, role))
}

// GameInput คือข้อมูลจากฟอร์มหลังบ้าน IsActive เป็น nil คือไม่เปลี่ยนสถานะการขาย
type GameInput struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Developer   string   `json:"developer"`
	ReleaseDate *string  `json:"releaseDate"`
	Price       float64  `json:"price"`
	Cover       string   `json:"cover"`
	GenreIDs    []int64  `json:"genreIds"`
	Screenshots []string `json:"screenshots"`
	IsActive    *bool    `json:"isActive"`
}

// AdminListGames คืนเกมทั้งหมด รวมเกมที่เลิกขาย
func (s *Store) AdminListGames(ctx context.Context) ([]Game, error) {
	return s.queryGames(ctx, gameSelect+` ORDER BY g.id`)
}

// saveRelations เขียนแนวเกมและภาพตัวอย่างใหม่ทั้งชุดใน transaction เดียวกับตัวเกม
func saveRelations(ctx context.Context, tx pgx.Tx, id int64, in GameInput) error {
	if _, err := tx.Exec(ctx, `DELETE FROM game_genres WHERE game_id = $1`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM game_screenshots WHERE game_id = $1`, id); err != nil {
		return err
	}
	for _, g := range in.GenreIDs {
		_, err := tx.Exec(ctx, `INSERT INTO game_genres VALUES ($1,$2) ON CONFLICT DO NOTHING`, id, g)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return ErrBadGenre
		}
		if err != nil {
			return err
		}
	}
	for i, url := range in.Screenshots {
		if _, err := tx.Exec(ctx, `INSERT INTO game_screenshots VALUES ($1,$2,$3)`, id, url, i); err != nil {
			return err
		}
	}
	return nil
}

// CreateGame เพิ่มเกมพร้อมแนวและภาพตัวอย่างในก้อนเดียว
func (s *Store) CreateGame(ctx context.Context, in GameInput) (Game, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Game{}, err
	}
	defer tx.Rollback(ctx)
	var id int64
	err = tx.QueryRow(ctx,
		`INSERT INTO games (title, description_th, developer, release_date, price, cover_url)
		 VALUES ($1,$2,$3,$4::date,$5,$6) RETURNING id`,
		in.Title, in.Description, in.Developer, in.ReleaseDate, in.Price, in.Cover).Scan(&id)
	if err != nil {
		return Game{}, err
	}
	if err := saveRelations(ctx, tx, id, in); err != nil {
		return Game{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Game{}, err
	}
	return s.GetGame(ctx, id)
}

// UpdateGame แก้เกมพร้อมแนวและภาพตัวอย่างในก้อนเดียว
func (s *Store) UpdateGame(ctx context.Context, id int64, in GameInput) (Game, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Game{}, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx,
		`UPDATE games SET title = $2, description_th = $3, developer = $4, release_date = $5::date,
		        price = $6, cover_url = $7, is_active = COALESCE($8, is_active), updated_at = now()
		 WHERE id = $1`,
		id, in.Title, in.Description, in.Developer, in.ReleaseDate, in.Price, in.Cover, in.IsActive)
	if err != nil {
		return Game{}, err
	}
	if tag.RowsAffected() == 0 {
		return Game{}, ErrNotFound
	}
	if err := saveRelations(ctx, tx, id, in); err != nil {
		return Game{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Game{}, err
	}
	return s.GetGame(ctx, id)
}

// StopSelling ซ่อนเกมจากร้าน ไม่ลบแถวจริงเพราะยังมีคนถือเกมนี้ในคลัง
func (s *Store) StopSelling(ctx context.Context, id int64) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE games SET is_active = FALSE, updated_at = now() WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AdminOrders คืนคำสั่งซื้อทั้งหมดพร้อมเจ้าของ
func (s *Store) AdminOrders(ctx context.Context) ([]Order, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT o.id, o.total::float8, o.created_at, m.id, m.email, m.display_name
		 FROM orders o JOIN members m ON m.id = o.member_id
		 ORDER BY o.created_at DESC, o.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Order{}
	for rows.Next() {
		o := Order{Items: []OrderItem{}, Member: &OrderMember{}}
		if err := rows.Scan(&o.ID, &o.Total, &o.CreatedAt, &o.Member.ID, &o.Member.Email, &o.Member.DisplayName); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, s.attachItems(ctx, out)
}

// AdminReviews คืนรีวิวทั้งหมดพร้อมชื่อเกมและอีเมลผู้เขียน
func (s *Store) AdminReviews(ctx context.Context) ([]Review, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT r.id, r.member_id, m.display_name, r.game_id, r.rating, r.text, r.created_at, r.updated_at, g.title, m.email
		 FROM reviews r JOIN members m ON m.id = r.member_id JOIN games g ON g.id = r.game_id
		 ORDER BY r.created_at DESC, r.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Review{}
	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.ID, &r.MemberID, &r.DisplayName, &r.GameID, &r.Rating, &r.Text, &r.CreatedAt, &r.UpdatedAt, &r.GameTitle, &r.Email); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AdminDeleteReview ลบรีวิวใดก็ได้
func (s *Store) AdminDeleteReview(ctx context.Context, id int64) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM reviews WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
