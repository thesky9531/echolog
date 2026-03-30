package blog

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

const sessionCookieName = "echolog_session"

type NavItem struct {
	Label string `json:"label"`
	Href  string `json:"href"`
}

type SiteResponse struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Nav         []NavItem `json:"nav"`
	ICPNumber   string    `json:"icpNumber"`
	ICPLinkURL  string    `json:"icpLinkUrl"`
}

type NavItemConfig struct {
	Label string `json:"label"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

type SiteConfig struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Nav         []NavItemConfig `json:"nav"`
	ICPNumber   string          `json:"icpNumber"`
	ICPLinkURL  string          `json:"icpLinkUrl"`
}

type PublicPost struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Published   bool   `json:"published"`
	PublishedAt string `json:"publishedAt"`
	UpdatedAt   string `json:"updatedAt"`
	Excerpt     string `json:"excerpt"`
	Content     string `json:"content,omitempty"`
}

type AdminPost struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Published   bool   `json:"published"`
	Excerpt     string `json:"excerpt"`
	Content     string `json:"content"`
	PublishedAt string `json:"publishedAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type PostsResponse struct {
	Items []PublicPost `json:"items"`
}

type loginRequest struct {
	Secret string `json:"secret"`
}

type updateSettingsRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Nav         []NavItemConfig `json:"nav"`
	ICPNumber   string          `json:"icpNumber"`
	ICPLinkURL  string          `json:"icpLinkUrl"`
}

type upsertPostRequest struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Published   bool   `json:"published"`
	Excerpt     string `json:"excerpt"`
	Content     string `json:"content"`
	PublishedAt string `json:"publishedAt"`
}

type sessionResponse struct {
	Authenticated bool `json:"authenticated"`
}

type Service struct {
	store *Store
	auth  *AuthManager
}

func NewService(storePath string) (*Service, error) {
	store, err := NewStore(storePath)
	if err != nil {
		return nil, err
	}

	return &Service{
		store: store,
		auth:  NewAuthManager(24 * time.Hour),
	}, nil
}

func (s *Service) SiteHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.PublicSite())
}

func (s *Service) PostsHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, PostsResponse{Items: s.store.PublicPosts()})
}

func (s *Service) PostDetailHandler(w http.ResponseWriter, r *http.Request) {
	post, ok := s.store.PublicPost(strings.TrimSpace(r.PathValue("slug")))
	if !ok {
		writeError(w, http.StatusNotFound, "post not found")
		return
	}

	writeJSON(w, http.StatusOK, post)
}

func (s *Service) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	secret := strings.TrimSpace(req.Secret)
	if subtle.ConstantTimeCompare([]byte(secret), []byte(s.store.AdminSecret())) != 1 {
		writeError(w, http.StatusUnauthorized, "invalid secret")
		return
	}

	token := s.auth.CreateSession()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((24 * time.Hour).Seconds()),
	})

	writeJSON(w, http.StatusOK, sessionResponse{Authenticated: true})
}

func (s *Service) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		s.auth.DeleteSession(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	writeJSON(w, http.StatusOK, sessionResponse{Authenticated: false})
}

func (s *Service) SessionHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, sessionResponse{Authenticated: s.isAuthenticated(r)})
}

func (s *Service) ManageSettingsHandler(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.store.Settings())
	case http.MethodPut:
		var req updateSettingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		settings, err := normalizeSettings(req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		updated, err := s.store.UpdateSettings(settings)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, updated)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Service) ManagePostsHandler(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string][]AdminPost{"items": s.store.AdminPosts()})
	case http.MethodPost:
		var req upsertPostRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		post, err := normalizePost("", req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		created, err := s.store.UpsertPost(post)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, created)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Service) ManagePostDetailHandler(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "post id is required")
		return
	}

	switch r.Method {
	case http.MethodPut:
		var req upsertPostRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		post, err := normalizePost(id, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		updated, err := s.store.UpsertPost(post)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		if err := s.store.DeletePost(id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func normalizeSettings(req updateSettingsRequest) (SiteConfig, error) {
	settings := SiteConfig{
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		ICPNumber:   strings.TrimSpace(req.ICPNumber),
		ICPLinkURL:  strings.TrimSpace(req.ICPLinkURL),
	}

	if settings.Name == "" {
		return SiteConfig{}, errBadRequest("site name is required")
	}

	if settings.ICPLinkURL != "" {
		if _, err := url.ParseRequestURI(settings.ICPLinkURL); err != nil {
			return SiteConfig{}, errBadRequest("icp link must be a valid URL")
		}
	}

	for _, item := range req.Nav {
		label := strings.TrimSpace(item.Label)
		kind := strings.TrimSpace(item.Type)
		value := strings.TrimSpace(item.Value)

		if label == "" || value == "" {
			continue
		}

		if !slices.Contains([]string{"post", "url"}, kind) {
			return SiteConfig{}, errBadRequest("nav type must be post or url")
		}

		if kind == "url" {
			if _, err := url.ParseRequestURI(value); err != nil && !strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "?") {
				return SiteConfig{}, errBadRequest("nav url must be an absolute URL or site path")
			}
		}

		settings.Nav = append(settings.Nav, NavItemConfig{
			Label: label,
			Type:  kind,
			Value: value,
		})
	}

	return settings, nil
}

func normalizePost(id string, req upsertPostRequest) (AdminPost, error) {
	post := AdminPost{
		ID:          strings.TrimSpace(id),
		Slug:        slugify(strings.TrimSpace(req.Slug)),
		Title:       strings.TrimSpace(req.Title),
		Published:   req.Published,
		Excerpt:     strings.TrimSpace(req.Excerpt),
		Content:     strings.TrimSpace(req.Content),
		PublishedAt: strings.TrimSpace(req.PublishedAt),
	}

	if post.Title == "" {
		return AdminPost{}, errBadRequest("post title is required")
	}

	if post.Slug == "" {
		post.Slug = slugify(post.Title)
	}

	if post.Slug == "" {
		return AdminPost{}, errBadRequest("post slug is required")
	}

	if post.PublishedAt == "" {
		post.PublishedAt = time.Now().Format(time.DateOnly)
	}

	return post, nil
}

func (s *Service) requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if s.isAuthenticated(r) {
		return true
	}

	writeError(w, http.StatusUnauthorized, "authentication required")
	return false
}

func (s *Service) isAuthenticated(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}

	return s.auth.Valid(cookie.Value)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

type badRequestError struct {
	message string
}

func (e badRequestError) Error() string {
	return e.message
}

func errBadRequest(message string) error {
	return badRequestError{message: message}
}
