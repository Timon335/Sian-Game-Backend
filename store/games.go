package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Game คือเกมหนึ่งเกมพร้อมแนว ภาพ คะแนนเฉลี่ย และยอดขาย
type Game struct {
	ID            int64     `json:"id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Developer     string    `json:"developer"`
	ReleaseDate   *string   `json:"releaseDate"`
	Price         float64   `json:"price"`
	Cover         string    `json:"cover"`
	Genres        []string  `json:"genres"`
	Screenshots   []string  `json:"screenshots"`
	RatingAverage float64   `json:"ratingAverage"`
	RatingCount   int64     `json:"ratingCount"`
	SalesCount    int64     `json:"salesCount"`
	IsActive      bool      `json:"isActive"`
	CreatedAt     time.Time `json:"createdAt"`
}

// Genre คือแนวเกม
type Genre struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

const gameSelect = `
SELECT g.id, g.title, g.description_th, g.developer, to_char(g.release_date, 'YYYY-MM-DD'),
       g.price::float8, g.cover_url,
       COALESCE((SELECT array_agg(x.name ORDER BY x.name) FROM game_genres gg JOIN genres x ON x.id = gg.genre_id WHERE gg.game_id = g.id), '{}'),
       COALESCE((SELECT array_agg(s.url ORDER BY s.position) FROM game_screenshots s WHERE s.game_id = g.id), '{}'),
       COALESCE((SELECT round(avg(r.rating), 1)::float8 FROM reviews r WHERE r.game_id = g.id), 0),
       (SELECT count(*) FROM reviews r WHERE r.game_id = g.id),
       (SELECT count(*) FROM order_items i WHERE i.game_id = g.id),
       g.is_active, g.created_at
FROM games g`

func scanGame(row pgx.Row) (Game, error) {
	var g Game
	err := row.Scan(&g.ID, &g.Title, &g.Description, &g.Developer, &g.ReleaseDate, &g.Price, &g.Cover,
		&g.Genres, &g.Screenshots, &g.RatingAverage, &g.RatingCount, &g.SalesCount, &g.IsActive, &g.CreatedAt)
	return g, err
}

func (s *Store) queryGames(ctx context.Context, sql string, args ...any) ([]Game, error) {
	rows, err := s.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Game{}
	for rows.Next() {
		g, err := scanGame(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// ListGames คืนเฉพาะเกมที่ขายอยู่ หน้าเว็บค้นหาและกรองต่อในเบราว์เซอร์
func (s *Store) ListGames(ctx context.Context) ([]Game, error) {
	return s.queryGames(ctx, gameSelect+` WHERE g.is_active ORDER BY g.id`)
}

// GetGame คืนเกมตามเลข รวมเกมที่เลิกขาย เพราะเจ้าของยังต้องเปิดดูจากคลังได้
func (s *Store) GetGame(ctx context.Context, id int64) (Game, error) {
	g, err := scanGame(s.Pool.QueryRow(ctx, gameSelect+` WHERE g.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return g, ErrNotFound
	}
	return g, err
}

// ListGenres คืนแนวเกมทั้งหมด
func (s *Store) ListGenres(ctx context.Context) ([]Genre, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, name FROM genres ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Genre{}
	for rows.Next() {
		var g Genre
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
