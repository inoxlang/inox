package jsoniter

import (
	"sync"
)

// Config customize how the API should behave.
// The API is created from Config by Froze.
type Config struct {
	IndentionStep int
}

// API the public interface of this package.
// Primary Marshal and Unmarshal.
type API interface {
	IteratorPool
	StreamPool
	Valid(data []byte) bool
}

// ConfigDefault the default API
var ConfigDefault = Config{}.Froze()

type frozenConfig struct {
	configBeforeFrozen Config
	streamPool         *sync.Pool
	iteratorPool       *sync.Pool
}

// Froze forge API from config
func (cfg Config) Froze() API {
	api := &frozenConfig{}
	api.streamPool = &sync.Pool{
		New: func() interface{} {
			return NewStream(api, nil, 512)
		},
	}
	api.iteratorPool = &sync.Pool{
		New: func() interface{} {
			return NewIterator(api)
		},
	}
	api.configBeforeFrozen = cfg
	return api
}

func (cfg *frozenConfig) Valid(data []byte) bool {
	iter := cfg.BorrowIterator(data)
	defer cfg.ReturnIterator(iter)
	iter.Skip()
	return iter.Error == nil
}
