package cache

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/joshmedeski/sesh/v2/model"
)

func init() {
	gob.Register(model.SeshSessions{})
	gob.Register(model.SeshSessionMap{})
	gob.Register(model.SeshSession{})
}

// CachedData holds cached sessions along with the time they were written.
type CachedData struct {
	Sessions  model.SeshSessions
	Timestamp time.Time
}

// Cache reads and writes session data to a persistent store.
type Cache interface {
	Read() (CachedData, error)
	Write(sessions model.SeshSessions) error
}

// FileCache implements Cache using a gob-encoded file under the XDG cache directory.
type FileCache struct {
	path string
}

// NewFileCache stores slow sources separately from older whole-list snapshots.
func NewFileCache() *FileCache {
	return &FileCache{path: filepath.Join(Dir(), "sessions.v2.gob")}
}

// NewFileCacheWithPath creates a FileCache at a specific path (useful for testing).
func NewFileCacheWithPath(path string) *FileCache {
	return &FileCache{path: path}
}

func (c *FileCache) Read() (CachedData, error) {
	data, err := os.ReadFile(c.path)
	if err != nil {
		return CachedData{}, fmt.Errorf("cache read: %w", err)
	}
	var cached CachedData
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&cached); err != nil {
		return CachedData{}, fmt.Errorf("cache decode: %w", err)
	}
	return cached, nil
}

func (c *FileCache) Write(sessions model.SeshSessions) error {
	var buf bytes.Buffer
	cached := CachedData{Sessions: sessions, Timestamp: time.Now()}
	if err := gob.NewEncoder(&buf).Encode(cached); err != nil {
		return fmt.Errorf("cache encode: %w", err)
	}
	return writeAtomic(c.path, buf.Bytes())
}
