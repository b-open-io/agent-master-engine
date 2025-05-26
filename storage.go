package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FileStorage implements Storage interface using filesystem
type FileStorage struct {
	basePath string
	mu       sync.RWMutex
	watchers map[string][]func([]byte)
	stopChan chan struct{}
}

// NewFileStorage creates a new file-based storage
func NewFileStorage(basePath string) (*FileStorage, error) {
	// Expand home directory
	if strings.HasPrefix(basePath, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		basePath = filepath.Join(home, basePath[1:])
	}

	// Ensure base directory exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &FileStorage{
		basePath: basePath,
		watchers: make(map[string][]func([]byte)),
		stopChan: make(chan struct{}),
	}, nil
}

// Read reads data from storage
func (fs *FileStorage) Read(key string) ([]byte, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	path := fs.keyToPath(key)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("key not found: %s", key)
		}
		return nil, err
	}

	return data, nil
}

// Write writes data to storage
func (fs *FileStorage) Write(key string, data []byte) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	path := fs.keyToPath(key)

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write atomically
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // Clean up
		return fmt.Errorf("failed to rename file: %w", err)
	}

	// Notify watchers
	if handlers, ok := fs.watchers[key]; ok {
		for _, handler := range handlers {
			go handler(data)
		}
	}

	return nil
}

// Delete removes data from storage
func (fs *FileStorage) Delete(key string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	path := fs.keyToPath(key)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("key not found: %s", key)
		}
		return err
	}

	return nil
}

// List lists keys with given prefix
func (fs *FileStorage) List(prefix string) ([]string, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	var keys []string

	err := filepath.Walk(fs.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Convert path back to key
		relPath, err := filepath.Rel(fs.basePath, path)
		if err != nil {
			return err
		}

		key := fs.pathToKey(relPath)
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}

		return nil
	})

	return keys, err
}

// Watch watches for changes to a key
func (fs *FileStorage) Watch(key string, handler func([]byte)) (func(), error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	fs.watchers[key] = append(fs.watchers[key], handler)

	// Return unsubscribe function
	return func() {
		fs.mu.Lock()
		defer fs.mu.Unlock()

		if handlers, ok := fs.watchers[key]; ok {
			// Remove this handler
			for i, h := range handlers {
				if &h == &handler {
					fs.watchers[key] = append(handlers[:i], handlers[i+1:]...)
					break
				}
			}

			// Clean up if no more handlers
			if len(fs.watchers[key]) == 0 {
				delete(fs.watchers, key)
			}
		}
	}, nil
}

// GetBasePath returns the base path of the file storage
func (fs *FileStorage) GetBasePath() string {
	return fs.basePath
}

// keyToPath converts storage key to filesystem path
func (fs *FileStorage) keyToPath(key string) string {
	// Replace : with / for hierarchical storage
	parts := strings.Split(key, ":")
	path := filepath.Join(fs.basePath, filepath.Join(parts...))

	// Add .json extension if not present
	if !strings.HasSuffix(path, ".json") {
		path += ".json"
	}

	return path
}

// pathToKey converts filesystem path to storage key
func (fs *FileStorage) pathToKey(relPath string) string {
	// Remove .json extension
	if strings.HasSuffix(relPath, ".json") {
		relPath = relPath[:len(relPath)-5]
	}

	// Replace path separators with :
	return strings.ReplaceAll(relPath, string(filepath.Separator), ":")
}

// MemoryStorage implements Storage interface in memory
type MemoryStorage struct {
	mu       sync.RWMutex
	data     map[string][]byte
	watchers map[string][]func([]byte)
}

// NewMemoryStorage creates a new memory-based storage
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		data:     make(map[string][]byte),
		watchers: make(map[string][]func([]byte)),
	}
}

// Read reads data from memory
func (ms *MemoryStorage) Read(key string) ([]byte, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	data, ok := ms.data[key]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	// Return copy to prevent modification
	result := make([]byte, len(data))
	copy(result, data)
	return result, nil
}

// Write writes data to memory
func (ms *MemoryStorage) Write(key string, data []byte) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Store copy to prevent external modification
	stored := make([]byte, len(data))
	copy(stored, data)
	ms.data[key] = stored

	// Notify watchers
	if handlers, ok := ms.watchers[key]; ok {
		for _, handler := range handlers {
			go handler(stored)
		}
	}

	return nil
}

// Delete removes data from memory
func (ms *MemoryStorage) Delete(key string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if _, ok := ms.data[key]; !ok {
		return fmt.Errorf("key not found: %s", key)
	}

	delete(ms.data, key)
	return nil
}

// List lists keys with given prefix
func (ms *MemoryStorage) List(prefix string) ([]string, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	var keys []string
	for key := range ms.data {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}

	return keys, nil
}

// Watch watches for changes to a key
func (ms *MemoryStorage) Watch(key string, handler func([]byte)) (func(), error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.watchers[key] = append(ms.watchers[key], handler)

	// Return unsubscribe function
	return func() {
		ms.mu.Lock()
		defer ms.mu.Unlock()

		if handlers, ok := ms.watchers[key]; ok {
			// Remove this handler
			for i, h := range handlers {
				if &h == &handler {
					ms.watchers[key] = append(handlers[:i], handlers[i+1:]...)
					break
				}
			}

			// Clean up if no more handlers
			if len(ms.watchers[key]) == 0 {
				delete(ms.watchers, key)
			}
		}
	}, nil
}

// StorageKeys defines standard storage keys
type StorageKeys struct{}

var Keys = StorageKeys{}

func (StorageKeys) Config() string {
	return "config"
}

func (StorageKeys) Target(name string) string {
	return fmt.Sprintf("targets:%s:config", name)
}

func (StorageKeys) Project(path string) string {
	// Sanitize path for use as key
	sanitized := strings.ReplaceAll(path, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, "~", "home")
	return fmt.Sprintf("projects:%s:config", sanitized)
}

func (StorageKeys) Backup(id string) string {
	return fmt.Sprintf("backups:%s:data", id)
}

func (StorageKeys) BackupList() string {
	return "backups:list"
}

func (StorageKeys) ServerCache() string {
	return "cache:servers:list"
}

func (StorageKeys) ProjectCache(path string) string {
	sanitized := strings.ReplaceAll(path, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, "~", "home")
	return fmt.Sprintf("cache:projects:%s", sanitized)
}

func (StorageKeys) AutoSyncState() string {
	return "state:autosync:status"
}

func (StorageKeys) LastSync(target string) string {
	return fmt.Sprintf("state:sync:%s:last", target)
}

// Helper functions for storage operations

// LoadJSON loads and unmarshals JSON data
func LoadJSON(storage Storage, key string, v interface{}) error {
	data, err := storage.Read(key)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, v)
}

// SaveJSON marshals and saves JSON data
func SaveJSON(storage Storage, key string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	return storage.Write(key, data)
}

// CopyKey copies data from one key to another
func CopyKey(storage Storage, srcKey, dstKey string) error {
	data, err := storage.Read(srcKey)
	if err != nil {
		return err
	}

	return storage.Write(dstKey, data)
}

// ExportStorage exports all data to a writer
func ExportStorage(storage Storage, w io.Writer) error {
	keys, err := storage.List("")
	if err != nil {
		return err
	}

	export := make(map[string]json.RawMessage)

	for _, key := range keys {
		data, err := storage.Read(key)
		if err != nil {
			continue // Skip errors
		}
		export[key] = json.RawMessage(data)
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(export)
}

// ImportStorage imports data from a reader
func ImportStorage(storage Storage, r io.Reader) error {
	var data map[string]json.RawMessage

	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&data); err != nil {
		return err
	}

	for key, value := range data {
		if err := storage.Write(key, []byte(value)); err != nil {
			return fmt.Errorf("failed to import key %s: %w", key, err)
		}
	}

	return nil
}
