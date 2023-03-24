package internal

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	"github.com/inox-project/inox/internal/utils"
)

const (
	TOKEN_BUCKET_CAPACITY_SCALE = 100
	TOKEN_BUCKET_INTERVAL       = time.Second / TOKEN_BUCKET_CAPACITY_SCALE
)

//Token bucket implementation, see https://github.com/DavidCai1993/token-bucket (MIT license)
//NOTE: The code has been slightly modified
//TODO: replace with an implementation that uses a single global goroutine

// tokenBucket represents a token bucket
// (https://en.wikipedia.org/wiki/Token_bucket) which based on multi goroutines,
// and is safe to use under concurrency environments.
type tokenBucket struct {
	interval                     time.Duration
	ticker                       *time.Ticker
	tokenMutex                   *sync.Mutex
	waitingQuqueMutex            *sync.Mutex
	waitingQuque                 *list.List
	cap                          int64
	avail                        int64
	increment                    int64
	decrementFn                  func(time.Time) int64
	context                      *Context
	cancelContextOnNegativeCount bool
	lastDecrementTime            time.Time
}

type waitingJob struct {
	ch        chan struct{}
	need      int64
	use       int64
	abandoned bool
}

type tokenBucketConfig struct {
	interval                     time.Duration
	cap                          int64
	initialAvail                 int64
	inc                          int64
	decrementFn                  func(time.Time) int64
	cancelContextOnNegativeCount bool
}

// newBucket returns a new token bucket with specified fill interval and
// capability. The bucket is initially full.
func newBucket(config tokenBucketConfig) *tokenBucket {
	if config.interval < 0 {
		panic(fmt.Sprintf("token bucket: interval %v should > 0", config.interval))
	}

	if config.cap < 0 {
		panic(fmt.Sprintf("token bucket: capability %v should > 0", config.cap))
	}

	avail := config.initialAvail
	if avail < 0 {
		avail = config.cap
	}

	tb := &tokenBucket{
		interval:                     config.interval,
		tokenMutex:                   &sync.Mutex{},
		waitingQuqueMutex:            &sync.Mutex{},
		waitingQuque:                 list.New(),
		cap:                          config.cap,
		avail:                        avail,
		increment:                    config.inc,
		ticker:                       time.NewTicker(config.interval),
		decrementFn:                  config.decrementFn,
		cancelContextOnNegativeCount: config.cancelContextOnNegativeCount,
		lastDecrementTime:            time.Now(),
	}

	go tb.adjustDaemon()

	return tb
}

func (tb *tokenBucket) SetContext(ctx *Context) {
	tb.tokenMutex.Lock()
	defer tb.tokenMutex.Unlock()

	tb.context = ctx
}

// Capability returns the capability of this token bucket.
func (tb *tokenBucket) Capability() int64 {
	return tb.cap
}

// Available returns how many tokens are available in the bucket.
func (tb *tokenBucket) Available() int64 {
	tb.tokenMutex.Lock()
	defer tb.tokenMutex.Unlock()

	return tb.avail
}

// TryTake trys to task specified count tokens from the bucket. if there are
// not enough tokens in the bucket, it will return false.
func (tb *tokenBucket) TryTake(count int64) bool {
	return tb.tryTake(count, count)
}

// Take tasks specified count tokens from the bucket, if there are
// not enough tokens in the bucket, it will keep waiting until count tokens are
// availible and then take them.
func (tb *tokenBucket) Take(count int64) {
	tb.waitAndTake(count, count)
}

func (tb *tokenBucket) GiveBack(count int64) {
	tb.tokenMutex.Lock()
	defer tb.tokenMutex.Unlock()

	tb.avail += count
	tb.avail = utils.Min(tb.cap, tb.avail)
}

// TakeMaxDuration tasks specified count tokens from the bucket, if there are
// not enough tokens in the bucket, it will keep waiting until count tokens are
// availible and then take them or just return false when reach the given max
// duration.
func (tb *tokenBucket) TakeMaxDuration(count int64, max time.Duration) bool {
	return tb.waitAndTakeMaxDuration(count, count, max)
}

// Wait will keep waiting until count tokens are availible in the bucket.
func (tb *tokenBucket) Wait(count int64) {
	tb.waitAndTake(count, 0)
}

// WaitMaxDuration will keep waiting until count tokens are availible in the
// bucket or just return false when reach the given max duration.
func (tb *tokenBucket) WaitMaxDuration(count int64, max time.Duration) bool {
	return tb.waitAndTakeMaxDuration(count, 0, max)
}

func (tb *tokenBucket) tryTake(need, use int64) bool {
	tb.checkCount(use)

	tb.tokenMutex.Lock()
	defer tb.tokenMutex.Unlock()

	if need <= tb.avail {
		tb.avail -= use

		return true
	}

	return false
}

func (tb *tokenBucket) waitAndTake(need, use int64) {
	if ok := tb.tryTake(need, use); ok {
		return
	}

	w := &waitingJob{
		ch:   make(chan struct{}),
		use:  use,
		need: need,
	}

	tb.addWaitingJob(w)

	<-w.ch
	tb.avail -= use
	w.ch <- struct{}{}

	close(w.ch)
}

func (tb *tokenBucket) waitAndTakeMaxDuration(need, use int64, max time.Duration) bool {
	if ok := tb.tryTake(need, use); ok {
		return true
	}

	w := &waitingJob{
		ch:   make(chan struct{}),
		use:  use,
		need: need,
	}

	defer close(w.ch)

	tb.addWaitingJob(w)

	select {
	case <-w.ch:
		tb.avail -= use
		w.ch <- struct{}{}
		return true
	case <-time.After(max):
		w.abandoned = true
		return false
	}
}

// Destroy destroys the token bucket and stop the inner channels.
func (tb *tokenBucket) Destroy() {
	tb.ticker.Stop()
}

func (tb *tokenBucket) adjustDaemon() {
	var waitingJobNow *waitingJob

	for range tb.ticker.C {

		tb.tokenMutex.Lock()

		if tb.decrementFn == nil {
			if tb.avail < tb.cap {
				tb.avail = tb.avail + tb.increment
			}
		} else {
			tb.avail = tb.avail - tb.decrementFn(tb.lastDecrementTime)
		}

		if tb.avail < 0 && tb.cancelContextOnNegativeCount && tb.context != nil {
			tb.context.Cancel() // add reason
		}

		tb.avail = utils.Max(0, tb.avail)

		tb.lastDecrementTime = time.Now()
		element := tb.getFrontWaitingJob()

		if element != nil {
			if waitingJobNow == nil || waitingJobNow.abandoned {
				waitingJobNow = element.Value.(*waitingJob)

				tb.removeWaitingJob(element)
			}
		}

		if waitingJobNow != nil && tb.avail >= waitingJobNow.need && !waitingJobNow.abandoned {
			waitingJobNow.ch <- struct{}{}

			<-waitingJobNow.ch

			waitingJobNow = nil
		}

		tb.tokenMutex.Unlock()
	}
}

func (tb *tokenBucket) addWaitingJob(w *waitingJob) {
	tb.waitingQuqueMutex.Lock()
	tb.waitingQuque.PushBack(w)
	tb.waitingQuqueMutex.Unlock()
}

func (tb *tokenBucket) getFrontWaitingJob() *list.Element {
	tb.waitingQuqueMutex.Lock()
	e := tb.waitingQuque.Front()
	tb.waitingQuqueMutex.Unlock()

	return e
}

func (tb *tokenBucket) removeWaitingJob(e *list.Element) {
	tb.waitingQuqueMutex.Lock()
	tb.waitingQuque.Remove(e)
	tb.waitingQuqueMutex.Unlock()
}

func (tb *tokenBucket) checkCount(count int64) {
	if count < 0 || count > tb.cap {
		panic(fmt.Sprintf("token-bucket: count %v should be less than bucket's"+
			" capablity %v", count, tb.cap))
	}
}
