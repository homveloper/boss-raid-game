package cache

import "errors"

var (
	// ErrCacheMiss is returned when a document is not found in cache
	ErrCacheMiss = errors.New("cache miss")
	
	// ErrClosed is returned when operating on a closed cache
	ErrClosed = errors.New("cache is closed")
)
