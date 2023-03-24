package internal

import (
	"fmt"
	"time"
)

var LimRegistry = limitationRegistry{
	kinds:              make(map[string]LimitationKind),
	minimumLimitations: make(map[string]int64),
}

func init() {
	LimRegistry.RegisterLimitation(EXECUTION_TOTAL_LIMIT_NAME, TotalLimitation, 0)
}

// A Limitation represents a limitation for a running piece of code, for example: the maximum rate of http requests.
// A Context stores one token bucket for each provided limitation.
type Limitation struct {
	Name  string
	Kind  LimitationKind
	Value int64

	DecrementFn func(lastDecrementTime time.Time) int64 //optional. Called on each tick of the associated bucket's timer.
}

type LimitationKind int

const (
	SimpleRateLimitation = LimitationKind(iota)
	ByteRateLimitation
	TotalLimitation
)

type Limiter struct {
	limitation Limitation
	bucket     *tokenBucket
}

type limitationRegistry struct {
	kinds              map[string]LimitationKind
	minimumLimitations map[string]int64
}

func (r *limitationRegistry) RegisterLimitation(name string, kind LimitationKind, minimumLimitation int64) {
	registeredKind, ok := r.kinds[name]
	if ok && (registeredKind != kind || minimumLimitation != r.minimumLimitations[name]) {
		panic(fmt.Errorf("cannot register the limitation '%s' with a different type or minimum", name))
	}

	if !ok {
		r.kinds[name] = kind
		r.minimumLimitations[name] = minimumLimitation
	}
}
func (r *limitationRegistry) getLimitationInfo(name string) (kind LimitationKind, minimum int64, ok bool) {
	registeredKind, ok := r.kinds[name]
	min := r.minimumLimitations[name]

	if !ok {
		return -1, -1, false
	}
	return registeredKind, min, true
}
