package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrEmailTaken หมายถึงอีเมลนี้มีบัญชีอยู่แล้ว
var ErrEmailTaken = errors.New("email taken")

// Member คือข้อมูลสมาชิกที่ส่งออกได้ PasswordHash ไม่ถูกแปลงเป็น JSON
type Member struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	DisplayName  string    `json:"displayName"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"createdAt"`
	PasswordHash string    `json:"-"`
}

const memberSelect = `SELECT id, email, display_name, role, created_at, password_hash FROM members`

func scanMember(row pgx.Row) (Member, error) {
	var m Member
	err := row.Scan(&m.ID, &m.Email, &m.DisplayName, &m.Role, &m.CreatedAt, &m.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, ErrNotFound
	}
	return m, err
}

// CreateMember สร้างสมาชิกใหม่ ผู้เรียกต้องส่งอีเมลตัวพิมพ์เล็กมา
func (s *Store) CreateMember(ctx context.Context, email, hash, name string) (Member, error) {
	m, err := scanMember(s.Pool.QueryRow(ctx,
		`INSERT INTO members (email, password_hash, display_name) VALUES ($1,$2,$3)
		 RETURNING id, email, display_name, role, created_at, password_hash`, email, hash, name))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return m, ErrEmailTaken
	}
	return m, err
}

// MemberByEmail ค้นสมาชิกจากอีเมลแบบไม่สนตัวพิมพ์
func (s *Store) MemberByEmail(ctx context.Context, email string) (Member, error) {
	return scanMember(s.Pool.QueryRow(ctx, memberSelect+` WHERE lower(email) = lower($1)`, email))
}

// MemberByID ค้นสมาชิกจากเลข
func (s *Store) MemberByID(ctx context.Context, id int64) (Member, error) {
	return scanMember(s.Pool.QueryRow(ctx, memberSelect+` WHERE id = $1`, id))
}

// UpdateDisplayName แก้ชื่อที่แสดง
func (s *Store) UpdateDisplayName(ctx context.Context, id int64, name string) (Member, error) {
	return scanMember(s.Pool.QueryRow(ctx,
		`UPDATE members SET display_name = $2, updated_at = now() WHERE id = $1
		 RETURNING id, email, display_name, role, created_at, password_hash`, id, name))
}

// UpdatePassword เปลี่ยนลายนิ้วมือของรหัสผ่าน
func (s *Store) UpdatePassword(ctx context.Context, id int64, hash string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE members SET password_hash = $2, updated_at = now() WHERE id = $1`, id, hash)
	return err
}
