# Sian Game Backend

API ของเว็บขายเกม Sian Game เขียนด้วย Go `net/http`, pgx, JWT และ bcrypt ใช้ PostgreSQL บน Supabase และ deploy บน Render

สัญญา API อยู่ที่ [openapi.yaml](openapi.yaml)

## โครงสร้าง

| โฟลเดอร์ | หน้าที่ |
|---|---|
| `handler/` | รับคำขอ ตรวจข้อมูล และตอบ JSON |
| `store/` | คุยกับฐานข้อมูล รวม transaction ตอนสั่งซื้อ |
| `mw/` | Recover, Logging, CORS, Timeout และการตรวจสิทธิ์ |
| `auth/` | รหัสผ่านและตั๋ว JWT |
| `db/` | `schema.sql` และ `seed.sql` |
| `cmd/dbload/` | โหลดไฟล์ SQL ลงฐานข้อมูล ใช้แทน psql |

## รันในเครื่อง

```bash
cp .env.example .env
go run ./cmd/dbload db/schema.sql db/seed.sql
go run .
```

ล้างข้อมูลแล้วโหลดใหม่ทั้งหมด (ข้อมูลเดิมจะหาย)

```bash
go run ./cmd/dbload db/reset.sql db/schema.sql db/seed.sql
```

ค่าใน `.env` ได้แก่ `DATABASE_URL` แบบ Session pooler พอร์ต 5432, `JWT_SECRET` ยาวอย่างน้อย 32 ตัวอักษร และ `ALLOWED_ORIGINS`

## เทสต์

```bash
go test ./...
```

เทสต์ฐานข้อมูลต้องตั้ง `TEST_DATABASE_URL` ชี้ PostgreSQL ที่ล้างข้อมูลได้ ถ้าไม่ตั้งจะข้ามไป และจะไม่ยอมรันกับ Supabase บน GitHub Actions ใช้ PostgreSQL ชั่วคราวของ workflow

```bash
TEST_DATABASE_URL='postgres://postgres@localhost:55432/siangame_test?sslmode=disable' go test -p 1 ./...
```
