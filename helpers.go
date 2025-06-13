package main

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addHeaderIfMissing := func(key, value string) {
			if w.Header().Get(key) == "" {
				w.Header().Set(key, value)
			}
		}

		addHeaderIfMissing("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'")
		addHeaderIfMissing("X-Frame-Options", "DENY")
		addHeaderIfMissing("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		addHeaderIfMissing("X-Content-Type-Options", "nosniff")

		// Optional: tighten caching for sensitive endpoints
		if r.URL.Path == "/login" || r.URL.Path == "/" {
			addHeaderIfMissing("Cache-Control", "no-cache, no-store, must-revalidate")
			addHeaderIfMissing("Pragma", "no-cache")
			addHeaderIfMissing("Expires", "0")
		}

		next.ServeHTTP(w, r)
	})
}

func generateRandomCSRFKey() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		AppLogger.Printf("Unable to generate CSRF key: %v", err)
	}
	return key
}

func GenerateSessionID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		AppLogger.Printf("Unable to generate session ID: %v", err)
		panic("unable to generate session ID")
	}
	return hex.EncodeToString(b)
}

func ApplyMiddlewares(handler http.Handler, withCSRF bool, withLogin bool) http.Handler {
	h := handler
	if withLogin {
		h = RequireLogin(h)
	}
	if withCSRF && csrfMiddleware != nil {
		h = csrfMiddleware(h)
		AppLogger.Printf("CSRF middleware applied to handler")
	}
	h = SecurityHeadersMiddleware(h)
	return h
}
