package core

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

const (
	THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME = "threads/simul-instances"
	EXECUTION_TOTAL_LIMIT_NAME                = "execution/total-time"

	// Note:
	// This limit represents a pseudo CPU time because it's not possible to accurately detect when
	// the goroutine executing a module is waiting for IO.
	//
	// Implementation note:
	// CPU time decrementation should not be paused during lockings that are both shorts & often successful on the first try
	// because it would introduce overhead. Pausing the decrementation involves an atomic write.
	EXECUTION_CPU_TIME_LIMIT_NAME = "execution/cpu-time"

	MAX_LIMIT_VALUE = math.MaxInt64 / TOKEN_BUCKET_CAPACITY_SCALE
)

var (
	limRegistry = limitRegistry{
		kinds:         make(map[string]LimitKind),
		minimumLimits: make(map[string]int64),
	}

	ErrTokenDecrementationAlreadyPaused = errors.New("token decrementation already paused")
	ErrTokenDecrementationNotPaused     = errors.New("token decrementation is not paused")
	ErrStateIdNotSet                    = errors.New("state id not set")
)

func init() {
	resetLimitRegistry()
}

func resetLimitRegistry() {
	limRegistry.Clear()
	limRegistry.RegisterLimit(THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME, TotalLimit, 0)
	limRegistry.RegisterLimit(EXECUTION_TOTAL_LIMIT_NAME, TotalLimit, 0)
	limRegistry.RegisterLimit(EXECUTION_CPU_TIME_LIMIT_NAME, TotalLimit, 0)
}

// A Limit represents a limit for a running piece of code, for example: the maximum rate of http requests.
// A Context stores one token bucket for each provided limit. A Limit does not hold any state.
type Limit struct {
	Name  string
	Kind  LimitKind
	Value int64

	DecrementFn TokenDecrementationFn //optional. Called on each tick of the associated bucket's timer.
}

func (l Limit) LessRestrictiveThan(other Limit) bool {
	if other.Name != l.Name {
		panic(errors.New("different name"))
	}
	if other.Kind != l.Kind {
		panic(errors.New("different kind"))
	}
	return l.Value >= other.Value
}

func (l Limit) MoreRestrictiveThan(other Limit) bool {
	if other.Name != l.Name {
		panic(errors.New("different name"))
	}
	if other.Kind != l.Kind {
		panic(errors.New("different kind"))
	}
	return l.Value < other.Value
}

type LimitKind int

const (
	SimpleRateLimit = LimitKind(iota)
	ByteRateLimit
	TotalLimit
)

type limitRegistry struct {
	lock          sync.Mutex
	kinds         map[string]LimitKind
	minimumLimits map[string]int64
}

func RegisterLimit(name string, kind LimitKind, minimumLimit int64) {
	limRegistry.RegisterLimit(name, kind, minimumLimit)
}

func (r *limitRegistry) RegisterLimit(name string, kind LimitKind, minimumLimit int64) {
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

func (r *limitRegistry) ForEachRegisteredLimit(fn func(name string, kind LimitKind, minimum int64) error) error {
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

func (r *limitRegistry) Clear() {
	r.lock.Lock()
	defer r.lock.Unlock()
	clear(r.kinds)
	clear(r.minimumLimits)
}

// limiter manages a limit for a single state, it is not thread safe.
type limiter struct {
	limit                Limit
	bucket               *tokenBucket //shared with parent if is child limiter
	parentLimiter        *limiter
	stateId              StateId
	pausedDecrementation bool

	// prevent race condition when Destroy & DefinitelyStopDecrementation are called
	// at the same time.
	definitelyStopped atomic.Bool

	//TODO: reduce CPU & memory usage by using only an atomic int64 for limits of kind total with no decrementation function.
	//the atomic value should be shared with child limiters.
}

func (l *limiter) SetStateOnce(id StateId) {
	l.stateId = id

	if l.limit.DecrementFn != nil {
		l.bucket.ResumeOneStateDecrementation()
	}
}

func (l *limiter) SetContextIfNotChild(ctx *Context) {
	if l.parentLimiter == nil {
		l.bucket.SetContext(ctx)
	}
}

func (l *limiter) Child() *limiter {
	return &limiter{
		limit:         l.limit,
		bucket:        l.bucket,
		parentLimiter: l,
	}
}

func (l *limiter) Destroy() {
	if l.parentLimiter == nil {
		l.definitelyStopped.Store(true)
		l.bucket.Destroy()
	} else if l.definitelyStopped.CompareAndSwap(false, true) && !l.pausedDecrementation {
		l.bucket.PauseOneStateDecrementation()
	}
}

func (l *limiter) Available() int64 {
	return l.bucket.Available()
}

func (l *limiter) Total() (int64, error) {
	if l.limit.Kind != TotalLimit {
		return -1, fmt.Errorf("context: '%s' is not a total limit", l.limit.Name)
	}

	return l.bucket.Available(), nil
}

func (l *limiter) Take(count int64) {

	available := l.bucket.Available()
	if l.limit.Kind == TotalLimit && l.limit.Value != 0 && available < count {
		panic(fmt.Errorf("cannot take %v tokens from bucket (%s), only %v token(s) available", count, l.limit.Name, available))
	}
	l.bucket.Take(count)
}

func (l *limiter) GiveBack(count int64) {
	l.bucket.GiveBack(count)
}

func (l *limiter) PauseDecrementation() {
	if l.stateId == 0 {
		panic(ErrStateIdNotSet)
	}
	if l.pausedDecrementation {
		panic(ErrTokenDecrementationAlreadyPaused)
	}
	l.bucket.PauseOneStateDecrementation()
	l.pausedDecrementation = true
}

func (l *limiter) PauseDecrementationIfNotPaused() {
	if l.pausedDecrementation {
		return
	}
	l.PauseDecrementation()
}

func (l *limiter) DefinitelyStopDecrementation() {
	if l.definitelyStopped.CompareAndSwap(false, true) && !l.pausedDecrementation {
		l.bucket.PauseOneStateDecrementation()
	}
}

func (l *limiter) ResumeDecrementation() {
	if l.stateId == 0 {
		panic(ErrStateIdNotSet)
	}
	if !l.pausedDecrementation {
		panic(ErrTokenDecrementationNotPaused)
	}
	l.bucket.ResumeOneStateDecrementation()
	l.pausedDecrementation = false
}

func isLimitWithDecrementingValue(limitName string) bool {
	switch limitName {
	case EXECUTION_TOTAL_LIMIT_NAME, EXECUTION_CPU_TIME_LIMIT_NAME:
		return true
	}
	return false
}

func MustMakeNotDecrementingLimit(limitName string, value int64) Limit {
	if isLimitWithDecrementingValue(limitName) {
		panic(fmt.Errorf("invalid argument: limit %q has auto decrementation", limitName))
	}

	kind, minimum, ok := limRegistry.getLimitInfo(limitName)
	if !ok {
		panic(fmt.Errorf("limit %q does not exist", limitName))
	}

	if value < minimum {
		panic(fmt.Errorf("value provided for limit %q (%d) is smaller than the allowed minimum (%d)", limitName, value, minimum))
	}

	return Limit{
		Name:  limitName,
		Kind:  kind,
		Value: value,
	}
}

func mustGetMinimumNotDecrementingLimit(limitName string) Limit {

	if isLimitWithDecrementingValue(limitName) {
		panic(fmt.Errorf("invalid argument: limit %q has auto decrementation", limitName))
	}

	kind, minimum, ok := limRegistry.getLimitInfo(limitName)
	if !ok {
		panic(fmt.Errorf("limit %q does not exist", limitName))
	}

	return Limit{
		Name:  limitName,
		Kind:  kind,
		Value: minimum,
	}
}

func GetLimit(ctx *Context, limitName string, limitValue Serializable) (_ Limit, resultErr error) {
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

	registeredKind, registeredMinimum, ok := limRegistry.getLimitInfo(limitName)
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
	if limit.Value > MAX_LIMIT_VALUE {
		resultErr = fmt.Errorf("invalid manifest, limits: value for limit '%s' is too high, hard maximum is %d", limitName, MAX_LIMIT_VALUE)
		return
	}

	//check & postprocess limits

	switch limit.Name {
	case EXECUTION_TOTAL_LIMIT_NAME:
		if limit.Value == 0 {
			resultErr = fmt.Errorf("invalid manifest, limits: %s should have a total value", EXECUTION_TOTAL_LIMIT_NAME)
			return
		}
		limit.DecrementFn = func(lastDecrementTime time.Time, decrementingStateCount int32) int64 {
			return time.Since(lastDecrementTime).Nanoseconds()
		}
	case EXECUTION_CPU_TIME_LIMIT_NAME:
		if limit.Value == 0 {
			resultErr = fmt.Errorf("invalid manifest, limits: %s should have a total value", EXECUTION_CPU_TIME_LIMIT_NAME)
			return
		}
		limit.DecrementFn = func(lastDecrementTime time.Time, decrementingStateCount int32) int64 {
			return time.Since(lastDecrementTime).Nanoseconds() * int64(decrementingStateCount)
		}
		//IMPORTANT: make sure to exclude all limits with auto decrementation in mustGetMinimumNotDecrementingLimit.
	}

	return limit, nil
}
