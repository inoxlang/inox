package core

import (
	"fmt"
	"log"
	"time"
)

var LimRegistry = limitRegistry{
	kinds:         make(map[string]LimitKind),
	minimumLimits: make(map[string]int64),
}

func init() {
	LimRegistry.RegisterLimit(EXECUTION_TOTAL_LIMIT_NAME, TotalLimit, 0)
	LimRegistry.RegisterLimit(EXECUTION_CPU_TIME_LIMIT_NAME, TotalLimit, 0)
}

// A Limit represents a limit for a running piece of code, for example: the maximum rate of http requests.
// A Context stores one token bucket for each provided limit.
type Limit struct {
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
	limit  Limit
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

func getLimit(ctx *Context, limitName string, limitValue Serializable) (_ Limit, resultErr error) {
	var limit Limit

	switch v := limitValue.(type) {
	case Rate:
		limit = Limit{Name: limitName}

		switch r := v.(type) {
		case ByteRate:
			limit.Kind = ByteRateLimit
			limit.Value = int64(r)
		case SimpleRate:
			limit.Kind = SimpleRateLimit
			limit.Value = int64(r)
		default:
			resultErr = fmt.Errorf("not a valid rate type %T", r)
			return
		}

	case Int:
		limit = Limit{
			Name:  limitName,
			Kind:  TotalLimit,
			Value: int64(v),
		}
	case Duration:
		limit = Limit{
			Name:  limitName,
			Kind:  TotalLimit,
			Value: int64(v),
		}
	default:
		resultErr = fmt.Errorf("invalid manifest, invalid value %s for a limit", Stringify(v, ctx))
		return
	}

	registeredKind, registeredMinimum, ok := LimRegistry.getLimitInfo(limitName)
	if !ok {
		resultErr = fmt.Errorf("invalid manifest, limits: '%s' is not a registered limit", limitName)
		return
	}
	if limit.Kind != registeredKind {
		resultErr = fmt.Errorf("invalid manifest, limits: value of '%s' has not a valid type", limitName)
		return
	}
	if registeredMinimum > 0 && limit.Value < registeredMinimum {
		resultErr = fmt.Errorf("invalid manifest, limits: value for limit '%s' is too low, minimum is %d", limitName, registeredMinimum)
		return
	}

	//check & postprocess limits

	switch limit.Name {
	case EXECUTION_TOTAL_LIMIT_NAME:
		if limit.Value == 0 {
			log.Panicf("invalid manifest, limits: %s should have a total value\n", EXECUTION_TOTAL_LIMIT_NAME)
		}
		limit.DecrementFn = func(lastDecrementTime time.Time) int64 {
			return time.Since(lastDecrementTime).Nanoseconds()
		}
	case EXECUTION_CPU_TIME_LIMIT_NAME:
		if limit.Value == 0 {
			log.Panicf("invalid manifest, limits: %s should have a total value\n", EXECUTION_CPU_TIME_LIMIT_NAME)
		}
		limit.DecrementFn = func(lastDecrementTime time.Time) int64 {
			return time.Since(lastDecrementTime).Nanoseconds()
		}
	}

	return limit, nil
}
