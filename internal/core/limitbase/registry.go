package limitbase

import (
	"fmt"
	"sync"
)

var (
	limRegistry = limitRegistry{
		kinds:         make(map[string]LimitKind),
		minimumLimits: make(map[string]int64),
	}
)

func ForEachRegisteredLimit(fn func(name string, kind LimitKind, minimum int64) error) error {
	return limRegistry.forEachRegisteredLimit(fn)
}

func GetRegisteredLimitInfo(name string) (kind LimitKind, minimum int64, ok bool) {
	return limRegistry.getLimitInfo(name)
}

type limitRegistry struct {
	lock          sync.Mutex
	kinds         map[ /* name */ string]LimitKind
	minimumLimits map[ /* name */ string]int64
}

func RegisterLimit(name string, kind LimitKind, minimumLimit int64) {
	limRegistry.registerLimit(name, kind, minimumLimit)
}

func (r *limitRegistry) registerLimit(name string, kind LimitKind, minimumLimit int64) {
	r.lock.Lock()
	defer r.lock.Unlock()

	registeredKind, ok := r.kinds[name]
	if ok && (registeredKind != kind || minimumLimit != r.minimumLimits[name]) {
		panic(fmt.Errorf("cannot register the limit '%s' with a different kind or minimum", name))
	}

	if !ok {
		r.kinds[name] = kind
		r.minimumLimits[name] = minimumLimit
	}
}

func (r *limitRegistry) getLimitInfo(name string) (kind LimitKind, minimum int64, ok bool) {
	r.lock.Lock()
	defer r.lock.Unlock()

	registeredKind, ok := r.kinds[name]
	min := r.minimumLimits[name]

	if !ok {
		return -1, -1, false
	}
	return registeredKind, min, true
}

func (r *limitRegistry) forEachRegisteredLimit(fn func(name string, kind LimitKind, minimum int64) error) error {
	for name, minimum := range r.minimumLimits {
		kind, ok := r.kinds[name]
		if !ok {
			panic(ErrUnreachable)
		}

		err := fn(name, kind, minimum)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *limitRegistry) clear() {
	r.lock.Lock()
	defer r.lock.Unlock()
	clear(r.kinds)
	clear(r.minimumLimits)
}
