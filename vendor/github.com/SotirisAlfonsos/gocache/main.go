package gocache

import (
	"sync"
	"time"
)

const (
	noExpiration time.Duration = 0
)

// Cache is the instance of the cache. Use New to create
type Cache struct {
	*cache
}

// New creates a new cache instance with expiration.
// For cache with no expiration policy provide a value of 0 or less
func New(expiration time.Duration) *Cache {
	var items []*Item

	if expiration <= 0 {
		expiration = noExpiration
	}

	c := &cache{
		items:      items,
		expiration: expiration,
	}

	return &Cache{
		c,
	}
}

// Key is the interface that is used as the key for the cache Items
type Key interface {
	Equals(key Key) bool
}

type cache struct {
	items      []*Item
	expiration time.Duration
	mu         sync.RWMutex //nolint:structcheck
}

// Item is the key value pair together with the expiration (if applicable)
// Both key and value are interfaces
type Item struct {
	Key      Key
	Value    interface{}
	expireAt int64
}

// Set will add the key value in the cache.
// If the key already exists based on Equals, then the value and expiration will be updated.
// If the key already exists but is expired. it will be removed from the cache and the key value pair will be added as a new Item
// If the key does not exist a new Item will be added in the cache
func (c *cache) Set(key Key, val interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.getItem(key)
	if !ok {
		item = c.newItem(key, val)
		c.items = append(c.items, item)
	} else {
		item.update(val, c.expiration)
	}
}

// Get will return the key value from the cache based on Equals together with
// boolean which will be true if the key was found in cache and false otherwise.
// If the key exists based on Equals, then the *Item will be returned together with status true.
// If the key exists but is expired. it will be removed from the cache and the method will return nil false
func (c *cache) Get(key Key) (*Item, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.getItem(key)
	if !ok {
		return nil, false
	}

	return item, true
}

// ItemCount will return the count of the items in the list.
// Expired items will be included in the count
func (c *cache) ItemCount() int {
	return len(c.items)
}

// GetAll will return all items in the cache.
// If expiration on items is set GetAll will lazily remove all expired items and return the rest
func (c *cache) GetAll() []Item {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.evict()

	items := make([]Item, 0, len(c.items))

	for _, item := range c.items {
		items = append(items, *item)
	}

	return items
}

// Delete will remove the item with the provided key from the cache.
func (c *cache) Delete(key Key) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, item := range c.items {
		if key.Equals(item.Key) {
			c.deleteIndex(i)
		}
	}
}

// DeleteAll deletes all items from the cache.
func (c *cache) DeleteAll() {
	c.mu.Lock()
	c.items = make([]*Item, 0)
	c.mu.Unlock()
}

// Evict will remove all expired items from the cache.
func (c *cache) Evict() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.evict()
}

func (c *cache) evict() {
	newItems := make([]*Item, 0)
	for _, item := range c.items {
		if c.expiration == noExpiration {
			newItems = append(newItems, item)
		} else if time.Now().UnixNano() < item.expireAt {
			newItems = append(newItems, item)
		}
	}
	c.items = newItems
}

func (c *cache) newItem(key Key, val interface{}) *Item {
	if c.expiration > 0 {
		expiresAt := time.Now().UnixNano() + c.expiration.Nanoseconds()

		return &Item{
			Key:      key,
			Value:    val,
			expireAt: expiresAt,
		}
	}

	return &Item{
		Key:   key,
		Value: val,
	}
}

func (c *cache) getItem(key Key) (*Item, bool) {
	item, i, ok := func() (*Item, int, bool) {
		for i, item := range c.items {
			if item.Key.Equals(key) {
				return item, i, true
			}
		}

		return nil, 0, false
	}()
	if !ok {
		return nil, false
	}

	switch {
	case c.expiration == noExpiration:
		return item, true
	case time.Now().UnixNano() > item.expireAt:
		c.deleteIndex(i)
		return nil, false
	}

	return item, true
}

func (c *cache) deleteIndex(i int) {
	c.items[i] = c.items[len(c.items)-1]
	c.items = c.items[:len(c.items)-1]
}

func (item *Item) update(val interface{}, expiration time.Duration) {
	item.Value = val
	if expiration > 0 {
		item.expireAt = time.Now().UnixNano() + expiration.Nanoseconds()
	}
}
