package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/joho/godotenv/autoload" // ในเครื่องโหลดจาก .env ส่วนบน Render ไม่มีไฟล์ก็ใช้ค่าจาก Environment

	"siangame/auth"
	"siangame/handler"
	"siangame/mw"
	"siangame/store"
)

func main() {
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		log.Fatal("JWT_SECRET ต้องยาวอย่างน้อย 32 ตัวอักษร")
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("ต้องกำหนด DATABASE_URL")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		log.Fatal("อ่าน DATABASE_URL ไม่ได้")
	}
	defer pool.Close()
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatal("ต่อฐานข้อมูลไม่ติด")
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
