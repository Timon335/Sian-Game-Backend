package mw

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var body struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("ไม่ใช่ JSON: %q", rec.Body.String())
	}
	return body.Error.Code
}

func TestCORSOnlyAllowedOrigin(t *testing.T) {
	h := CORS([]string{"https://sian.test"}, okHandler())

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://sian.test")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://sian.test" {
		t.Errorf("origin ที่อนุญาต ได้ %q", got)
	}

	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://evil.test")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("origin แปลกปลอมต้องไม่ได้ header แต่ได้ %q", got)
	}
}

func TestPreflightReturns204(t *testing.T) {
	h := CORS([]string{"https://sian.test"}, okHandler())
	req := httptest.NewRequest("OPTIONS", "/api/me", nil)
	req.Header.Set("Origin", "https://sian.test")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Error("ต้องบอก header ที่อนุญาต")
	}
}

func TestRecoverReturnsJSON500(t *testing.T) {
	h := Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("พัง") }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d", rec.Code)
	}
	if code := errorCode(t, rec); code != "INTERNAL" {
		t.Errorf("code = %q", code)
	}
}

func TestTimeoutReturnsJSON503(t *testing.T) {
	h := Timeout(10*time.Millisecond, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d", rec.Code)
	}
	if code := errorCode(t, rec); code != "TIMEOUT" {
		t.Errorf("code = %q", code)
	}
}
