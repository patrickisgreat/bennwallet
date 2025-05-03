package middleware

import (
	"log"
	"net/http"
	"os"
	"strings"
)

// EnableCORS creates a middleware that handles CORS headers
func EnableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the origin from the request
		origin := r.Header.Get("Origin")

		// Define allowed origins
		allowedOrigins := getAllowedOrigins()

		// Check if the origin is allowed
		if isAllowedOrigin(origin, allowedOrigins) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else if isDevelopmentMode() {
			// In development mode, be more permissive
			log.Printf("Development mode: allowing origin %s", origin)
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			// For when no origin is provided or not allowed
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigins[0])
		}

		// Set other CORS headers - expand the allowed headers to include all common ones
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set("Access-Control-Allow-Headers",
			"Content-Type, Authorization, X-Requested-With, Accept, Origin, Access-Control-Request-Method, Access-Control-Request-Headers")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "3600") // Cache preflight request results

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isDevelopmentMode returns true if running in development mode
func isDevelopmentMode() bool {
	env := os.Getenv("ENV")
	return env == "" || env == "development" || env == "dev"
}

// getAllowedOrigins returns the list of allowed origins based on environment
func getAllowedOrigins() []string {
	// Production environment - check environment variable first
	corsOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if corsOrigins != "" {
		return strings.Split(corsOrigins, ",")
	}

	// Default allowed origins including production and development
	return []string{
		"https://bennwallet-prod.fly.dev",  // Production
		"https://benwallett-ab39d.web.app", // Firebase hosting
		"http://localhost:5173",            // Vite development server
		"http://localhost:3000",            // Alternative local development
		"http://localhost:8080",            // Backend port
	}
}

// isAllowedOrigin checks if the provided origin is in the allowed list
func isAllowedOrigin(origin string, allowedOrigins []string) bool {
	if origin == "" {
		return false
	}

	for _, allowed := range allowedOrigins {
		if origin == allowed {
			return true
		}
	}

	return false
}
