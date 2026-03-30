package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"echolog/backend/internal/blog"
	"echolog/backend/internal/health"
)

const apiPrefix = "/v1/echolog"

func main() {
	storePath, err := resolveStorePath()
	if err != nil {
		log.Fatalf("failed to resolve store path: %v", err)
	}

	service, err := blog.NewService(storePath)
	if err != nil {
		log.Fatalf("failed to initialize blog service: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health.Handler)
	mux.HandleFunc("GET "+apiPrefix+"/site", service.SiteHandler)
	mux.HandleFunc("GET "+apiPrefix+"/posts", service.PostsHandler)
	mux.HandleFunc("GET "+apiPrefix+"/posts/{slug}", service.PostDetailHandler)
	mux.HandleFunc("POST "+apiPrefix+"/auth/login", service.LoginHandler)
	mux.HandleFunc("POST "+apiPrefix+"/auth/logout", service.LogoutHandler)
	mux.HandleFunc("GET "+apiPrefix+"/auth/session", service.SessionHandler)
	mux.HandleFunc(apiPrefix+"/manage/settings", service.ManageSettingsHandler)
	mux.HandleFunc(apiPrefix+"/manage/posts", service.ManagePostsHandler)
	mux.HandleFunc(apiPrefix+"/manage/posts/{id}", service.ManagePostDetailHandler)

	addr := ":8080"
	log.Printf("backend server listening on %s", addr)
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func resolveStorePath() (string, error) {
	candidates := []string{
		filepath.Join("data", "store.local.json"),
		filepath.Join("..", "..", "data", "store.local.json"),
		filepath.Join("backend", "data", "store.local.json"),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", os.ErrNotExist
}
