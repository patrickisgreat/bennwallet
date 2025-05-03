package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestIsAllowedOrigin(t *testing.T) {
	allowedOrigins := []string{
		"https://example.com",
		"http://localhost:5173",
	}

	testCases := []struct {
		name     string
		origin   string
		expected bool
	}{
		{
			name:     "Allowed origin",
			origin:   "https://example.com",
			expected: true,
		},
		{
			name:     "Another allowed origin",
			origin:   "http://localhost:5173",
			expected: true,
		},
		{
			name:     "Disallowed origin",
			origin:   "https://evil.com",
			expected: false,
		},
		{
			name:     "Empty origin",
			origin:   "",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isAllowedOrigin(tc.origin, allowedOrigins)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v for origin %s", tc.expected, result, tc.origin)
			}
		})
	}
}

func TestGetAllowedOrigins(t *testing.T) {
	// Save original environment variable
	originalCors := os.Getenv("CORS_ALLOWED_ORIGINS")
	defer os.Setenv("CORS_ALLOWED_ORIGINS", originalCors)

	// Test with environment variable set
	testOrigins := "https://test1.com,https://test2.com"
	os.Setenv("CORS_ALLOWED_ORIGINS", testOrigins)

	origins := getAllowedOrigins()
	if len(origins) != 2 {
		t.Errorf("Expected 2 origins, got %d", len(origins))
	}
	if origins[0] != "https://test1.com" || origins[1] != "https://test2.com" {
		t.Errorf("Expected specific origins, got %v", origins)
	}

	// Test with environment variable unset
	os.Unsetenv("CORS_ALLOWED_ORIGINS")
	origins = getAllowedOrigins()
	if len(origins) < 3 {
		t.Errorf("Expected at least 3 default origins, got %d", len(origins))
	}

	// Check that default origins include common development servers
	hasLocalhost := false
	for _, origin := range origins {
		if strings.Contains(origin, "localhost") {
			hasLocalhost = true
			break
		}
	}
	if !hasLocalhost {
		t.Error("Default origins should include localhost development servers")
	}
}

func TestIsDevelopmentMode(t *testing.T) {
	// Save original environment variable
	originalEnv := os.Getenv("ENV")
	defer os.Setenv("ENV", originalEnv)

	// Test with ENV unset
	os.Unsetenv("ENV")
	if !isDevelopmentMode() {
		t.Error("With ENV unset, should be in development mode")
	}

	// Test with ENV set to development
	os.Setenv("ENV", "development")
	if !isDevelopmentMode() {
		t.Error("With ENV=development, should be in development mode")
	}

	// Test with ENV set to production
	os.Setenv("ENV", "production")
	if isDevelopmentMode() {
		t.Error("With ENV=production, should not be in development mode")
	}
}

func TestEnableCORS(t *testing.T) {
	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with the CORS middleware
	handler := EnableCORS(testHandler)

	testCases := []struct {
		name           string
		method         string
		origin         string
		expectedStatus int
		checkHeaders   bool
	}{
		{
			name:           "Normal GET request with allowed origin",
			method:         "GET",
			origin:         "http://localhost:5173",
			expectedStatus: http.StatusOK,
			checkHeaders:   true,
		},
		{
			name:           "OPTIONS preflight request",
			method:         "OPTIONS",
			origin:         "http://localhost:5173",
			expectedStatus: http.StatusOK,
			checkHeaders:   true,
		},
		{
			name:           "Request with no origin",
			method:         "GET",
			origin:         "",
			expectedStatus: http.StatusOK,
			checkHeaders:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test request
			req := httptest.NewRequest(tc.method, "/api/test", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Serve the request
			handler.ServeHTTP(rr, req)

			// Check status code
			if rr.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, rr.Code)
			}

			// Check headers if needed
			if tc.checkHeaders {
				// For OPTIONS requests, we expect CORS headers
				allowMethods := rr.Header().Get("Access-Control-Allow-Methods")
				if allowMethods == "" {
					t.Error("Expected Access-Control-Allow-Methods header to be set")
				}

				allowHeaders := rr.Header().Get("Access-Control-Allow-Headers")
				if allowHeaders == "" {
					t.Error("Expected Access-Control-Allow-Headers header to be set")
				}

				// For empty origin, the default origin is set instead,
				// so we can still expect the header to exist
				allowOrigin := rr.Header().Get("Access-Control-Allow-Origin")

				// In development mode (which tests run in), any origin should be allowed
				if tc.origin != "" && allowOrigin != tc.origin {
					// Only check this in development mode
					if isDevelopmentMode() {
						t.Errorf("Expected origin %s to be allowed in dev mode, got %s", tc.origin, allowOrigin)
					}
				}
			}
		})
	}
}

func TestCORSWithNonAllowedOrigin(t *testing.T) {
	// Save original environment variable to restore later
	originalEnv := os.Getenv("ENV")
	defer os.Setenv("ENV", originalEnv)

	// Set production environment to test stricter CORS
	os.Setenv("ENV", "production")

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with the CORS middleware
	handler := EnableCORS(testHandler)

	// Create test request with non-allowed origin
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Origin", "https://evil.com")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(rr, req)

	// Check that a default origin was set, not the requested one
	allowOrigin := rr.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin == "https://evil.com" {
		t.Error("Non-allowed origin should not be reflected in Access-Control-Allow-Origin")
	}

	// Check that some origin was set (the default one)
	if allowOrigin == "" {
		t.Error("Access-Control-Allow-Origin should be set to a default value")
	}
}
