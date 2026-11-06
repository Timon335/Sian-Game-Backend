// Package auth ดูแลรหัสผ่านและตั๋ว JWT
package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword ทำรหัสผ่านเป็นลายนิ้วมือที่ย้อนกลับไม่ได้
func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(h), err
}

// CheckPassword ตรวจว่ารหัสผ่านตรงกับลายนิ้วมือหรือไม่
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
