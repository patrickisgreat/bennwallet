package api

import (
	"context"
	"database/sql"
	"net/http"

	"bennwallet/backend/handlers"

	"github.com/gorilla/mux"
)

// Server represents the API server
type Server struct {
	db          *sql.DB
	router      *mux.Router
	ynabHandler *handlers.YNABHandler
}

// NewServer creates a new API server
func NewServer(db *sql.DB) *Server {
	s := &Server{
		db:          db,
		router:      mux.NewRouter(),
		ynabHandler: handlers.NewYNABHandler(db),
	}
	s.RegisterRoutes()
	return s
}

// RegisterRoutes registers all API routes
func (s *Server) RegisterRoutes() {
	// ... existing routes ...

	// Add a route for linking Firebase UID with username
	s.router.HandleFunc("/users/link-firebase", s.LinkFirebaseUser).Methods("POST")

	// YNAB configuration endpoints
	s.router.HandleFunc("/ynab/config", s.authenticate(s.GetYNABConfig)).Methods("GET")
	s.router.HandleFunc("/ynab/config", s.authenticate(s.UpdateYNABConfig)).Methods("PUT")
	s.router.HandleFunc("/ynab/sync/categories", s.authenticate(s.SyncYNABCategories)).Methods("POST")
}

// LinkFirebaseUser handles POST /users/link-firebase
func (s *Server) LinkFirebaseUser(w http.ResponseWriter, r *http.Request) {
	handlers.CreateOrUpdateFirebaseUser(w, r)
}

// GetYNABConfig handles GET /ynab/config
func (s *Server) GetYNABConfig(w http.ResponseWriter, r *http.Request) {
	s.ynabHandler.GetYNABConfig(w, r)
}

// UpdateYNABConfig handles PUT /ynab/config
func (s *Server) UpdateYNABConfig(w http.ResponseWriter, r *http.Request) {
	s.ynabHandler.UpdateYNABConfig(w, r)
}

// SyncYNABCategories handles POST /ynab/sync/categories
func (s *Server) SyncYNABCategories(w http.ResponseWriter, r *http.Request) {
	s.ynabHandler.SyncYNABCategories(w, r)
}

// Handler returns the HTTP handler for the API server
func (s *Server) Handler() http.Handler {
	return s.router
}

// authenticate is a middleware that checks for authentication
func (s *Server) authenticate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the user ID from query parameters
		userId := r.URL.Query().Get("userId")

		if userId == "" {
			http.Error(w, "User ID is required", http.StatusUnauthorized)
			return
		}

		// Add the user ID to the context
		ctx := context.WithValue(r.Context(), "user_id", userId)

		next(w, r.WithContext(ctx))
	}
}
