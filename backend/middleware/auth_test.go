package middleware

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

// Define the function variables that will be mocked
var (
	firebaseInitApp = func(ctx context.Context, config *firebase.Config, opts ...option.ClientOption) (*firebase.App, error) {
		return nil, fmt.Errorf("default mock - should be overridden in tests")
	}

	firebaseGetAuth = func(app *firebase.App, ctx context.Context) (*auth.Client, error) {
		return nil, fmt.Errorf("default mock - should be overridden in tests")
	}
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
		userID := r.Context().Value(UserIDKey)
		if userID == nil || userID.(string) != "admin-user-1" {
			t.Errorf("Expected user_id 'admin-user-1', got %v", userID)
		}

		// Check role
		role := r.Context().Value(UserRoleKey)
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
	ctx := context.WithValue(req.Context(), UserIDKey, "test-user-123")
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
	// Save and reset original env vars
	originalJSON := os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON")
	defer os.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", originalJSON)

	// Clear environment variable to simulate dev mode
	os.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", "")

	// Reset firebaseAuth
	originalAuth := firebaseAuth
	firebaseAuth = nil
	defer func() { firebaseAuth = originalAuth }()

	// Initialize Firebase - should go into dev mode
	err := InitializeFirebase()
	if err != nil {
		t.Errorf("InitializeFirebase should not return error in dev mode: %v", err)
	}

	// Check that Firebase is in dev mode (firebaseAuth is nil)
	if firebaseAuth != nil {
		t.Error("Expected firebaseAuth to be nil in dev mode")
	}
}

func TestInitializeFirebase_WithJSONEnv(t *testing.T) {
	// Save original env vars and restore afterwards
	originalJSON := os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON")
	originalBase64 := os.Getenv("FIREBASE_SERVICE_ACCOUNT_BASE64")
	originalRaw := os.Getenv("FIREBASE_SERVICE_ACCOUNT")
	defer func() {
		os.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", originalJSON)
		os.Setenv("FIREBASE_SERVICE_ACCOUNT_BASE64", originalBase64)
		os.Setenv("FIREBASE_SERVICE_ACCOUNT", originalRaw)
	}()

	// Clear other env vars first to ensure they're not used
	os.Setenv("FIREBASE_SERVICE_ACCOUNT_BASE64", "")
	os.Setenv("FIREBASE_SERVICE_ACCOUNT", "")

	// Save the original firebaseAuth value and mock functions
	originalAuth := firebaseAuth
	originalInitApp := firebaseInitApp
	originalGetAuth := firebaseGetAuth
	defer func() {
		firebaseAuth = originalAuth
		firebaseInitApp = originalInitApp
		firebaseGetAuth = originalGetAuth
	}()

	// Make sure firebaseAuth is nil before the test
	firebaseAuth = nil

	// Mock the Firebase functions - must be done before setting the environment variable
	firebaseInitApp = func(ctx context.Context, config *firebase.Config, opts ...option.ClientOption) (*firebase.App, error) {
		return &firebase.App{}, nil
	}

	firebaseGetAuth = func(app *firebase.App, ctx context.Context) (*auth.Client, error) {
		return &auth.Client{}, nil
	}

	// Create a simpler valid JSON without a complex private key
	validJSON := `{
		"type": "service_account",
		"project_id": "test-project",
		"private_key_id": "test-key-id",
		"client_email": "firebase-adminsdk-test@test-project.iam.gserviceaccount.com"
	}`

	// Set the env var for testing
	os.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", validJSON)

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
	// Save original env vars and restore afterwards
	originalJSON := os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON")
	originalBase64 := os.Getenv("FIREBASE_SERVICE_ACCOUNT_BASE64")
	originalRaw := os.Getenv("FIREBASE_SERVICE_ACCOUNT")
	defer func() {
		os.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", originalJSON)
		os.Setenv("FIREBASE_SERVICE_ACCOUNT_BASE64", originalBase64)
		os.Setenv("FIREBASE_SERVICE_ACCOUNT", originalRaw)
	}()

	// Clear ALL env vars to ensure they're not used
	os.Setenv("FIREBASE_SERVICE_ACCOUNT_BASE64", "")
	os.Setenv("FIREBASE_SERVICE_ACCOUNT", "")

	// Set an invalid JSON string
	os.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", "this is not valid JSON")

	// Mock firebase initialization with a function that returns an error
	originalInitApp := firebaseInitApp
	originalGetAuth := firebaseGetAuth
	defer func() {
		firebaseInitApp = originalInitApp
		firebaseGetAuth = originalGetAuth
	}()

	firebaseInitApp = func(ctx context.Context, config *firebase.Config, opts ...option.ClientOption) (*firebase.App, error) {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Save the original firebaseAuth value
	originalAuth := firebaseAuth
	defer func() {
		firebaseAuth = originalAuth
	}()

	// Make sure firebaseAuth is nil before the test
	firebaseAuth = nil

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
