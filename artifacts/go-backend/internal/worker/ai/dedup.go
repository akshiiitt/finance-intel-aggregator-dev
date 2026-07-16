// dedup.go — Jaccard-similarity title deduplication for the AI worker.
//
// When the same story is covered by multiple news outlets, instead of
// creating a separate processed_items row for each, we:
//  1. Detect near-duplicate titles using Jaccard similarity at threshold 0.72
//  2. Increment coverage_count on the existing record
//  3. Append the new source to also_sources
//  4. Mark the raw_item as processed without creating a new processed_item
//
// Matches the Node.js implementation exactly:
//   - LRU cache of 2 000 recent processed titles (stdlib container/list — no extra deps)
//   - Stop-word filtering before tokenisation
//   - Jaccard coefficient on word-set overlap
//   - Seeded from the last 1 500 DB records at startup

package ai

import (
	"container/list"
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

const (
	titleCacheSize   = 2000 // max entries in the LRU
	jaccardThreshold = 0.72 // near-duplicate threshold (matches Node.js at 0.72)
	seedLimit        = 1500 // records to seed from DB on startup
	minWords         = 4    // don't dedup titles shorter than this
)

// titleEntry stores one cached title in the LRU.
type titleEntry struct {
	processedID int64
	source      string
	url         string
	words       map[string]struct{} // normalised significant words
	key         string              // fingerprint key used in the map
}

// titleLRU is a thread-safe, fixed-capacity LRU cache mapping fingerprint
// strings to titleEntry values.  It is implemented with a hash map +
// doubly-linked list so both Get and Add are O(1) average.
// We roll our own to avoid the hashicorp/golang-lru/v2 dependency which
// is not available in the current module.
type titleLRU struct {
	mu       sync.Mutex
	cap      int
	ll       *list.List
	items    map[string]*list.Element
}

func newTitleLRU(capacity int) *titleLRU {
	return &titleLRU{
		cap:   capacity,
		ll:    list.New(),
		items: make(map[string]*list.Element, capacity),
	}
}

// Add inserts or updates a key; evicts the oldest entry if at capacity.
func (c *titleLRU) Add(key string, entry titleEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.ll.MoveToFront(el)
		el.Value.(*titleEntry).processedID = entry.processedID
		el.Value.(*titleEntry).source = entry.source
		el.Value.(*titleEntry).url = entry.url
		el.Value.(*titleEntry).words = entry.words
		return
	}
	if c.ll.Len() >= c.cap {
		oldest := c.ll.Back()
		if oldest != nil {
			c.ll.Remove(oldest)
			delete(c.items, oldest.Value.(*titleEntry).key)
		}
	}
	e := &titleEntry{
		processedID: entry.processedID,
		source:      entry.source,
		url:         entry.url,
		words:       entry.words,
		key:         key,
	}
	el := c.ll.PushFront(e)
	c.items[key] = el
}

// FindDuplicate searches the cache for a duplicate title within the lock,
// avoiding the O(N) heap allocation of extracting all entries.
func (c *titleLRU) FindDuplicate(url string, words map[string]struct{}, minWords int, threshold float64) (int64, string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for el := c.ll.Front(); el != nil; el = el.Next() {
		e := el.Value.(*titleEntry)
		if url != "" && e.url == url {
			return e.processedID, e.source, true
		}
		if len(words) >= minWords && jaccard(words, e.words) >= threshold {
			return e.processedID, e.source, true
		}
	}
	return 0, "", false
}

// _titleLRU is the process-wide LRU cache.
var (
	_titleLRU  *titleLRU
	_titleOnce sync.Once
	_titleMu   sync.RWMutex
)

func getCache() *titleLRU {
	_titleOnce.Do(func() {
		_titleLRU = newTitleLRU(titleCacheSize)
	})
	return _titleLRU
}

// InitTitleCache seeds the in-memory LRU from the last seedLimit processed_items.
// Called once at worker startup so recent dedup history survives restarts.
func InitTitleCache(ctx context.Context, pool *pgxpool.Pool) {
	cache := getCache()

	rows, err := pool.Query(ctx, `
		SELECT id, title, COALESCE(source, '') as source, source_url
		FROM processed_items
		ORDER BY fetched_at DESC
		LIMIT $1
	`, seedLimit)
	if err != nil {
		log.Warn().Err(err).Msg("dedup: failed to seed title cache from DB")
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int64
		var title, source, url string
		if err := rows.Scan(&id, &title, &source, &url); err != nil {
			continue
		}
		addToCache(cache, title, url, id, source)
		count++
	}
	log.Info().Int("seeded", count).Msg("dedup: title cache seeded from DB")
}

// CheckDuplicate returns the existing processed_items.id and source if the title
// is a near-duplicate of a cached entry or if the URL matches exactly. Returns found=false otherwise.
func CheckDuplicate(title, url string) (existingID int64, existingSource string, found bool) {
	words := titleWords(title)
	cache := getCache()
	return cache.FindDuplicate(url, words, minWords, jaccardThreshold)
}

// RegisterTitle adds a newly inserted processed_item to the dedup cache.
func RegisterTitle(title, url string, processedID int64, source string) {
	addToCache(getCache(), title, url, processedID, source)
}

// addToCache normalises the title and inserts it into the provided LRU.
func addToCache(cache *titleLRU, title, url string, id int64, source string) {
	words := titleWords(title)
	fp := fingerprint(words)
	cache.Add(fp, titleEntry{
		processedID: id,
		source:      source,
		url:         url,
		words:       words,
		key:         fp,
	})
}

var stopWords = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "and": {}, "or": {}, "but": {},
	"in": {}, "on": {}, "at": {}, "to": {}, "for": {}, "of": {},
	"is": {}, "are": {}, "was": {}, "were": {}, "be": {}, "been": {},
	"has": {}, "have": {}, "had": {}, "it": {}, "its": {}, "this": {},
	"that": {}, "with": {}, "from": {}, "by": {}, "as": {}, "into": {},
	"he": {}, "she": {}, "we": {}, "they": {}, "his": {}, "her": {},
	"our": {}, "their": {}, "not": {}, "no": {}, "new": {},
}

// titleWords tokenises a title into significant words, stripping stop-words.
// Matches the Node.js titleWords() function exactly.
func titleWords(title string) map[string]struct{} {


	words := make(map[string]struct{})
	raw := strings.FieldsFunc(strings.ToLower(title), func(r rune) bool {
		return !('a' <= r && r <= 'z') && !('0' <= r && r <= '9')
	})
	for _, w := range raw {
		if len(w) > 2 {
			if _, isStop := stopWords[w]; !isStop {
				words[w] = struct{}{}
			}
		}
	}
	return words
}

// jaccard computes the Jaccard similarity coefficient of two word sets.
func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	intersection := 0
	for w := range a {
		if _, ok := b[w]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// fingerprint creates a deterministic string key from the word set,
// used as the LRU cache key so exact-match lookups are O(1).
func fingerprint(words map[string]struct{}) string {
	sorted := make([]string, 0, len(words))
	for w := range words {
		sorted = append(sorted, w)
	}
	sort.Strings(sorted)
	return strings.Join(sorted, " ")
}
