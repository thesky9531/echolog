package blog

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

type storeData struct {
	AdminSecret string      `json:"adminSecret"`
	Site        SiteConfig  `json:"site"`
	Posts       []AdminPost `json:"posts"`
}

type Store struct {
	mu   sync.RWMutex
	path string
	data storeData
}

func NewStore(path string) (*Store, error) {
	store := &Store{path: path}
	if err := store.load(); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	bytes, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}

	var data storeData
	if err := json.Unmarshal(bytes, &data); err != nil {
		return err
	}

	if !strings.Contains(string(bytes), `"published"`) {
		for index := range data.Posts {
			data.Posts[index].Published = true
		}
	}

	if strings.TrimSpace(data.AdminSecret) == "" {
		return errors.New("adminSecret is required in local config")
	}

	if strings.TrimSpace(data.Site.Name) == "" {
		return errors.New("site.name is required in local config")
	}

	s.data = data
	return nil
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	payload, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}

	payload = append(payload, '\n')
	return os.WriteFile(s.path, payload, 0o644)
}

func (s *Store) AdminSecret() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.AdminSecret
}

func (s *Store) Settings() SiteConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneSiteConfig(s.data.Site)
}

func (s *Store) UpdateSettings(settings SiteConfig) (SiteConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.Site = cloneSiteConfig(settings)
	if err := s.saveLocked(); err != nil {
		return SiteConfig{}, err
	}

	return cloneSiteConfig(s.data.Site), nil
}

func (s *Store) PublicSite() SiteResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	response := SiteResponse{
		Name:        s.data.Site.Name,
		Description: s.data.Site.Description,
		ICPNumber:   s.data.Site.ICPNumber,
		ICPLinkURL:  s.data.Site.ICPLinkURL,
	}

	for _, item := range s.data.Site.Nav {
		href := resolveNavHref(item, s.data.Posts)
		if href == "" {
			continue
		}

		response.Nav = append(response.Nav, NavItem{
			Label: item.Label,
			Href:  href,
		})
	}

	return response
}

func (s *Store) PublicPosts() []PublicPost {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]PublicPost, 0, len(s.data.Posts))
	for _, post := range s.sortedPostsLocked() {
		if !post.Published {
			continue
		}

		items = append(items, PublicPost{
			ID:          post.ID,
			Slug:        post.Slug,
			Title:       post.Title,
			Published:   post.Published,
			PublishedAt: post.PublishedAt,
			UpdatedAt:   post.UpdatedAt,
			Excerpt:     post.Excerpt,
		})
	}

	return items
}

func (s *Store) PublicPost(slug string) (PublicPost, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, post := range s.data.Posts {
		if post.Slug != slug || !post.Published {
			continue
		}

		return PublicPost{
			ID:          post.ID,
			Slug:        post.Slug,
			Title:       post.Title,
			Published:   post.Published,
			PublishedAt: post.PublishedAt,
			UpdatedAt:   post.UpdatedAt,
			Excerpt:     post.Excerpt,
			Content:     post.Content,
		}, true
	}

	return PublicPost{}, false
}

func (s *Store) AdminPosts() []AdminPost {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]AdminPost(nil), s.sortedPostsLocked()...)
}

func (s *Store) UpsertPost(post AdminPost) (AdminPost, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Format(time.RFC3339)
	post.UpdatedAt = now

	if post.ID == "" {
		post.ID = randomID()
	}

	duplicateSlug := slices.ContainsFunc(s.data.Posts, func(item AdminPost) bool {
		return item.Slug == post.Slug && item.ID != post.ID
	})
	if duplicateSlug {
		return AdminPost{}, errBadRequest("post slug must be unique")
	}

	replaced := false
	for index, item := range s.data.Posts {
		if item.ID != post.ID {
			continue
		}

		if post.PublishedAt == "" {
			post.PublishedAt = item.PublishedAt
		}
		if post.Excerpt == "" {
			post.Excerpt = item.Excerpt
		}

		s.data.Posts[index] = post
		replaced = true
		break
	}

	if !replaced {
		if post.PublishedAt == "" {
			post.PublishedAt = time.Now().Format(time.DateOnly)
		}
		s.data.Posts = append(s.data.Posts, post)
	}

	if err := s.saveLocked(); err != nil {
		return AdminPost{}, err
	}

	return post, nil
}

func (s *Store) DeletePost(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	index := slices.IndexFunc(s.data.Posts, func(item AdminPost) bool {
		return item.ID == id
	})
	if index == -1 {
		return errBadRequest("post not found")
	}

	s.data.Posts = append(s.data.Posts[:index], s.data.Posts[index+1:]...)
	if err := s.saveLocked(); err != nil {
		return err
	}

	return nil
}

func (s *Store) sortedPostsLocked() []AdminPost {
	items := append([]AdminPost(nil), s.data.Posts...)
	slices.SortFunc(items, func(a, b AdminPost) int {
		return strings.Compare(b.PublishedAt, a.PublishedAt)
	})
	return items
}

func resolveNavHref(item NavItemConfig, posts []AdminPost) string {
	switch item.Type {
	case "post":
		for _, post := range posts {
			if (post.Slug == item.Value || post.ID == item.Value) && post.Published {
				return "/?post=" + post.Slug
			}
		}
		return ""
	case "url":
		return item.Value
	default:
		return ""
	}
}

func cloneSiteConfig(site SiteConfig) SiteConfig {
	clone := site
	clone.Nav = append([]NavItemConfig(nil), site.Nav...)
	return clone
}

func randomID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return time.Now().Format("20060102150405")
	}

	return hex.EncodeToString(bytes[:])
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false

	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || r == ' ':
			if builder.Len() > 0 && !lastDash {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}

	return strings.Trim(builder.String(), "-")
}
