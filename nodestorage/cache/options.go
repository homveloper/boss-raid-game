package cache

import (
	"time"
)

// Common option types

// Option is a function that configures an option
type Option interface{}

// MapCacheOption is a function that configures MapCache options
type MapCacheOption func(*MapCacheOptions)

// BadgerCacheOption is a function that configures BadgerCache options
type BadgerCacheOption func(*BadgerCacheOptions)

// MapCacheOptions represents configuration options for the MapCache
type MapCacheOptions struct {
	// Maximum number of documents in cache
	MaxSize int64

	// Default TTL for cache entries
	DefaultTTL time.Duration

	// How often to run the eviction process
	EvictionInterval time.Duration
}

// BadgerCacheOptions represents configuration options for the BadgerCache
type BadgerCacheOptions struct {
	// Path to store BadgerDB files
	Path string

	// Whether to use in-memory storage
	InMemory bool

	// Default TTL for cache entries
	DefaultTTL time.Duration

	// GC interval for BadgerDB
	GCInterval time.Duration
}

// Default options

// DefaultMapCacheOptions returns the default MapCache options
func DefaultMapCacheOptions() *MapCacheOptions {
	return &MapCacheOptions{
		MaxSize:          10000,
		DefaultTTL:       time.Hour * 24,
		EvictionInterval: time.Minute * 5,
	}
}

// DefaultBadgerCacheOptions returns the default BadgerCache options
func DefaultBadgerCacheOptions() *BadgerCacheOptions {
	return &BadgerCacheOptions{
		Path:       "./badger-data",
		InMemory:   false,
		DefaultTTL: time.Hour * 24,
		GCInterval: time.Minute * 5,
	}
}

// MapCache options

// WithMapMaxSize sets the maximum number of documents in the map cache
func WithMapMaxSize(maxSize int64) MapCacheOption {
	return func(opts *MapCacheOptions) {
		opts.MaxSize = maxSize
	}
}

// WithMapDefaultTTL sets the default TTL for map cache entries
func WithMapDefaultTTL(ttl time.Duration) MapCacheOption {
	return func(opts *MapCacheOptions) {
		opts.DefaultTTL = ttl
	}
}

// WithMapEvictionInterval sets how often to run the eviction process for map cache
func WithMapEvictionInterval(interval time.Duration) MapCacheOption {
	return func(opts *MapCacheOptions) {
		opts.EvictionInterval = interval
	}
}

// BadgerCache options

// WithBadgerPath sets the path to store BadgerDB files
func WithBadgerPath(path string) BadgerCacheOption {
	return func(opts *BadgerCacheOptions) {
		opts.Path = path
	}
}

// WithBadgerInMemory sets whether to use in-memory storage for BadgerDB
func WithBadgerInMemory(inMemory bool) BadgerCacheOption {
	return func(opts *BadgerCacheOptions) {
		opts.InMemory = inMemory
	}
}

// WithBadgerDefaultTTL sets the default TTL for BadgerDB cache entries
func WithBadgerDefaultTTL(ttl time.Duration) BadgerCacheOption {
	return func(opts *BadgerCacheOptions) {
		opts.DefaultTTL = ttl
	}
}

// WithBadgerGCInterval sets how often to run the garbage collection for BadgerDB
func WithBadgerGCInterval(interval time.Duration) BadgerCacheOption {
	return func(opts *BadgerCacheOptions) {
		opts.GCInterval = interval
	}
}
