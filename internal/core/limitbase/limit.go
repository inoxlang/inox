package limitbase

import (
	"errors"
	"fmt"
	"math"
)

const (
	THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME = "threads/simul-instances"
	EXECUTION_TOTAL_LIMIT_NAME                = "execution/total-time"

	// Note:
	// This limit represents a pseudo CPU time because it's not possible to accurately detect when
	// the goroutine executing a module is waiting for IO.
	//
	// Implementation note:
	// CPU time token depletion should not be paused during lockings that are both shorts & often successful on the first try
	// because it would introduce overhead. Pausing the depletion involves an atomic write.
	EXECUTION_CPU_TIME_LIMIT_NAME = "execution/cpu-time"

	MAX_LIMIT_VALUE = math.MaxInt64 / TOKEN_BUCKET_CAPACITY_SCALE

	//Token count should be scaled by this value when calling .Take() for a frequency limit.
	//This is not related to the internal scaling of token buckets.
	FREQ_LIMIT_SCALE = 1000
)

var (
	ErrTokenDepletionAlreadyPaused = errors.New("token depletion already paused")
	ErrTokenDepletionNotPaused     = errors.New("token depletion is not paused")
	ErrStateIdNotSet               = errors.New("state id not set")
	ErrUnreachable                 = errors.New("unreachable")
)

func init() {
	ResetLimitRegistry()
}

func ResetLimitRegistry() {
	limRegistry.clear()
	limRegistry.registerLimit(THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME, TotalLimit, 0)
	limRegistry.registerLimit(EXECUTION_TOTAL_LIMIT_NAME, TotalLimit, 0)
	limRegistry.registerLimit(EXECUTION_CPU_TIME_LIMIT_NAME, TotalLimit, 0)
}

// A Limit represents a limit for a running piece of code, for example: the maximum rate of http requests.
// A Context stores one token bucket for each provided limit. A Limit does not hold any state.
type Limit struct {
	Name  string
	Kind  LimitKind
	Value int64

	DecrementFn TokenDepletionFn //optional. Called on each tick of the associated bucket's timer.
}

func (l Limit) LessOrAsRestrictiveAs(other Limit) bool {
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
	FrequencyLimit = LimitKind(iota)
	ByteRateLimit
	TotalLimit
)

func isLimitWithAutoDepletion(limitName string) bool {
	switch limitName {
	case EXECUTION_TOTAL_LIMIT_NAME, EXECUTION_CPU_TIME_LIMIT_NAME:
		return true
	}
	return false
}

func MustMakeNotAutoDepletingCountLimit(limitName string, value int64) Limit {
	if isLimitWithAutoDepletion(limitName) {
		panic(fmt.Errorf("invalid argument: limit %q has auto depletion", limitName))
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

func MustGetMinimumNotAutoDepletingCountLimit(limitName string) Limit {

	if isLimitWithAutoDepletion(limitName) {
		panic(fmt.Errorf("invalid argument: limit %q has auto depletion", limitName))
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
