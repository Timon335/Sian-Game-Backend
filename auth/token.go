package auth

import (
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Issuer คือช่องขายตั๋ว ออกและตรวจ JWT ด้วยกุญแจลับเดียว
type Issuer struct {
	Secret []byte
	TTL    time.Duration
}

type claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// Issue ออกตั๋วที่พกเลขสมาชิกและบทบาท
func (i *Issuer) Issue(memberID int64, role string) (string, error) {
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(memberID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(i.TTL)),
		},
	}).SignedString(i.Secret)
}

// Parse ตรวจตราประทับและวันหมดอายุ แล้วคืนเลขสมาชิกกับบทบาท
func (i *Issuer) Parse(token string) (int64, string, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return i.Secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil {
		return 0, "", err
	}
	id, err := strconv.ParseInt(c.Subject, 10, 64)
	if err != nil {
		return 0, "", errors.New("เลขสมาชิกในตั๋วไม่ถูกต้อง")
	}
	return id, c.Role, nil
}
