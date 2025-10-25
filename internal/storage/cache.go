// package storage

// // Types:
// type Cache struct {
//     data  map[string]interface{}
//     mutex sync.RWMutex
//     ttl   time.Duration
// }

// // Functions:
// - NewCache(ttl time.Duration) *Cache
// - Set(key string, value interface{})
// - Get(key string) (interface{}, bool)
// - Delete(key string)
// - Clear()

// // Implements:
// - Thread-safe caching
// - TTL-based expiration
// - LRU eviction (optional)
