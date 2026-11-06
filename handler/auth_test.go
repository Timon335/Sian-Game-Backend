package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"siangame/auth"
	"siangame/store"
)

func register(a *app, email, password, name string) int {
	return a.call("POST", "/api/auth/register", "", map[string]string{"email": email, "password": password, "displayName": name}).Code
}

func TestRegisterThenLogin(t *testing.T) {
	a := newApp(t)
	if c := register(a, "a@x.test", "password1", "เอ"); c != http.StatusCreated {
		t.Fatalf("สมัคร status = %d", c)
	}
	rec := a.call("POST", "/api/auth/login", "", map[string]string{"email": "a@x.test", "password": "password1"})
	wantStatus(t, rec, http.StatusOK)
	got := decode[struct {
		Member store.Member
		Token  string
	}](t, rec)
	if got.Token == "" || got.Member.Email != "a@x.test" || got.Member.Role != "member" {
		t.Errorf("ผลลัพธ์ = %+v", got)
	}
	if strings.Contains(rec.Body.String(), "password") {
		t.Error("ห้ามส่ง password hash กลับ")
	}
}

func TestRegisterLowercasesEmail(t *testing.T) {
	a := newApp(t)
	register(a, "  MixED@X.test ", "password1", "เอ")
	if n := a.count(`SELECT count(*) FROM members WHERE email='mixed@x.test'`); n != 1 {
		t.Errorf("พบ %d แถว", n)
	}
	rec := a.call("POST", "/api/auth/login", "", map[string]string{"email": "MIXED@x.test", "password": "password1"})
	wantStatus(t, rec, http.StatusOK)
}

func TestDuplicateEmailDifferentCase409(t *testing.T) {
	a := newApp(t)
	register(a, "a@x.test", "password1", "เอ")
	rec := a.call("POST", "/api/auth/register", "", map[string]string{"email": "A@X.TEST", "password": "password1", "displayName": "บี"})
	if code := wantStatus(t, rec, http.StatusConflict); code != "EMAIL_TAKEN" {
		t.Errorf("code = %q", code)
	}
}

func TestRegisterValidation(t *testing.T) {
	a := newApp(t)
	cases := []struct{ name, email, password, display, code string }{
		{"อีเมลผิด", "bad", "password1", "เอ", "BAD_EMAIL"},
		{"รหัสสั้น", "a@x.test", "short", "เอ", "BAD_PASSWORD"},
		{"ชื่อว่าง", "a@x.test", "password1", "   ", "BAD_NAME"},
	}
	for _, c := range cases {
		rec := a.call("POST", "/api/auth/register", "", map[string]string{"email": c.email, "password": c.password, "displayName": c.display})
		if code := wantStatus(t, rec, http.StatusBadRequest); code != c.code {
			t.Errorf("%s: code = %q", c.name, code)
		}
	}
}

func TestPasswordOver72Bytes400(t *testing.T) {
	a := newApp(t)
	// ภาษาไทย 1 ตัวใช้ 3 ไบต์: 30 ตัวอักษร = 90 ไบต์
	rec := a.call("POST", "/api/auth/register", "", map[string]string{"email": "a@x.test", "password": strings.Repeat("ก", 30), "displayName": "เอ"})
	if code := wantStatus(t, rec, http.StatusBadRequest); code != "BAD_PASSWORD" {
		t.Errorf("code = %q", code)
	}
}

func TestLoginWrongPassword401(t *testing.T) {
	a := newApp(t)
	register(a, "a@x.test", "password1", "เอ")
	for _, body := range []map[string]string{
		{"email": "a@x.test", "password": "wrong-pass"},
		{"email": "none@x.test", "password": "password1"},
	} {
		if code := wantStatus(t, a.call("POST", "/api/auth/login", "", body), http.StatusUnauthorized); code != "BAD_CREDENTIALS" {
			t.Errorf("code = %q", code)
		}
	}
}

func TestNoTokenIs401JSON(t *testing.T) {
	a := newApp(t)
	if code := wantStatus(t, a.call("GET", "/api/me", "", nil), http.StatusUnauthorized); code != "NO_TOKEN" {
		t.Errorf("code = %q", code)
	}
}

func TestExpiredTokenIs401JSON(t *testing.T) {
	a := newApp(t)
	id, _ := a.addMember("a@x.test", "member")
	old := &auth.Issuer{Secret: a.tickets.Secret, TTL: -time.Minute}
	tok, _ := old.Issue(id, "member")
	if code := wantStatus(t, a.call("GET", "/api/me", tok, nil), http.StatusUnauthorized); code != "BAD_TOKEN" {
		t.Errorf("code = %q", code)
	}
}

func TestMeUsesTokenID(t *testing.T) {
	a := newApp(t)
	a.addMember("other@x.test", "member")
	_, tok := a.addMember("me@x.test", "member")
	rec := a.call("GET", "/api/me?id=1", tok, nil)
	wantStatus(t, rec, http.StatusOK)
	if got := decode[store.Member](t, rec); got.Email != "me@x.test" {
		t.Errorf("ได้ข้อมูลของ %q", got.Email)
	}
}

func TestUpdateName(t *testing.T) {
	a := newApp(t)
	_, tok := a.addMember("a@x.test", "member")
	rec := a.call("PUT", "/api/me", tok, map[string]string{"displayName": "  ชื่อใหม่ "})
	wantStatus(t, rec, http.StatusOK)
	if got := decode[store.Member](t, rec); got.DisplayName != "ชื่อใหม่" {
		t.Errorf("ชื่อ = %q", got.DisplayName)
	}
}

func TestUpdateNameBlank400(t *testing.T) {
	a := newApp(t)
	_, tok := a.addMember("a@x.test", "member")
	if code := wantStatus(t, a.call("PUT", "/api/me", tok, map[string]string{"displayName": "  "}), http.StatusBadRequest); code != "BAD_NAME" {
		t.Errorf("code = %q", code)
	}
}

func loginToken(t *testing.T, a *app, email, password string) string {
	t.Helper()
	rec := a.call("POST", "/api/auth/login", "", map[string]string{"email": email, "password": password})
	wantStatus(t, rec, http.StatusOK)
	return decode[struct{ Token string }](t, rec).Token
}

func TestChangePasswordRequiresCurrent(t *testing.T) {
	a := newApp(t)
	register(a, "a@x.test", "password1", "เอ")
	tok := loginToken(t, a, "a@x.test", "password1")
	rec := a.call("PUT", "/api/me/password", tok, map[string]string{"currentPassword": "wrong-pass", "newPassword": "password2"})
	if code := wantStatus(t, rec, http.StatusBadRequest); code != "BAD_CURRENT_PASSWORD" {
		t.Errorf("code = %q", code)
	}
	if c := a.call("POST", "/api/auth/login", "", map[string]string{"email": "a@x.test", "password": "password2"}).Code; c != http.StatusUnauthorized {
		t.Errorf("รหัสใหม่ต้องยังใช้ไม่ได้ status = %d", c)
	}
}

func TestChangePasswordThenLoginWithNew(t *testing.T) {
	a := newApp(t)
	register(a, "a@x.test", "password1", "เอ")
	tok := loginToken(t, a, "a@x.test", "password1")
	rec := a.call("PUT", "/api/me/password", tok, map[string]string{"currentPassword": "password1", "newPassword": "password2"})
	wantStatus(t, rec, http.StatusNoContent)
	loginToken(t, a, "a@x.test", "password2")
	if c := a.call("POST", "/api/auth/login", "", map[string]string{"email": "a@x.test", "password": "password1"}).Code; c != http.StatusUnauthorized {
		t.Errorf("รหัสเดิมต้องใช้ไม่ได้ status = %d", c)
	}
}

func TestChangePasswordTooShort400(t *testing.T) {
	a := newApp(t)
	register(a, "a@x.test", "password1", "เอ")
	tok := loginToken(t, a, "a@x.test", "password1")
	rec := a.call("PUT", "/api/me/password", tok, map[string]string{"currentPassword": "password1", "newPassword": "x"})
	if code := wantStatus(t, rec, http.StatusBadRequest); code != "BAD_PASSWORD" {
		t.Errorf("code = %q", code)
	}
}
