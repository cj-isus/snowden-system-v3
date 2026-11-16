package quic

import (
	"strings"
	"sync"

	list "github.com/sagernet/quic-go/internal/utils/linkedlist"
)

const (
	maxTokenStoreOrigins = 64
	maxTokensPerOrigin   = 16
	maxTokenStoreBytes   = 1 << 20
)

type cachedClientToken struct {
	token *ClientToken
	bytes int // immutable referenced-view credit; never inspect payload in cleanup
}

type singleOriginTokenStore struct {
	tokens []cachedClientToken
	len    int
	p      int
	bytes  int
}

func newSingleOriginTokenStore(size int) *singleOriginTokenStore {
	if size <= 0 {
		return &singleOriginTokenStore{}
	}
	return &singleOriginTokenStore{tokens: make([]cachedClientToken, min(size, maxTokensPerOrigin))}
}

func (s *singleOriginTokenStore) Add(token *ClientToken) {
	if len(s.tokens) == 0 || token == nil || len(token.data) == 0 || cap(token.data) > maxClientTokenSize {
		return
	}
	s.bytes += cap(token.data) - s.tokens[s.p].bytes
	s.tokens[s.p] = cachedClientToken{token: token, bytes: cap(token.data)}
	s.p = s.index(s.p + 1)
	s.len = min(s.len+1, len(s.tokens))
}

func (s *singleOriginTokenStore) Pop() *ClientToken {
	if s.len == 0 {
		return nil
	}
	s.p = s.index(s.p - 1)
	entry := s.tokens[s.p]
	s.tokens[s.p] = cachedClientToken{}
	s.bytes -= entry.bytes
	s.len--
	return entry.token
}

func (s *singleOriginTokenStore) Len() int {
	return s.len
}

func (s *singleOriginTokenStore) index(i int) int {
	mod := len(s.tokens)
	return (i + mod) % mod
}

func (s *singleOriginTokenStore) metadataReady() bool {
	if s == nil || len(s.tokens) == 0 || len(s.tokens) > maxTokensPerOrigin ||
		cap(s.tokens) != len(s.tokens) || s.p < 0 || s.p >= len(s.tokens) ||
		s.len < 0 || s.len > len(s.tokens) || s.bytes < 0 {
		return false
	}
	count, bytes := 0, 0
	for _, entry := range s.tokens {
		if entry.token == nil {
			if entry.bytes != 0 {
				return false
			}
			continue
		}
		if entry.bytes <= 0 || entry.bytes > maxClientTokenSize {
			return false
		}
		count++
		bytes += entry.bytes
	}
	return count == s.len && bytes == s.bytes
}

type lruTokenStoreEntry struct {
	key   string
	cache *singleOriginTokenStore
}

type lruTokenStore struct {
	mutex sync.Mutex

	m                map[string]*list.Element[*lruTokenStoreEntry]
	q                *list.List[*lruTokenStoreEntry]
	capacity         int
	singleOriginSize int
	bytes            int // referenced token views + bounded owned key bytes
	failed           bool
	operation        lruTokenOperation
}

type lruTokenOperation struct {
	active bool
	key    string
	input  *ClientToken
	entry  *lruTokenStoreEntry
}

var _ TokenStore = &lruTokenStore{}

// NewLRUTokenStore creates a new LRU cache for tokens received by the client.
// maxOrigins specifies how many origins this cache is saving tokens for.
// tokensPerOrigin specifies the maximum number of tokens per origin.
// Nonpositive sizes disable caching. Sizes are clamped to 64 origins and 16
// tokens per origin, with 1 MiB of referenced token/key bytes per store.
// Tokens over 16 KiB and keys over 1024 bytes are ignored. No global/RSS bound.
func NewLRUTokenStore(maxOrigins, tokensPerOrigin int) TokenStore {
	if maxOrigins <= 0 || tokensPerOrigin <= 0 {
		return &lruTokenStore{}
	}
	return &lruTokenStore{
		m:                make(map[string]*list.Element[*lruTokenStoreEntry]),
		q:                list.New[*lruTokenStoreEntry](),
		capacity:         min(maxOrigins, maxTokenStoreOrigins),
		singleOriginSize: min(tokensPerOrigin, maxTokensPerOrigin),
	}
}

func (s *lruTokenStore) readyLocked() bool {
	return !s.failed && !s.operation.active && s.operation.key == "" &&
		s.operation.input == nil && s.operation.entry == nil &&
		s.capacity > 0 && s.capacity <= maxTokenStoreOrigins &&
		s.singleOriginSize > 0 && s.singleOriginSize <= maxTokensPerOrigin &&
		s.m != nil && s.q != nil && s.q.Len() == len(s.m) && s.q.Len() <= s.capacity &&
		s.bytes >= 0 && s.bytes <= maxTokenStoreBytes && (s.q.Len() != 0 || s.bytes == 0)
}

func (s *lruTokenStore) guardLocked(returned *bool) {
	if !*returned {
		s.failed = true // keep original operation; optional cache is not retried
	}
}

func (s *lruTokenStore) evictLocked(el *list.Element[*lruTokenStoreEntry]) bool {
	if el == nil || el.Value == nil {
		return false
	}
	entry := el.Value
	s.operation.entry = entry
	if !entry.cache.metadataReady() || len(entry.key) == 0 || len(entry.key) > s.bytes ||
		entry.cache.bytes > s.bytes-len(entry.key) {
		return false
	}
	s.bytes -= len(entry.key) + entry.cache.bytes
	delete(s.m, entry.key)
	s.q.Remove(el)
	el.Value = nil
	clear(entry.cache.tokens) // metadata references only; never overwrite token bytes
	entry.cache.tokens = nil
	entry.cache = nil
	entry.key = ""
	return true
}

func (s *lruTokenStore) Put(key string, token *ClientToken) {
	if token == nil || len(token.data) == 0 || cap(token.data) > maxClientTokenSize ||
		len(key) == 0 || len(key) > maxTokenStoreKeyBytes {
		return
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.capacity == 0 || s.failed {
		return
	}
	if !s.readyLocked() {
		s.failed = true
		return
	}
	s.operation = lruTokenOperation{active: true, key: key, input: token}
	returned := false
	defer s.guardLocked(&returned)
	// Each unsuccessful iteration removes one origin; at most 64 evictions.
	for attempts := 0; attempts <= s.capacity; attempts++ {
		el := s.m[key]
		required := cap(token.data)
		if el != nil {
			if el.Value == nil || !el.Value.cache.metadataReady() {
				return
			}
			required -= el.Value.cache.tokens[el.Value.cache.p].bytes
		} else {
			required += len(key)
		}
		if required < -s.bytes {
			return
		}
		if required <= maxTokenStoreBytes-s.bytes && (el != nil || s.q.Len() < s.capacity) {
			s.bytes += required // reserve payload/key credit before clone/metadata
			if el == nil {
				s.operation.entry = &lruTokenStoreEntry{}
				entry := s.operation.entry
				entry.key = strings.Clone(key)
				entry.cache = newSingleOriginTokenStore(s.singleOriginSize)
				entry.cache.Add(token)
				s.m[entry.key] = s.q.PushFront(entry)
			} else {
				s.operation.entry = el.Value
				el.Value.cache.Add(token)
				s.q.MoveToFront(el)
			}
			s.operation = lruTokenOperation{}
			returned = true
			return
		}
		if !s.evictLocked(s.q.Back()) {
			return
		}
	}
}

func (s *lruTokenStore) Pop(key string) *ClientToken {
	if len(key) == 0 || len(key) > maxTokenStoreKeyBytes {
		return nil
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.capacity == 0 || s.failed {
		return nil
	}
	if !s.readyLocked() {
		s.failed = true
		return nil
	}
	s.operation = lruTokenOperation{active: true, key: key}
	returned := false
	defer s.guardLocked(&returned)
	if el, ok := s.m[key]; ok {
		s.operation.entry = el.Value
		if el.Value == nil || !el.Value.cache.metadataReady() {
			return nil
		}
		cache := el.Value.cache
		credit := 0
		if cache.Len() != 0 {
			entry := cache.tokens[cache.index(cache.p-1)]
			s.operation.input, credit = entry.token, entry.bytes
			if entry.token == nil || credit <= 0 || credit > s.bytes {
				return nil
			}
		}
		token := cache.Pop()
		s.bytes -= credit
		if cache.Len() == 0 {
			if !s.evictLocked(el) {
				return nil
			}
		} else {
			s.q.MoveToFront(el)
		}
		s.operation = lruTokenOperation{}
		returned = true
		return token
	}
	s.operation = lruTokenOperation{}
	returned = true
	return nil
}
