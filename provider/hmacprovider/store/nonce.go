package store

import (
	"time"
)

const (
	DefaultTTL             = time.Hour * 4
	DefaultCleanupInterval = time.Minute * 15
	DefaultMaxSize         = 2000000 // 2 million entries, ~280Mb of uuids
)

type NonceStore interface {
	AddIfNotExists(nonce string) bool
}
