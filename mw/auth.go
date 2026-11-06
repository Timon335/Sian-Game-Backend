package mw

import (
	"context"
	"net/http"
	"strings"

	"siangame/auth"
)

type contextKey string

const (
	memberIDKey contextKey = "memberID"
	roleKey     contextKey = "role"
)

// MemberID อ่านเลขสมาชิกที่ RequireLogin ตรวจจากตั๋วแล้ว
func MemberID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(memberIDKey).(int64)
	return id, ok
}

// Role อ่านบทบาทที่ RequireLogin ตรวจจากตั๋วแล้ว
func Role(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(roleKey).(string)
	return role, ok
}

// RequireLogin คือพนักงานตรวจตั๋ว ไม่มีตั๋วหรือตั๋วใช้ไม่ได้ตอบ 401
func RequireLogin(issuer *auth.Issuer, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, prefix) {
			writeError(w, http.StatusUnauthorized, "NO_TOKEN", "กรุณาเข้าสู่ระบบก่อน")
			return
		}
		id, role, err := issuer.Parse(strings.TrimSpace(strings.TrimPrefix(header, prefix)))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "BAD_TOKEN", "การเข้าสู่ระบบหมดอายุ กรุณาเข้าสู่ระบบใหม่")
			return
		}
		ctx := context.WithValue(r.Context(), memberIDKey, id)
		ctx = context.WithValue(ctx, roleKey, role)
		next(w, r.WithContext(ctx))
	}
}

// RequireRole ตรวจบทบาทหลังตรวจตั๋วแล้ว บทบาทไม่ตรงตอบ 403
func RequireRole(required string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		role, ok := Role(r.Context())
		if !ok || role != required {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "คุณไม่มีสิทธิ์เข้าถึงส่วนนี้")
			return
		}
		next(w, r)
	}
}
