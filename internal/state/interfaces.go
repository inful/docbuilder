package state

import "time"

// StoreHealth represents the health status of the state store.
type StoreHealth struct {
	Status      string     `json:"status"`
	Message     string     `json:"message,omitempty"`
	LastBackup  *time.Time `json:"last_backup,omitempty"`
	StorageSize *int64     `json:"storage_size_bytes,omitempty"`
	CheckedAt   time.Time  `json:"checked_at"`
}
