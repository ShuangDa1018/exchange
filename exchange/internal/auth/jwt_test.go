package auth_test

import (
	"context"
	"sync"
	"testing"

	"github.com/demo/exchange/internal/auth"
	"github.com/demo/exchange/internal/store"
)

// memUsers：测试用内存用户表
type memUsers struct {
	mu   sync.Mutex
	hash map[string]string
}

func newMemUsers() *memUsers {
	return &memUsers{hash: make(map[string]string)}
}

func (m *memUsers) CreateUser(_ context.Context, userID, passwordHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.hash[userID]; ok {
		return store.ErrUserExists
	}
	m.hash[userID] = passwordHash
	return nil
}

func (m *memUsers) GetUserPassword(_ context.Context, userID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.hash[userID]
	if !ok {
		return "", store.ErrUserNotFound
	}
	return h, nil
}

func (m *memUsers) EnsureUser(_ context.Context, userID, passwordHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.hash[userID]; !ok {
		m.hash[userID] = passwordHash
	}
	return nil
}

func TestLoginAndParse(t *testing.T) {
	users := newMemUsers()
	s := auth.New("test-secret", 1, users)
	if err := s.SeedDemoUsers(context.Background()); err != nil {
		t.Fatal(err)
	}
	tok, err := s.Login("alice", "alice")
	if err != nil {
		t.Fatal(err)
	}
	uid, err := s.Parse(tok)
	if err != nil || uid != "alice" {
		t.Fatalf("parse=%s err=%v", uid, err)
	}
	if _, err := s.Login("alice", "wrong"); err == nil {
		t.Fatal("expected bad password error")
	}
}

func TestRegisterAndLogin(t *testing.T) {
	s := auth.New("test-secret", 1, newMemUsers())
	tok, err := s.Register("carol", "secret")
	if err != nil {
		t.Fatal(err)
	}
	uid, err := s.Parse(tok)
	if err != nil || uid != "carol" {
		t.Fatalf("parse=%s err=%v", uid, err)
	}
	if _, err := s.Register("carol", "other"); err == nil {
		t.Fatal("expected duplicate user error")
	}
	tok2, err := s.Login("carol", "secret")
	if err != nil {
		t.Fatal(err)
	}
	uid, err = s.Parse(tok2)
	if err != nil || uid != "carol" {
		t.Fatalf("login parse=%s err=%v", uid, err)
	}
}
