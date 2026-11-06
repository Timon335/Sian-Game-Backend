package auth

import (
	"testing"
	"time"
)

func newIssuer(ttl time.Duration) *Issuer {
	return &Issuer{Secret: []byte("test-secret-test-secret-test-secret"), TTL: ttl}
}

func TestTokenRoundTrip(t *testing.T) {
	iss := newIssuer(time.Hour)
	tok, err := iss.Issue(7, "admin")
	if err != nil {
		t.Fatal(err)
	}
	id, role, err := iss.Parse(tok)
	if err != nil || id != 7 || role != "admin" {
		t.Errorf("id=%d role=%q err=%v", id, role, err)
	}
}

func TestForgedTokenRejected(t *testing.T) {
	tok, _ := newIssuer(time.Hour).Issue(1, "member")
	other := &Issuer{Secret: []byte("another-secret-another-secret-12345"), TTL: time.Hour}
	if _, _, err := other.Parse(tok); err == nil {
		t.Error("ตั๋วจากกุญแจอื่นต้องไม่ผ่าน")
	}
	if _, _, err := newIssuer(time.Hour).Parse(tok + "x"); err == nil {
		t.Error("ตั๋วที่ถูกแก้ต้องไม่ผ่าน")
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	iss := newIssuer(-time.Minute)
	tok, _ := iss.Issue(1, "member")
	if _, _, err := iss.Parse(tok); err == nil {
		t.Error("ตั๋วหมดอายุต้องไม่ผ่าน")
	}
}
