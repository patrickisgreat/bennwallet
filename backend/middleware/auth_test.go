package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
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
