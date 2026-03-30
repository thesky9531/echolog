package blog

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type AuthManager struct {
	mu       sync.Mutex
	ttl      time.Duration
	sessions map[string]time.Time
}

func NewAuthManager(ttl time.Duration) *AuthManager {
	return &AuthManager{
		ttl:      ttl,
		sessions: map[string]time.Time{},
	}
}

func (a *AuthManager) CreateSession() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	token := randomToken()
	a.sessions[token] = time.Now().Add(a.ttl)
	return token
}

func (a *AuthManager) Valid(token string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	expiresAt, ok := a.sessions[token]
	if !ok {
		return false
	}

	if time.Now().After(expiresAt) {
		delete(a.sessions, token)
		return false
	}

	return true
}

func (a *AuthManager) DeleteSession(token string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.sessions, token)
}

func randomToken() string {
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return time.Now().Format("20060102150405.000000000")
	}

	return hex.EncodeToString(bytes[:])
}
