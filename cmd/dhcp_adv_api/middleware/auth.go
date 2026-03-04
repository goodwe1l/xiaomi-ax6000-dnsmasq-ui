package middleware

import "net/http"

type AuthCheckFunc func(r *http.Request) bool
type UnauthorizedHandler func(w http.ResponseWriter, r *http.Request)

func RequireAuth(check AuthCheckFunc, onUnauthorized UnauthorizedHandler, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !check(r) {
			onUnauthorized(w, r)
			return
		}
		next(w, r)
	}
}
