// Package handler แปลงคำขอ HTTP เป็นการเรียก store แล้วตอบเป็น JSON
package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"siangame/auth"
	"siangame/store"
)

// Handler รวมของที่ทุกเส้นทางต้องใช้
type Handler struct {
	Store   *store.Store
	Tickets *auth.Issuer
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

// internal จดรายละเอียดไว้ใน log ฝั่งเรา แต่บอกผู้ใช้แค่ว่าระบบขัดข้อง
func internal(w http.ResponseWriter, err error) {
	log.Printf("internal error: %v", err)
	writeError(w, http.StatusInternalServerError, "INTERNAL", "ระบบขัดข้อง ลองใหม่อีกครั้ง")
}

func pathID(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id, err == nil && id > 0
}

func badID(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, "BAD_ID", "เลขที่ส่งมาไม่ถูกต้อง")
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", "อ่านข้อมูลที่ส่งมาไม่ได้")
		return false
	}
	return true
}
