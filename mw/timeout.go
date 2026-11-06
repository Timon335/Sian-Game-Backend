package mw

import (
	"bytes"
	"context"
	"net/http"
	"time"
)

// bufferedWriter เก็บคำตอบไว้ก่อน เพื่อไม่ให้ handler กับ Timeout เขียนชนกัน
type bufferedWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (b *bufferedWriter) Header() http.Header { return b.header }

func (b *bufferedWriter) WriteHeader(code int) {
	if b.status == 0 {
		b.status = code
	}
}

func (b *bufferedWriter) Write(p []byte) (int, error) {
	if b.status == 0 {
		b.status = http.StatusOK
	}
	return b.body.Write(p)
}

// Timeout ให้แต่ละคำขอมีเวลาจำกัด ถ้าเกินตอบ 503 และ context ของคำขอถูกยกเลิก
func Timeout(d time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), d)
		defer cancel()

		buf := &bufferedWriter{header: http.Header{}}
		done := make(chan any, 1)
		go func() {
			defer func() { done <- recover() }()
			next.ServeHTTP(buf, r.WithContext(ctx))
		}()

		select {
		case v := <-done:
			if v != nil {
				panic(v) // ส่งต่อให้ Recover
			}
			for k, vals := range buf.header {
				w.Header()[k] = vals
			}
			if buf.status == 0 {
				buf.status = http.StatusOK
			}
			w.WriteHeader(buf.status)
			w.Write(buf.body.Bytes())
		case <-ctx.Done():
			writeError(w, http.StatusServiceUnavailable, "TIMEOUT", "ระบบใช้เวลานานเกินไป ลองใหม่อีกครั้ง")
		}
	})
}
