// Package session 会话密钥管理（登录成功后 SM4 会话密钥与令牌绑定，内存存储）
package session

import (
	"sync"
	"time"
)

type entry struct {
	key      []byte
	userID   uint
	username string
	expireAt time.Time
}

// Manager 会话管理器
type Manager struct {
	mu      sync.RWMutex
	entries map[string]*entry
	ttl     time.Duration
}

// NewManager 创建会话管理器
func NewManager(ttl time.Duration) *Manager {
	m := &Manager{entries: make(map[string]*entry), ttl: ttl}
	go m.gc()
	return m
}

// Set 绑定会话密钥
func (m *Manager) Set(token string, key []byte, userID uint, username string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[token] = &entry{key: key, userID: userID, username: username, expireAt: time.Now().Add(m.ttl)}
}

// Get 获取会话密钥
func (m *Manager) Get(token string) ([]byte, uint, string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.entries[token]
	if !ok {
		return nil, 0, "", false
	}
	if time.Now().After(e.expireAt) {
		return nil, 0, "", false
	}
	return e.key, e.userID, e.username, true
}

// Delete 删除会话
func (m *Manager) Delete(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, token)
}

// KickUser 踢掉指定用户的所有旧会话（并发登录互踢）
func (m *Manager) KickUser(username string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, e := range m.entries {
		if e.username == username {
			delete(m.entries, k)
		}
	}
}

// Count 会话数量
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entries)
}

func (m *Manager) gc() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for k, e := range m.entries {
			if now.After(e.expireAt) {
				delete(m.entries, k)
			}
		}
		m.mu.Unlock()
	}
}
