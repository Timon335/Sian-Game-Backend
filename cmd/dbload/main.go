// dbload รันไฟล์ SQL ลงฐานข้อมูลตาม DATABASE_URL ใช้แทน psql
package main

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	url := os.Getenv("DATABASE_URL")
	if url == "" || len(os.Args) < 2 {
		log.Fatal("วิธีใช้: ตั้ง DATABASE_URL แล้วรัน go run ./cmd/dbload db/schema.sql db/seed.sql")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		log.Fatal("ต่อฐานข้อมูลไม่ติด")
	}
	defer conn.Close(ctx)

	for _, path := range os.Args[1:] {
		sql, err := os.ReadFile(path)
		if err != nil {
			log.Fatal(err)
		}
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			log.Fatalf("%s: %v", path, err)
		}
		log.Println("โหลดแล้ว", path)
	}
}
