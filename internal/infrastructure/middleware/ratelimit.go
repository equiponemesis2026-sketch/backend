package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type rateLimiter struct {
	mu        sync.Mutex
	requests  map[string][]time.Time
	limit     int
	window    time.Duration
	cleanupAt time.Time
}

// RateLimit limita las peticiones por IP dentro de una ventana de tiempo.
func RateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	rl := &rateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			now := time.Now()

			rl.mu.Lock()
			rl.purge(now)

			var recent []time.Time
			for _, t := range rl.requests[ip] {
				if now.Sub(t) < window {
					recent = append(recent, t)
				}
			}

			if len(recent) >= limit {
				rl.mu.Unlock()
				http.Error(w, `{"status":"error","message":"too many requests"}`, http.StatusTooManyRequests)
				return
			}

			rl.requests[ip] = append(recent, now)
			rl.mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

// purge elimina periódicamente las entradas de IPs inactivas para evitar fugas de memoria.
func (rl *rateLimiter) purge(now time.Time) {
	if now.Sub(rl.cleanupAt) < time.Minute {
		return
	}
	rl.cleanupAt = now
	for ip, hits := range rl.requests {
		var active []time.Time
		for _, t := range hits {
			if now.Sub(t) < rl.window {
				active = append(active, t)
			}
		}
		if len(active) == 0 {
			delete(rl.requests, ip)
		} else {
			rl.requests[ip] = active
		}
	}
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return strings.TrimSpace(xrip)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
