package mw

import (
	"encoding/json"
	"log"
	"net/http"
	"runtime/debug"
)

// writeError ตอบ error เป็น JSON รูปแบบเดียวกับ handler
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

// Recover กัน panic ไม่ให้ทำ server ล้ม และบอกผู้ใช้แค่ว่าระบบขัดข้อง
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				log.Printf("panic: %v\n%s", v, debug.Stack())
				writeError(w, http.StatusInternalServerError, "INTERNAL", "ระบบขัดข้อง ลองใหม่อีกครั้ง")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
