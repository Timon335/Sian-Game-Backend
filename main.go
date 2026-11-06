package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/joho/godotenv/autoload" // ในเครื่องโหลดจาก .env ส่วนบน Render ไม่มีไฟล์ก็ใช้ค่าจาก Environment

	"siangame/auth"
	"siangame/handler"
	"siangame/mw"
	"siangame/store"
)

// envValueOf ตัดช่องว่างและบรรทัดใหม่ที่ติดมาตอนวางค่าในหน้า Environment
func envValueOf(raw string) string {
	return strings.TrimSpace(raw)
}

func envValue(name string) string {
	return envValueOf(os.Getenv(name))
}

// poolConfig อ่าน DATABASE_URL ถ้าผิดจะบอกแค่สาเหตุ ไม่ส่งตัว URL ออกไป เพราะมีรหัสผ่านอยู่ข้างใน
func poolConfig(databaseURL string) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	var parseErr *pgconn.ParseConfigError
	if errors.As(err, &parseErr) {
		return nil, parseErr.Unwrap()
	}
	return cfg, err
}

// pingDatabase ตรวจว่าต่อฐานข้อมูลได้จริง ข้อความจาก pgx บอก host และสาเหตุ แต่ไม่มีรหัสผ่าน
func pingDatabase(ctx context.Context, pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ต่อฐานข้อมูลไม่ติด: %w", err)
	}
	return nil
}

func main() {
	secret := envValue("JWT_SECRET")
	if len(secret) < 32 {
		log.Fatal("JWT_SECRET ต้องยาวอย่างน้อย 32 ตัวอักษร")
	}
	databaseURL := envValue("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("ต้องกำหนด DATABASE_URL")
	}
	cfg, err := poolConfig(databaseURL)
	if err != nil {
		log.Fatal("อ่าน DATABASE_URL ไม่ได้: ", err)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		log.Fatal("สร้าง connection pool ไม่ได้")
	}
	defer pool.Close()
	if err := pingDatabase(context.Background(), pool); err != nil {
		log.Fatal(err)
	}

	h := &handler.Handler{
		Store:   &store.Store{Pool: pool},
		Tickets: &auth.Issuer{Secret: []byte(secret), TTL: time.Hour},
	}
	origins := strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")
	// ด่านทั้งสี่: กัน panic, จดบันทึก, ตรวจ origin, จำกัดเวลา
	root := mw.Recover(mw.Logging(mw.CORS(origins, mw.Timeout(15*time.Second, h.Routes()))))

	port := os.Getenv("PORT") // Render ส่งค่ามาให้ ในเครื่องใช้ 8080
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{Addr: ":" + port, Handler: root, ReadHeaderTimeout: 5 * time.Second}

	// ปิดร้านอย่างเรียบร้อย: รอคำขอที่ค้างอยู่ก่อนปิดฐานข้อมูล
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		log.Println("Sian Game API ฟังอยู่ที่ :" + port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Println("ปิด server ไม่เรียบร้อย:", err)
	}
}
