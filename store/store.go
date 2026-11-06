// Package store คือชั้นที่คุยกับ PostgreSQL ทั้งหมด handler ไม่เขียน SQL เอง
package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound หมายถึงไม่พบแถวที่ขอ
var ErrNotFound = errors.New("not found")

// Store ถือ connection pool ตัวเดียวของทั้งโปรแกรม
type Store struct {
	Pool *pgxpool.Pool
}
