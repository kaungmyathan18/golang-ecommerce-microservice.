package ratelimit

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter struct {
	rdb *redis.Client
	rpm int
}

func New(rdb *redis.Client, rpm int) *Limiter {
	return &Limiter{rdb: rdb, rpm: rpm}
}

func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if l.rpm <= 0 {
			next.ServeHTTP(w, r)
			return
		}
		ip := r.Header.Get("X-Real-IP")
		if ip == "" {
			ip = r.Header.Get("X-Forwarded-For")
		}
		if ip == "" {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err == nil {
				ip = host
			} else {
				ip = r.RemoteAddr
			}
		}
		if strings.Contains(ip, ",") {
			ip = strings.TrimSpace(strings.Split(ip, ",")[0])
		}
		key := fmt.Sprintf("ratelimit:%s:%s", ip, time.Now().UTC().Format("200601021504"))
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		count, err := l.rdb.Incr(ctx, key).Result()
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		if count == 1 {
			_ = l.rdb.Expire(ctx, key, time.Minute).Err()
		}
		if int(count) > l.rpm {
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
