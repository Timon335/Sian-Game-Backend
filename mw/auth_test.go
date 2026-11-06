package mw

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"siangame/auth"
)

var issuer = &auth.Issuer{Secret: []byte("test-secret-test-secret-test-secret"), TTL: time.Hour}

func guarded(role string) http.HandlerFunc {
	return RequireLogin(issuer, RequireRole(role, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
}

func TestMemberOnAdminIs403JSON(t *testing.T) {
	tok, _ := issuer.Issue(1, "member")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	guarded("admin").ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rec.Code)
	}
	if code := errorCode(t, rec); code != "FORBIDDEN" {
		t.Errorf("code = %q", code)
	}
}

func TestAdminPasses(t *testing.T) {
	tok, _ := issuer.Issue(1, "admin")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	guarded("admin").ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestForgedTokenIs401JSON(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer forged.token.here")
	rec := httptest.NewRecorder()
	guarded("admin").ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
	if code := errorCode(t, rec); code != "BAD_TOKEN" {
		t.Errorf("code = %q", code)
	}
}
