package handler

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"siangame/auth"
	"siangame/mw"
	"siangame/store"
)

// validPassword ตรวจความยาว bcrypt รับได้ไม่เกิน 72 ไบต์
func validPassword(p string) bool {
	return utf8.RuneCountInString(p) >= 8 && len(p) <= 72
}

func writeBadPassword(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, "BAD_PASSWORD", "รหัสผ่านต้องยาว 8 ตัวอักษรขึ้นไป และไม่เกิน 72 ไบต์")
}

func validName(n string) bool {
	return n != "" && utf8.RuneCountInString(n) <= 50
}

func writeBadName(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, "BAD_NAME", "ต้องมีชื่อที่แสดง และยาวไม่เกิน 50 ตัวอักษร")
}

// Register สมัครสมาชิก บทบาทเริ่มต้นเป็น member เสมอ
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	name := strings.TrimSpace(req.DisplayName)
	switch {
	case !strings.Contains(email, "@") || len(email) > 255:
		writeError(w, http.StatusBadRequest, "BAD_EMAIL", "รูปแบบอีเมลไม่ถูกต้อง")
		return
	case !validPassword(req.Password):
		writeBadPassword(w)
		return
	case !validName(name):
		writeBadName(w)
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		internal(w, err)
		return
	}
	m, err := h.Store.CreateMember(r.Context(), email, hash, name)
	if errors.Is(err, store.ErrEmailTaken) {
		writeError(w, http.StatusConflict, "EMAIL_TAKEN", "อีเมลนี้มีบัญชีอยู่แล้ว")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

// Login ตรวจรหัสผ่านแล้วออกตั๋ว ข้อความผิดพลาดเหมือนกันทุกกรณีเพื่อไม่บอกว่ามีอีเมลนี้หรือไม่
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	m, err := h.Store.MemberByEmail(r.Context(), strings.TrimSpace(req.Email))
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		internal(w, err)
		return
	}
	if err != nil || !auth.CheckPassword(m.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "BAD_CREDENTIALS", "อีเมลหรือรหัสผ่านไม่ถูกต้อง")
		return
	}
	token, err := h.Tickets.Issue(m.ID, m.Role)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"member": m, "token": token})
}

// currentMember โหลดสมาชิกตามเลขในตั๋ว ถ้าบัญชีหายไปแล้วตอบ 401
func (h *Handler) currentMember(w http.ResponseWriter, r *http.Request) (store.Member, bool) {
	id, _ := mw.MemberID(r.Context())
	m, err := h.Store.MemberByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "MEMBER_GONE", "ไม่พบบัญชีนี้แล้ว กรุณาเข้าสู่ระบบใหม่")
		return m, false
	}
	if err != nil {
		internal(w, err)
		return m, false
	}
	return m, true
}

// Me ตอบข้อมูลของคนที่ถือตั๋ว เลขสมาชิกมาจากตั๋วเท่านั้น
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	if m, ok := h.currentMember(w, r); ok {
		writeJSON(w, http.StatusOK, m)
	}
}

// UpdateMe แก้ชื่อที่แสดงของตัวเอง
func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DisplayName string `json:"displayName"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.DisplayName)
	if !validName(name) {
		writeBadName(w)
		return
	}
	id, _ := mw.MemberID(r.Context())
	m, err := h.Store.UpdateDisplayName(r.Context(), id, name)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// ChangePassword เปลี่ยนรหัสผ่าน ต้องใส่รหัสเดิมให้ถูกก่อน
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	m, ok := h.currentMember(w, r)
	if !ok {
		return
	}
	if !auth.CheckPassword(m.PasswordHash, req.CurrentPassword) {
		writeError(w, http.StatusBadRequest, "BAD_CURRENT_PASSWORD", "รหัสผ่านเดิมไม่ถูกต้อง")
		return
	}
	if !validPassword(req.NewPassword) {
		writeBadPassword(w)
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		internal(w, err)
		return
	}
	if err := h.Store.UpdatePassword(r.Context(), m.ID, hash); err != nil {
		internal(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
