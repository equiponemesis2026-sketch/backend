package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserIDKey contextKey = "userID"

// ParseUserID valida un JWT HS256 y devuelve el `sub` (user_id).
// Se usa en contextos sin middleware HTTP, como el handshake de WebSocket.
func ParseUserID(tokenString string, secret []byte) (string, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !token.Valid {
		return "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid token claims")
	}
	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return "", errors.New("invalid token subject")
	}
	return sub, nil
}

func JWTAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"status":"error","message":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, `{"status":"error","message":"invalid authorization format"}`, http.StatusUnauthorized)
				return
			}

			token, err := jwt.Parse(parts[1], func(t *jwt.Token) (interface{}, error) {
				return []byte(secret), nil
			}, jwt.WithValidMethods([]string{"HS256"}))
			if err != nil || !token.Valid {
				http.Error(w, `{"status":"error","message":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, `{"status":"error","message":"invalid token claims"}`, http.StatusUnauthorized)
				return
			}

			sub, ok := claims["sub"].(string)
			if !ok || sub == "" {
				http.Error(w, `{"status":"error","message":"invalid token subject"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, sub)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
