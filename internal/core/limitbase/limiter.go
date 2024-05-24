package limitbase

import (
	"fmt"
	"sync/atomic"
)

// A Limiter manages a limit for a single state, it is not thread safe.
type Limiter struct {
	limit           Limit
	bucket          *tokenBucket //shared with parent if is child limiter
	parentLimiter   *Limiter
	stateId         int64
	pausedDepletion bool

	// prevent race condition when Destroy & DefinitelyStopDepletion are called
	// at the same time.
	definitelyStopped atomic.Bool

	//TODO: reduce CPU & memory usage by using only an atomic int64 for limits of kind total with no decrement function.
	//the atomic value should be shared with child limiters.
}

func NewLimiter(limit Limit, bucketConfig TokenBucketConfig) *Limiter {
	return &Limiter{
		limit:  limit,
		bucket: newBucket(bucketConfig),
	}
}

func (l *Limiter) Limit() Limit {
	return l.limit
}

func (l *Limiter) SetStateOnce(id int64) {
	l.stateId = id

	if l.limit.DecrementFn != nil {
		l.bucket.ResumeOneStateDepletion()
	}
}

func (l *Limiter) SetContextIfNotChild(ctx Context) {
	//The context is only set if the limiter is not a child because the bucket is shared between the parent and children.
	//In other words if $l is a child limiter the context of its bucket should already be set.
	if l.parentLimiter == nil {
		l.bucket.SetContext(ctx)
	}
}

func (l *Limiter) Child() *Limiter {
	return &Limiter{
		limit:         l.limit,
		bucket:        l.bucket,
		parentLimiter: l,
	}
}

func (l *Limiter) Destroy() {
	if l.parentLimiter == nil {
		l.definitelyStopped.Store(true)
		l.bucket.Destroy()
	} else if l.definitelyStopped.CompareAndSwap(false, true) && !l.pausedDepletion {
		l.bucket.PauseOneStateDepletion()
	}
}

func (l *Limiter) ParentLimiter() *Limiter {
	return l.parentLimiter
}

func (l *Limiter) Available() int64 {
	return l.bucket.Available()
}

// Total checks that the limit is of kind total and returns the number of available tokens.
func (l *Limiter) Total() (int64, error) {
	if l.limit.Kind != TotalLimit {
		return -1, fmt.Errorf("context: '%s' is not a total limit", l.limit.Name)
	}

	return l.bucket.Available(), nil
}

// Take takes count tokens from the bucket, it panics for total limits when the available count is less than count.
func (l *Limiter) Take(count int64) {
	available := l.bucket.Available()
	if l.limit.Kind == TotalLimit && l.limit.Value != 0 && available < count {
		panic(fmt.Errorf("cannot take %v tokens from bucket (%s), only %v token(s) available", count, l.limit.Name, available))
	}
	l.bucket.Take(count)
}

func (l *Limiter) GiveBack(count int64) {
	l.bucket.GiveBack(count)
}

func (l *Limiter) Bucket() *tokenBucket {
	return l.bucket
}

// PauseDepletion pauses the token depletion.
func (l *Limiter) PauseDepletion() {
	if l.stateId == 0 {
		panic(ErrStateIdNotSet)
	}
	if l.pausedDepletion {
		panic(ErrTokenDepletionAlreadyPaused)
	}
	l.bucket.PauseOneStateDepletion()
	l.pausedDepletion = true
}

func (l *Limiter) PauseDepletionIfNotPaused() {
	if l.pausedDepletion {
		return
	}
	l.PauseDepletion()
}

func (l *Limiter) DefinitelyStopDepletion() {
	if l.definitelyStopped.CompareAndSwap(false, true) && !l.pausedDepletion {
		l.bucket.PauseOneStateDepletion()
	}
}

func (l *Limiter) ResumeDepletion() {
	if l.stateId == 0 {
		panic(ErrStateIdNotSet)
	}
	if !l.pausedDepletion {
		panic(ErrTokenDepletionNotPaused)
	}
	l.bucket.ResumeOneStateDepletion()
	l.pausedDepletion = false
}
