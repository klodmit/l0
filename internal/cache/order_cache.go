package cache

import (
	"sync"
	"time"

	"l0/internal/model"
)

type OrderCache interface {
	Get(id string) (model.Order, bool)
	Set(id string, o model.Order)
}

type ttlCache struct {
	data map[string]entry
	mu   sync.RWMutex
	ttl  time.Duration
}

type entry struct {
	o   model.Order
	exp time.Time
}

func NewOrderTTLCache(ttl time.Duration) OrderCache {
	return &ttlCache{
		data: make(map[string]entry),
		ttl:  ttl,
	}
}

func (c *ttlCache) Get(id string) (model.Order, bool) {
	c.mu.RLock()
	e, ok := c.data[id]
	c.mu.RUnlock()
	if !ok {
		return model.Order{}, false
	}
	if time.Now().After(e.exp) {
		// истёк — удалим и промах
		c.mu.Lock()
		delete(c.data, id)
		c.mu.Unlock()
		return model.Order{}, false
	}
	return e.o, true
}

func (c *ttlCache) Set(id string, o model.Order) {
	c.mu.Lock()
	c.data[id] = entry{o: o, exp: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}
