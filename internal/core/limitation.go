package core

import (
	"fmt"
	"time"
)

var LimRegistry = limitRegistry{
	kinds:         make(map[string]LimitKind),
	minimumLimits: make(map[string]int64),
}

func init() {
	LimRegistry.RegisterLimit(EXECUTION_TOTAL_LIMIT_NAME, TotalLimit, 0)
}

// A Limits represents a limit for a running piece of code, for example: the maximum rate of http requests.
// A Context stores one token bucket for each provided limit.
type Limits struct {
	Name  string
	Kind  LimitKind
	Value int64

	DecrementFn func(lastDecrementTime time.Time) int64 //optional. Called on each tick of the associated bucket's timer.
}

type LimitKind int

const (
	SimpleRateLimit = LimitKind(iota)
	ByteRateLimit
	TotalLimit
)

type Limiter struct {
	limit  Limits
	bucket *tokenBucket
}

type limitRegistry struct {
	kinds         map[string]LimitKind
	minimumLimits map[string]int64
}

func (r *limitRegistry) RegisterLimit(name string, kind LimitKind, minimumLimit int64) {
	registeredKind, ok := r.kinds[name]
	if ok && (registeredKind != kind || minimumLimit != r.minimumLimits[name]) {
		panic(fmt.Errorf("cannot register the limit '%s' with a different type or minimum", name))
	}

	if !ok {
		r.kinds[name] = kind
		r.minimumLimits[name] = minimumLimit
	}
}
func (r *limitRegistry) getLimitInfo(name string) (kind LimitKind, minimum int64, ok bool) {
	registeredKind, ok := r.kinds[name]
	min := r.minimumLimits[name]

	if !ok {
		return -1, -1, false
	}
	return registeredKind, min, true
}
