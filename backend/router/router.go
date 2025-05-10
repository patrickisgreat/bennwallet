package router

import (
	"bennwallet/backend/handlers"
	"net/http"

	"github.com/gorilla/mux"
)

func SetupRouter(apiRouter *mux.Router) {
	// Permissions endpoints
	apiRouter.HandleFunc("/permissions", handlers.GetUserPermissions).Methods("GET")
	apiRouter.HandleFunc("/permissions", handlers.GrantPermission).Methods("POST")
	apiRouter.HandleFunc("/permissions/all", handlers.GetAllPermissions).Methods("GET")
	apiRouter.HandleFunc("/permissions/{id}", handlers.RevokePermission).Methods("DELETE")
}

func StartServer() {
	apiRouter := mux.NewRouter()
	SetupRouter(apiRouter)
	http.ListenAndServe(":8080", apiRouter)
}
