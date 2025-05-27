package engine

import (
	"sync"
)

// Event data types
type ConfigChangeEvent struct {
	Config     *Config
	OldConfig  *Config
	ChangeType string
}

type ServerChangeEvent struct {
	ServerName string
	Server     ServerWithMetadata
	ChangeType string
}

type SyncEvent struct {
	Result SyncResult
}

type BackupEvent struct {
	BackupInfo BackupInfo
}

type ErrorEvent struct {
	Error   error
	Context string
}

// eventBus provides internal event handling
type eventBus struct {
	mu       sync.RWMutex
	handlers map[EventType][]interface{}
}

func newEventBus() *eventBus {
	return &eventBus{
		handlers: make(map[EventType][]interface{}),
	}
}

func (eb *eventBus) on(event EventType, handler interface{}) func() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.handlers[event] = append(eb.handlers[event], handler)

	// Return unsubscribe function
	return func() {
		eb.mu.Lock()
		defer eb.mu.Unlock()

		if handlers, ok := eb.handlers[event]; ok {
			for i, h := range handlers {
				if &h == &handler {
					eb.handlers[event] = append(handlers[:i], handlers[i+1:]...)
					break
				}
			}
		}
	}
}

func (eb *eventBus) emit(event EventType, data interface{}) {
	eb.mu.RLock()
	handlers := eb.handlers[event]
	eb.mu.RUnlock()

	for _, handler := range handlers {
		// Call handler in goroutine to prevent blocking
		go func(h interface{}) {
			switch event {
			case EventConfigLoaded, EventConfigSaved, EventConfigChanged,
				EventAutoSyncStarted, EventAutoSyncStopped, EventFileChanged:
				if fn, ok := h.(func(ConfigChange)); ok {
					if evt, ok := data.(ConfigChange); ok {
						fn(evt)
					}
				} else if fn, ok := h.(ConfigChangeHandler); ok {
					if evt, ok := data.(ConfigChange); ok {
						fn(evt)
					}
				}
			case EventServerAdded, EventServerUpdated, EventServerRemoved:
				if fn, ok := h.(func(ServerChangeEvent)); ok {
					if evt, ok := data.(ServerChangeEvent); ok {
						fn(evt)
					}
				}
			case EventSyncStarted, EventSyncCompleted, EventSyncFailed:
				if fn, ok := h.(func(SyncResult)); ok {
					if result, ok := data.(SyncResult); ok {
						fn(result)
					}
				}
			case EventBackupCreated, EventBackupRestored:
				if fn, ok := h.(func(BackupInfo)); ok {
					if info, ok := data.(BackupInfo); ok {
						fn(info)
					}
				}
			case EventError:
				if fn, ok := h.(func(error)); ok {
					if err, ok := data.(error); ok {
						fn(err)
					}
				}
			}
		}(handler)
	}
}