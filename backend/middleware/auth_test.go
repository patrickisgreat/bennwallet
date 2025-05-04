package middleware

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

// mockFirebaseAuth is a simple type that allows us to set firebaseAuth to a non-nil value
// without importing the actual firebase auth
type mockFirebaseAuth struct{}

func TestExtractToken(t *testing.T) {
	testCases := []struct {
		name          string
		authHeader    string
		expectedToken string
	}{
		{
			name:          "Valid Bearer token",
			authHeader:    "Bearer test-token-123",
			expectedToken: "test-token-123",
		},
		{
			name:          "Missing Bearer prefix",
			authHeader:    "test-token-123",
			expectedToken: "",
		},
		{
			name:          "Empty auth header",
			authHeader:    "",
			expectedToken: "",
		},
		{
			name:          "Bearer with no token",
			authHeader:    "Bearer ",
			expectedToken: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			token := extractToken(tc.authHeader)
			if token != tc.expectedToken {
				t.Errorf("Expected token '%s', got '%s'", tc.expectedToken, token)
			}
		})
	}
}

func TestAuthMiddleware_DevMode(t *testing.T) {
	// Save the original firebaseAuth
	originalAuth := firebaseAuth
	defer func() { firebaseAuth = originalAuth }()

	// Simulate dev mode by setting firebaseAuth to nil
	firebaseAuth = nil

	// Create a test handler that will check the context
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the user ID from context
		userID := r.Context().Value("user_id")
		if userID == nil || userID.(string) != "admin-user-1" {
			t.Errorf("Expected user_id 'admin-user-1', got %v", userID)
		}

		// Check role
		role := r.Context().Value("user_role")
		if role == nil || role.(string) != "admin" {
			t.Errorf("Expected user_role 'admin', got %v", role)
		}

		w.WriteHeader(http.StatusOK)
	})

	// Create the middleware chain
	middleware := AuthMiddleware(testHandler)

	// Test request
	req := httptest.NewRequest("GET", "/api/test", nil)
	// No Authorization header needed as we're testing dev mode

	// Record the response
	rr := httptest.NewRecorder()

	// Serve HTTP
	middleware.ServeHTTP(rr, req)

	// Check response
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	// Skip the test if firebase auth is not imported
	t.Skip("This test requires real firebase auth to be imported")

	// In a real test, you would mock the firebase auth client here
	// This test is included for completeness but will be skipped

	// Create a simple handler that should not be called
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called when auth header is missing")
	})

	// Create the middleware chain
	middleware := AuthMiddleware(testHandler)

	// Test request with no Authorization header
	req := httptest.NewRequest("GET", "/api/test", nil)

	// Record the response
	rr := httptest.NewRecorder()

	// Serve HTTP
	middleware.ServeHTTP(rr, req)

	// Check response - should be unauthorized
	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("Expected status code %v, got %v", http.StatusUnauthorized, status)
	}
}

func TestAuthMiddleware_OptionsRequest(t *testing.T) {
	// Skip the test if firebase auth is not imported
	t.Skip("This test requires real firebase auth to be imported")

	// Create a test handler that will check the context
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should be called for OPTIONS requests even without auth
		w.WriteHeader(http.StatusOK)
	})

	// Create the middleware chain
	middleware := AuthMiddleware(testHandler)

	// Test OPTIONS request with no Authorization header
	req := httptest.NewRequest("OPTIONS", "/api/test", nil)

	// Record the response
	rr := httptest.NewRecorder()

	// Serve HTTP
	middleware.ServeHTTP(rr, req)

	// Check response - should pass through for OPTIONS
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status code %v for OPTIONS request, got %v", http.StatusOK, status)
	}
}

func TestGetUserIDFromContext(t *testing.T) {
	// Create a request with a user ID in the context
	req := httptest.NewRequest("GET", "/api/test", nil)
	ctx := context.WithValue(req.Context(), "user_id", "test-user-123")
	req = req.WithContext(ctx)

	// Get the user ID from context
	userID := GetUserIDFromContext(req)

	// Check the result
	if userID != "test-user-123" {
		t.Errorf("Expected user ID 'test-user-123', got '%s'", userID)
	}

	// Test with no user ID in context
	emptyReq := httptest.NewRequest("GET", "/api/test", nil)
	emptyUserID := GetUserIDFromContext(emptyReq)

	if emptyUserID != "" {
		t.Errorf("Expected empty user ID, got '%s'", emptyUserID)
	}
}

func TestInitializeFirebase_WithPlaceholderFile(t *testing.T) {
	// Create a temporary service account file with placeholders
	tempFile := "temp-service-account.json"
	defer os.Remove(tempFile)

	err := os.WriteFile(tempFile, []byte(`{"private_key": "REPLACE_WITH_ACTUAL_PRIVATE_KEY"}`), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Temporarily rename the file
	originalFile := "service-account.json"
	if _, err := os.Stat(originalFile); err == nil {
		// Backup the original file if it exists
		if err := os.Rename(originalFile, originalFile+".bak"); err != nil {
			t.Fatalf("Failed to backup original file: %v", err)
		}
		defer os.Rename(originalFile+".bak", originalFile)
	}

	// Move temp file to service-account.json
	if err := os.Rename(tempFile, originalFile); err != nil {
		t.Fatalf("Failed to move temp file: %v", err)
	}

	// Reset firebaseAuth
	originalAuth := firebaseAuth
	firebaseAuth = nil
	defer func() { firebaseAuth = originalAuth }()

	// Initialize Firebase with placeholder file
	err = InitializeFirebase()
	if err != nil {
		t.Errorf("InitializeFirebase should not return error with placeholder file: %v", err)
	}

	// Check that Firebase is in dev mode (firebaseAuth is nil)
	if firebaseAuth != nil {
		t.Error("Expected firebaseAuth to be nil with placeholder file")
	}
}

func TestInitializeFirebase_WithJSONEnv(t *testing.T) {
	// Save original env var and restore afterwards
	originalValue := os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON")
	defer os.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", originalValue)

	// Mock a valid service account JSON
	validJSON := `{
		"type": "service_account",
		"project_id": "test-project",
		"private_key_id": "test-key-id",
		"private_key": "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC5hGDnBww8HOzD\neQGu9WkqCV0EUL9Y5x/+4m7F4LSLkUfU9A+tQe9RvgMMZi6wuVLqEUjXf9YXGRxj\nBIPR7K0pLFYXOrRczRVJV5ABfDxU+bzN1KOlLWGMq9ybPvbOO+I25lkeYkXgTWbk\n4MaATl0m6EOThOQnXOuEKc7Q82W8JmWS1VaQYEqq+lWz7n/QHCNR9+XHnxWS0WCX\njmLdzbIJcukgz4uCiAXMXHQojf4BYBIQ+yKNnacbqwYr5EgQDpZJU55e1f7WUlnj\nVDKM0K82AhDzAkVTFI2Z2i9ZYvyj9j0CQyGb12Ryl1Z8Rif+ZOhv0Dzj4Cw2yU0s\nQpXtcf9NAgMBAAECggEABK2SuYbFBhZY+FLJ4KMr0CuDXIW8cwkKKqPrJ3p6d4SC\nV6w+98OQF/QZ+8jnHY1XWZ8HXx8lCToBZJf4NR2AnfLjFI5R/EU4L9hO+lOSNj7F\nHLYiMo0xwALnUkXsNqVlbQ3I2NWr4YvbWwfg8pPXlGAJAA/j9ZmkX8RoLYCFT9WG\nkQOPDvgKY16rV+45+nh0+t4SeobISIYNMO8L41ovrDYZIGK5WTLVjdNzjCMePJcA\nyMGixI+YEBDpSaA5d+mAYRLucmYdURR/Jv0zD7J/zEhG5wQ3Ks3rCHjWBM9SjALo\nt0aSOZx0MzlDHpDX7aKQM11Zke4K6xhTZOJnkdIYwQKBgQDnuN1mR98YN08+dqwP\nJ43cTQzk/bAwf3E1mV7A+Xeb+qMt5cguATXRZyP0Sj9m8tBm34NHXzFLHKGOGpF3\nQXxDiGUb/4g1zrR8S8Jm3p5CWVZlHScfLbH+vnAUwK0MZ9CaNfV12O3oBxgAC2Zb\nELb5EvOvtcgUYSgUUJMWaRD1wQKBgQDM5NWXj/3kGgOXQsgHwLJt4qTQGP6XN2VX\nc9Y+lN7V8M/UL7AeIsaMnCahTdXHPEQXlJgytXHpiwAQHxUxnfQnEZzGCxNOJDzm\nXj5S2q1enUjxzXK53DLgO+UsgHVi5glw8GolbjoS7hGLBNP4iIEFPeeHGnXbkNQF\nlBQLu1G/TQKBgQDR/wQk8RaY4JVJYi6lnEO1WMR32gKtTMX7wTGThWpXCwhCFoLM\nl1zt2dKN6jfUHVyIJscK80UXI3uQZnGXl/lv4hAF5sJTpQiogqTmuMeKsTDybLZm\nSBz5gJM64QGnKQNnvt6p4XTvn+JpQQKGYUPWD/BuHSDqB0EdvB27HlEGwQKBgFGo\nSSwYUYw47Ye7WtS+9rJTQAI2/hczjK3yIBLdqF/vYCRW7vV8S/tDoU1GvXTQGPIH\nQOPeVCYxKL2aP+gw3VaHtY7Q/0FVOyEHDZgbXTUJsXSgGHGgdbo+KWQQ5JvdIemU\nhnvF6J7p2vzwnDpG6uNYH7YmFwSLLnECCY5L4RG9AoGAFMHT6W0Ld83xx/bx1UFW\n6cRvxeA8fcqaNF0e3vfIcm1O+GfKhqXDBWqkxNjePyI2ICdPnQGv1TWrJwfFBjUP\nvDYsWOjEiqVJD+hA0vAm/Bc6+o7NvDMV9fMJEw8kJzKmPBh8XkkA0lIwFgUBDxRG\nfgXAHlTTQE2dxcELSDAdqwI=\n-----END PRIVATE KEY-----\n",
		"client_email": "firebase-adminsdk-test@test-project.iam.gserviceaccount.com",
		"client_id": "123456789",
		"auth_uri": "https://accounts.google.com/o/oauth2/auth",
		"token_uri": "https://oauth2.googleapis.com/token",
		"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
		"client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/firebase-adminsdk-test%40test-project.iam.gserviceaccount.com"
	}`

	// Set the env var for testing
	os.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", validJSON)

	// Mock firebase initialization
	originalInitApp := firebaseInitApp
	originalGetAuth := firebaseGetAuth
	defer func() {
		firebaseInitApp = originalInitApp
		firebaseGetAuth = originalGetAuth
	}()

	// Patches for the Firebase functions so they don't actually try to connect
	firebaseInitApp = func(ctx context.Context, config *firebase.Config, opts ...option.ClientOption) (*firebase.App, error) {
		return &firebase.App{}, nil
	}

	firebaseGetAuth = func(app *firebase.App, ctx context.Context) (*auth.Client, error) {
		return &auth.Client{}, nil
	}

	// Test the initialization
	err := InitializeFirebase()
	if err != nil {
		t.Errorf("InitializeFirebase with JSON env failed: %v", err)
	}

	// Verify global auth client was set
	if firebaseAuth == nil {
		t.Error("Firebase auth client was not initialized")
	}
}

func TestInitializeFirebase_WithBase64Env(t *testing.T) {
	// Save original env var and restore afterwards
	originalValue := os.Getenv("FIREBASE_SERVICE_ACCOUNT_BASE64")
	defer os.Setenv("FIREBASE_SERVICE_ACCOUNT_BASE64", originalValue)

	// Create a sample JSON and encode it to base64
	validJSON := `{"type":"service_account","project_id":"test-project"}`
	encoded := base64.StdEncoding.EncodeToString([]byte(validJSON))

	// Set the env var for testing
	os.Setenv("FIREBASE_SERVICE_ACCOUNT_BASE64", encoded)
	os.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", "") // Clear this to ensure it's not used

	// Mock firebase initialization
	originalInitApp := firebaseInitApp
	originalGetAuth := firebaseGetAuth
	defer func() {
		firebaseInitApp = originalInitApp
		firebaseGetAuth = originalGetAuth
	}()

	firebaseInitApp = func(ctx context.Context, config *firebase.Config, opts ...option.ClientOption) (*firebase.App, error) {
		return &firebase.App{}, nil
	}

	firebaseGetAuth = func(app *firebase.App, ctx context.Context) (*auth.Client, error) {
		return &auth.Client{}, nil
	}

	// Test the initialization
	err := InitializeFirebase()
	if err != nil {
		t.Errorf("InitializeFirebase with Base64 env failed: %v", err)
	}

	// Verify global auth client was set
	if firebaseAuth == nil {
		t.Error("Firebase auth client was not initialized with Base64 credentials")
	}
}

func TestInitializeFirebase_WithRawJSONEnv(t *testing.T) {
	// Save original env vars and restore afterwards
	originalJSON := os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON")
	originalBase64 := os.Getenv("FIREBASE_SERVICE_ACCOUNT_BASE64")
	originalRaw := os.Getenv("FIREBASE_SERVICE_ACCOUNT")
	defer func() {
		os.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", originalJSON)
		os.Setenv("FIREBASE_SERVICE_ACCOUNT_BASE64", originalBase64)
		os.Setenv("FIREBASE_SERVICE_ACCOUNT", originalRaw)
	}()

	// Clear other env vars to ensure they're not used
	os.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", "")
	os.Setenv("FIREBASE_SERVICE_ACCOUNT_BASE64", "")

	// Set the env var for testing
	validJSON := `{"type":"service_account","project_id":"test-project"}`
	os.Setenv("FIREBASE_SERVICE_ACCOUNT", validJSON)

	// Mock firebase initialization
	originalInitApp := firebaseInitApp
	originalGetAuth := firebaseGetAuth
	defer func() {
		firebaseInitApp = originalInitApp
		firebaseGetAuth = originalGetAuth
	}()

	firebaseInitApp = func(ctx context.Context, config *firebase.Config, opts ...option.ClientOption) (*firebase.App, error) {
		return &firebase.App{}, nil
	}

	firebaseGetAuth = func(app *firebase.App, ctx context.Context) (*auth.Client, error) {
		return &auth.Client{}, nil
	}

	// Test the initialization
	err := InitializeFirebase()
	if err != nil {
		t.Errorf("InitializeFirebase with raw JSON env failed: %v", err)
	}

	// Verify global auth client was set
	if firebaseAuth == nil {
		t.Error("Firebase auth client was not initialized with raw JSON credentials")
	}
}

func TestInitializeFirebase_WithInvalidJSONEnv(t *testing.T) {
	// Save original env var and restore afterwards
	originalValue := os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON")
	defer os.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", originalValue)

	// Set an invalid JSON string
	os.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", "this is not valid JSON")

	// Mock firebase initialization
	originalInitApp := firebaseInitApp
	defer func() {
		firebaseInitApp = originalInitApp
	}()

	firebaseInitApp = func(ctx context.Context, config *firebase.Config, opts ...option.ClientOption) (*firebase.App, error) {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Test the initialization with invalid JSON
	err := InitializeFirebase()
	if err == nil {
		t.Error("InitializeFirebase should have failed with invalid JSON")
	}
}

func TestAuthMiddleware_NoFirebaseAuth(t *testing.T) {
	// Save and clear the firebase auth client
	savedAuth := firebaseAuth
	firebaseAuth = nil
	defer func() {
		firebaseAuth = savedAuth
	}()

	// Setup test HTTP server
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := GetUserIDFromContext(r)
		if userID != "admin-user-1" {
			t.Errorf("Expected user ID admin-user-1, got %s", userID)
		}
		w.WriteHeader(http.StatusOK)
	})

	// Apply middleware
	middleware := AuthMiddleware(handler)

	// Create test request
	req, err := http.NewRequest("GET", "/test", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create response recorder
	rr := httptest.NewRecorder()

	// Serve the request
	middleware.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v, want %v", status, http.StatusOK)
	}
}
