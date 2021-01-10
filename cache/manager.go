package cache

import (
	"github.com/SotirisAlfonsos/chaos-bot/proto"
	"github.com/go-kit/kit/log"
	"github.com/patrickmn/go-cache"
)

type Manager struct {
	cache  *cache.Cache
	logger log.Logger
}

func NewCacheManager(logger log.Logger) *Manager {
	return &Manager{
		cache:  cache.New(0, 0),
		logger: logger,
	}
}

func (m *Manager) Register(key string, function func() (*proto.StatusResponse, error)) error {
	if _, ok := m.cache.Get(key); ok {
		return m.cache.Replace(key, function, 0)
	}
	return m.cache.Add(key, function, 0)
}

func (m *Manager) GetAll() map[string]cache.Item {
	return m.cache.Items()
}

func (m *Manager) Get(key string) func() (*proto.StatusResponse, error) {
	if f, ok := m.cache.Get(key); ok {
		return f.(func() (*proto.StatusResponse, error))
	}
	return nil
}

func (m *Manager) Delete(key string) {
	m.cache.Delete(key)
}

func (m *Manager) ItemCount() int {
	return m.cache.ItemCount()
}
