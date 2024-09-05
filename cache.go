package gaetway

import (
	"sync"

	"github.com/ardikabs/gaetway/pkg/util"
)

// Cache is an interface that defines methods for storing and retrieving data in an internal cache.
// It is designed to maintain data persistently throughout Envoy's lifespan.
type Cache interface {
	// Store allows you to save a value of any type under a key of any type.
	//
	// Please use caution! The Store function overwrites any existing data.
	Store(key, value any)

	// Load retrieves a value associated with a specific key and assigns it to the receiver.
	//
	// It returns true if a compatible value is successfully loaded,
	// false if no value is found, or an error occurs during the process.
	//
	// If the receiver is not a pointer to the stored data type,
	// Load will return an ErrIncompatibleReceiver.
	//
	// Example usage:
	//   type mystruct struct{}
	//
	//   data := new(mystruct)
	//   cache.Store("keyName", data)
	//
	//   receiver := new(mystruct)
	//   _, _ = cache.Load("keyName", &receiver)
	Load(key, receiver any) (ok bool, err error)
}

type inmemoryCache struct {
	stash sync.Map
}

func newInternalCache() *inmemoryCache {
	return &inmemoryCache{}
}

func (c *inmemoryCache) Store(key, value any) {
	c.stash.Store(key, value)
}

func (c *inmemoryCache) Load(key, receiver any) (bool, error) {
	if receiver == nil {
		return false, ErrNilReceiver
	}

	v, ok := c.stash.Load(key)
	if !ok {
		return false, nil
	}

	if !util.CastTo(receiver, v) {
		return false, ErrIncompatibleReceiver
	}

	return true, nil
}
