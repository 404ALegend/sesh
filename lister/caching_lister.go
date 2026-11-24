package lister

import (
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/joshmedeski/sesh/v2/cache"
	"github.com/joshmedeski/sesh/v2/model"
)

const softTTL = 5 * time.Second

// Mux sessions and panes must stay live; config comes from the current config.
var cachedSources = []string{"tmuxinator", "zoxide"}

// CachingLister serves slow sources from cache and delegates assembly to inner.
type CachingLister struct {
	inner SourceLister
	cache cache.Cache
	wg    sync.WaitGroup
}

// NewCachingLister creates a CachingLister that decorates inner with file-based caching.
func NewCachingLister(inner SourceLister, c cache.Cache) *CachingLister {
	return &CachingLister{inner: inner, cache: c}
}

// List uses stale-while-revalidate caching only when a cached source is requested.
func (cl *CachingLister) List(opts ListOptions) (model.SeshSessions, error) {
	requested := srcs(opts)
	if !slices.ContainsFunc(cachedSources, func(src string) bool { return slices.Contains(requested, src) }) {
		return cl.inner.ListWith(opts, nil)
	}

	cached, err := cl.cache.Read()
	if err == nil {
		age := time.Since(cached.Timestamp)
		if age < softTTL {
			slog.Debug("cache: hit (fresh)", "age", age)
		} else {
			slog.Debug("cache: hit (stale, revalidating)", "age", age)
			cl.RefreshCache()
		}
		return cl.inner.ListWith(opts, splitBySource(cached.Sessions))
	}

	// Cold start -- fetch synchronously
	slog.Debug("cache: miss (cold start)")
	fetched, err := cl.inner.ListSources(cachedSources)
	if err != nil {
		for _, src := range cachedSources {
			if !slices.Contains(requested, src) {
				// A source this list doesn't need may have failed. Only errors
				// from the requested sources should prevent this list.
				return cl.inner.ListWith(opts, nil)
			}
		}
		return model.SeshSessions{}, err
	}
	if err := cl.cache.Write(flatten(fetched)); err != nil {
		slog.Warn("cache: write failed on cold start", "error", err)
	}
	return cl.inner.ListWith(opts, fetched)
}

// flatten stores only raw slow-source rows, preserving each source's order.
func flatten(fetched SourceResults) model.SeshSessions {
	size := 0
	for _, src := range cachedSources {
		size += len(fetched[src].OrderedIndex)
	}
	flat := model.SeshSessions{
		Directory:    make(model.SeshSessionMap, size),
		OrderedIndex: make([]string, 0, size),
	}
	for _, src := range cachedSources {
		sessions := fetched[src]
		flat.OrderedIndex = append(flat.OrderedIndex, sessions.OrderedIndex...)
		for _, key := range sessions.OrderedIndex {
			flat.Directory[key] = sessions.Directory[key]
		}
	}
	return flat
}

// splitBySource marks even empty cached sources as already fetched. Rows for
// other sources cannot override live data.
func splitBySource(stored model.SeshSessions) SourceResults {
	results := make(SourceResults, len(cachedSources))
	for _, src := range cachedSources {
		results[src] = model.SeshSessions{
			Directory:    make(model.SeshSessionMap),
			OrderedIndex: []string{},
		}
	}
	for _, key := range stored.OrderedIndex {
		session := stored.Directory[key]
		if result, ok := results[session.Src]; ok {
			result.Directory[key] = session
			result.OrderedIndex = append(result.OrderedIndex, key)
			results[session.Src] = result
		}
	}
	return results
}

func (cl *CachingLister) revalidate() {
	fetched, err := cl.inner.ListSources(cachedSources)
	if err != nil {
		slog.Warn("cache: background revalidation fetch failed", "error", err)
		return
	}
	if err := cl.cache.Write(flatten(fetched)); err != nil {
		slog.Warn("cache: background revalidation write failed", "error", err)
	}
}

// RefreshCache refreshes the cached sources in the background, without a cache read.
func (cl *CachingLister) RefreshCache() {
	cl.wg.Add(1)
	go func() {
		defer cl.wg.Done()
		cl.revalidate()
	}()
}

// Wait blocks until all background refresh goroutines have completed.
// Call this before process exit to avoid truncated cache writes.
func (cl *CachingLister) Wait() {
	cl.wg.Wait()
}

// --- Delegate all other Lister methods to inner ---

func (cl *CachingLister) ListTmuxPanes() (model.SeshSessions, error) {
	return cl.inner.ListTmuxPanes()
}

func (cl *CachingLister) FindTmuxSession(name string) (model.SeshSession, bool) {
	return cl.inner.FindTmuxSession(name)
}

func (cl *CachingLister) FindTmuxSessionByBase(base string) (model.SeshSession, bool) {
	return cl.inner.FindTmuxSessionByBase(base)
}

func (cl *CachingLister) GetAttachedTmuxSession() (model.SeshSession, bool) {
	return cl.inner.GetAttachedTmuxSession()
}

func (cl *CachingLister) GetLastTmuxSession() (model.SeshSession, bool) {
	return cl.inner.GetLastTmuxSession()
}

func (cl *CachingLister) FindConfigSession(name string) (model.SeshSession, bool) {
	return cl.inner.FindConfigSession(name)
}

func (cl *CachingLister) FindConfigWildcard(path string) (model.WildcardConfig, bool) {
	return cl.inner.FindConfigWildcard(path)
}

func (cl *CachingLister) FindZoxideSession(name string) (model.SeshSession, bool) {
	return cl.inner.FindZoxideSession(name)
}

func (cl *CachingLister) FindTmuxinatorConfig(name string) (model.SeshSession, bool) {
	return cl.inner.FindTmuxinatorConfig(name)
}
