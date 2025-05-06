package middleware

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

// Define context keys
type contextKey string

const UserIDKey contextKey = "user_id"
const UserRoleKey contextKey = "user_role"

var firebaseAuth *auth.Client

// InitializeFirebase initializes the Firebase Admin SDK
func InitializeFirebase() error {
	log.Println("Starting Firebase initialization...")

	// Debug: Check which environment variables are available
	hasJSON := os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON") != ""
	hasBase64 := os.Getenv("FIREBASE_SERVICE_ACCOUNT_BASE64") != ""
	hasEnv := os.Getenv("FIREBASE_SERVICE_ACCOUNT") != ""

	log.Printf("Firebase env vars present: JSON=%v, Base64=%v, Raw=%v", hasJSON, hasBase64, hasEnv)

	// First check for direct JSON Firebase credentials in environment variables (production)
	firebaseCredentialsJSON := os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON")
	if firebaseCredentialsJSON != "" {
		log.Println("Using JSON Firebase credentials from environment")
		log.Printf("Credentials JSON length: %d", len(firebaseCredentialsJSON))

		// Initialize Firebase with credentials JSON directly
		opt := option.WithCredentialsJSON([]byte(firebaseCredentialsJSON))
		config := &firebase.Config{ProjectID: "benwallett-ab39d"}

		app, err := firebase.NewApp(context.Background(), config, opt)
		if err != nil {
			log.Printf("Error initializing Firebase app from JSON env: %v", err)
			return err
		}

		firebaseAuth, err = app.Auth(context.Background())
		if err != nil {
			log.Printf("Error getting Firebase Auth client: %v", err)
			return err
		}

		log.Println("Firebase Admin SDK initialized successfully with JSON credentials")
		return nil
	}

	// Next check for Base64-encoded Firebase credentials in environment variables
	firebaseCredentialsBase64 := os.Getenv("FIREBASE_SERVICE_ACCOUNT_BASE64")
	if firebaseCredentialsBase64 != "" {
		log.Println("Using base64-encoded Firebase credentials from environment")

		// Decode the base64 string
		credBytes, err := base64.StdEncoding.DecodeString(firebaseCredentialsBase64)
		if err != nil {
			log.Printf("Error decoding base64 Firebase credentials: %v", err)
			return err
		}

		// Initialize Firebase with credentials JSON directly
		opt := option.WithCredentialsJSON(credBytes)
		config := &firebase.Config{ProjectID: "benwallett-ab39d"}

		app, err := firebase.NewApp(context.Background(), config, opt)
		if err != nil {
			log.Printf("Error initializing Firebase app from base64 env: %v", err)
			return err
		}

		firebaseAuth, err = app.Auth(context.Background())
		if err != nil {
			log.Printf("Error getting Firebase Auth client: %v", err)
			return err
		}

		log.Println("Firebase Admin SDK initialized successfully with base64 credentials")
		return nil
	}

	// Next check for raw JSON Firebase credentials in environment variables
	firebaseCredentials := os.Getenv("FIREBASE_SERVICE_ACCOUNT")
	if firebaseCredentials != "" {
		log.Println("Using Firebase credentials from environment variable")

		// Initialize Firebase with credentials JSON directly
		opt := option.WithCredentialsJSON([]byte(firebaseCredentials))
		config := &firebase.Config{ProjectID: "benwallett-ab39d"}

		app, err := firebase.NewApp(context.Background(), config, opt)
		if err != nil {
			log.Printf("Error initializing Firebase app from env variable: %v", err)
			return err
		}

		firebaseAuth, err = app.Auth(context.Background())
		if err != nil {
			log.Printf("Error getting Firebase Auth client: %v", err)
			return err
		}

		log.Println("Firebase Admin SDK initialized successfully with env credentials")
		return nil
	}

	// Use default application credentials when no specific credentials are provided
	// This is designed for development environments
	log.Println("No specific Firebase credentials found, using application default credentials")
	log.Println("Running in development mode with auth checks disabled")

	// For tests and development, enable authentication bypass
	return nil
}

// AuthMiddleware verifies Firebase JWT tokens from the Authorization header
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If Firebase auth is not initialized, skip token verification (dev mode)
		if firebaseAuth == nil {
			log.Println("Firebase auth not initialized, skipping token verification")

			// In dev mode, default to the first admin user for testing
			ctx := context.WithValue(r.Context(), UserIDKey, "admin-user-1")
			ctx = context.WithValue(ctx, UserRoleKey, "admin")
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Get the Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header is required", http.StatusUnauthorized)
			return
		}

		// Skip auth for OPTIONS requests (CORS preflight)
		if r.Method == "OPTIONS" {
			next.ServeHTTP(w, r)
			return
		}

		// Get the Authorization header
		idToken := extractToken(authHeader)

		if idToken == "" {
			// Fallback to query parameter for backward compatibility
			idToken = r.URL.Query().Get("auth")

			// Also try the userId parameter for very old clients
			if idToken == "" {
				userId := r.URL.Query().Get("userId")
				if userId != "" {
					// For backward compatibility, still allow access with just userId
					// This should be removed after all clients are updated
					log.Printf("Warning: Request using deprecated userId parameter: %s", userId)
					ctx := context.WithValue(r.Context(), UserIDKey, userId)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}

				http.Error(w, "Unauthorized: No token provided", http.StatusUnauthorized)
				return
			}
		}

		// Verify the token with Firebase
		token, err := verifyToken(idToken)
		if err != nil {
			log.Printf("Error verifying token: %v", err)
			http.Error(w, "Unauthorized: Invalid token", http.StatusUnauthorized)
			return
		}

		// Add the user ID to the request context
		ctx := context.WithValue(r.Context(), UserIDKey, token.UID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractToken gets the token from the Authorization header
func extractToken(authHeader string) string {
	if authHeader == "" {
		return ""
	}

	parts := strings.Split(authHeader, "Bearer ")
	if len(parts) != 2 {
		return ""
	}

	return parts[1]
}

// verifyToken verifies the Firebase JWT token
func verifyToken(idToken string) (*auth.Token, error) {
	if firebaseAuth == nil {
		return nil, errors.New("Firebase auth client not initialized")
	}

	ctx := context.Background()
	token, err := firebaseAuth.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, fmt.Errorf("error verifying ID token: %w", err)
	}

	return token, nil
}

// GetUserIDFromContext retrieves the user ID from the request context
func GetUserIDFromContext(r *http.Request) string {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		return ""
	}
	return userID
}
