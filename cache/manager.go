package cache

import (
	"fmt"
	"strings"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/go-kit/kit/log"
	"github.com/patrickmn/go-cache"
)

type Manager struct {
	cache  *cache.Cache
	logger log.Logger
}

type Key struct {
	Job    string
	Target string
}

func (k *Key) createUniqueKey() string {
	return fmt.Sprintf("%s,%s", k.Job, k.Target)
}

type Items map[Key]cache.Item

func NewCacheManager(logger log.Logger) *Manager {
	return &Manager{
		cache:  cache.New(0, 0),
		logger: logger,
	}
}

func (m *Manager) Register(key *Key, function func() (*v1.StatusResponse, error)) error {
	uniqueKeyName := key.createUniqueKey()
	if _, ok := m.cache.Get(uniqueKeyName); ok {
		return m.cache.Replace(uniqueKeyName, function, 0)
	}
	return m.cache.Add(uniqueKeyName, function, 0)
}

// GetAll items from the cache
func (m *Manager) GetAll() Items {
	cacheItems := m.cache.Items()
	items := make(map[Key]cache.Item, len(cacheItems))

	for keyString, val := range cacheItems {
		keySplit := strings.Split(keyString, ",")
		key := &Key{
			Job:    keySplit[0],
			Target: keySplit[1],
		}
		items[*key] = val
	}

	return items
}

// GetAll single item from the cache
func (m *Manager) Get(key *Key) func() (*v1.StatusResponse, error) {
	uniqueKeyName := key.createUniqueKey()
	if f, ok := m.cache.Get(uniqueKeyName); ok {
		return f.(func() (*v1.StatusResponse, error))
	}
	return nil
}

func (m *Manager) Delete(key *Key) {
	uniqueKeyName := key.createUniqueKey()
	m.cache.Delete(uniqueKeyName)
}

func (m *Manager) ItemCount() int {
	return m.cache.ItemCount()
}
