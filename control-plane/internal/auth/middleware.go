package auth

import (
	"net/http"
)

func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicPath(r.Method, r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		tok, err := BearerToken(r.Header.Get("Authorization"))
		if err != nil {
			writeAuthErr(w, http.StatusUnauthorized, ErrUnauthorized)
			return
		}
		p, err := s.AuthenticateAccess(r.Context(), tok)
		if err != nil {
			writeAuthErr(w, http.StatusUnauthorized, ErrUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), p)))
	})
}

func isPublicPath(method, path string) bool {
	if method == http.MethodGet && path == "/healthz" {
		return true
	}
	if method == http.MethodPost {
		switch path {
		case "/v1/auth/bootstrap", "/v1/auth/login", "/v1/auth/refresh":
			return true
		}
	}
	return false
}

func writeAuthErr(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write([]byte(`{"error":"` + err.Error() + `"}`))
}
