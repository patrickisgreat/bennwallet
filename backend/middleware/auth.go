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

var firebaseAuth *auth.Client

// InitializeFirebase initializes the Firebase Admin SDK
func InitializeFirebase() error {
	// First check for Base64-encoded Firebase credentials in environment variables (production)
	firebaseCredentialsBase64 := os.Getenv("FIREBASE_SERVICE_ACCOUNT_BASE64")
	if firebaseCredentialsBase64 != "" {
		log.Println("Using base64-encoded Firebase credentials from environment")

		// Decode the base64 string
		credBytes, err := base64.StdEncoding.DecodeString(firebaseCredentialsBase64)
		if err != nil {
			log.Printf("Error decoding base64 Firebase credentials: %v", err)
			return err
		}

		// Create a temporary file with the credentials
		tmpFile, err := os.CreateTemp("", "firebase-*.json")
		if err != nil {
			log.Printf("Error creating temporary file for Firebase credentials: %v", err)
			return err
		}
		defer os.Remove(tmpFile.Name()) // Clean up temp file

		// Write the decoded credentials to the file
		if _, err := tmpFile.Write(credBytes); err != nil {
			log.Printf("Error writing Firebase credentials to temporary file: %v", err)
			return err
		}

		// Close the file
		if err := tmpFile.Close(); err != nil {
			log.Printf("Error closing temporary file: %v", err)
			return err
		}

		// Initialize Firebase with credentials from the temporary file
		opt := option.WithCredentialsFile(tmpFile.Name())
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

		// Create a temporary file with the credentials
		tmpFile, err := os.CreateTemp("", "firebase-*.json")
		if err != nil {
			log.Printf("Error creating temporary file for Firebase credentials: %v", err)
			return err
		}
		defer os.Remove(tmpFile.Name()) // Clean up temp file

		// Write the credentials to the file
		if _, err := tmpFile.Write([]byte(firebaseCredentials)); err != nil {
			log.Printf("Error writing Firebase credentials to temporary file: %v", err)
			return err
		}

		// Close the file
		if err := tmpFile.Close(); err != nil {
			log.Printf("Error closing temporary file: %v", err)
			return err
		}

		// Initialize Firebase with credentials from the temporary file
		opt := option.WithCredentialsFile(tmpFile.Name())
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

	// Finally, check if the service-account.json file exists and doesn't contain placeholders
	credentials, err := os.ReadFile("service-account.json")
	if err != nil {
		log.Printf("Could not read service-account.json: %v", err)
		log.Println("Running in development mode with auth checks disabled")
		return nil
	}

	// Check if the file contains placeholder text
	if strings.Contains(string(credentials), "REPLACE_WITH_ACTUAL_PRIVATE_KEY") {
		log.Println("Service account file contains placeholder values")
		log.Println("Running in development mode with auth checks disabled")
		return nil
	}

	// Use credentials from file
	log.Println("Using Firebase credentials from service-account.json")
	opt := option.WithCredentialsFile("service-account.json")
	config := &firebase.Config{ProjectID: "benwallett-ab39d"}

	app, err := firebase.NewApp(context.Background(), config, opt)
	if err != nil {
		log.Printf("Error initializing Firebase app from file: %v", err)
		return err
	}

	firebaseAuth, err = app.Auth(context.Background())
	if err != nil {
		log.Printf("Error getting Firebase Auth client: %v", err)
		return err
	}

	log.Println("Firebase Admin SDK initialized successfully with file credentials")
	return nil
}

// AuthMiddleware verifies Firebase JWT tokens from the Authorization header
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If Firebase auth is not initialized, skip token verification (dev mode)
		if firebaseAuth == nil {
			log.Println("Firebase auth not initialized, skipping token verification")

			// In dev mode, default to the first admin user for testing
			ctx := context.WithValue(r.Context(), "user_id", "admin-user-1")
			ctx = context.WithValue(ctx, "user_role", "admin")
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
					ctx := context.WithValue(r.Context(), "user_id", userId)
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
		ctx := context.WithValue(r.Context(), "user_id", token.UID)
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
	userID, ok := r.Context().Value("user_id").(string)
	if !ok {
		return ""
	}
	return userID
}
