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

	// YNAB configuration endpoints
	s.router.HandleFunc("/api/ynab/config", s.authenticate(s.GetYNABConfig)).Methods("GET")
	s.router.HandleFunc("/api/ynab/config", s.authenticate(s.UpdateYNABConfig)).Methods("PUT")
	s.router.HandleFunc("/api/ynab/sync/categories", s.authenticate(s.SyncYNABCategories)).Methods("POST")
}

// GetYNABConfig handles GET /api/ynab/config
func (s *Server) GetYNABConfig(w http.ResponseWriter, r *http.Request) {
	s.ynabHandler.GetYNABConfig(w, r)
}

// UpdateYNABConfig handles PUT /api/ynab/config
func (s *Server) UpdateYNABConfig(w http.ResponseWriter, r *http.Request) {
	s.ynabHandler.UpdateYNABConfig(w, r)
}

// SyncYNABCategories handles POST /api/ynab/sync/categories
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
		// Authentication logic here
		// For now, we'll just pass a dummy user ID in the context
		ctx := context.WithValue(r.Context(), "user_id", "test_user")

		next(w, r.WithContext(ctx))
	}
}
