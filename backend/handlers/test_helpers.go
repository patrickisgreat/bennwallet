package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
)

// MockAuthContext adds a mock user ID to the request context for testing
func MockAuthContext(req *http.Request, userID string) *http.Request {
	ctx := context.WithValue(req.Context(), "user_id", userID)
	return req.WithContext(ctx)
}

// NewAuthenticatedRequest creates a new HTTP request with a mock authenticated user
func NewAuthenticatedRequest(method, url string, body interface{}) *http.Request {
	var req *http.Request

	if body != nil {
		// Convert body to JSON buffer if needed
		buf, _ := json.Marshal(body)
		req = httptest.NewRequest(method, url, bytes.NewBuffer(buf))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, url, nil)
	}

	// Add mock user authentication
	return MockAuthContext(req, "test-user-id")
}
