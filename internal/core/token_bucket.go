package core

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/utils"
)

const (
	TOKEN_BUCKET_MANAGEMENT_TICK_INTERVAL = time.Millisecond
	TOKEN_BUCKET_CAPACITY_SCALE           = int64(time.Second / TOKEN_BUCKET_MANAGEMENT_TICK_INTERVAL)

	MAX_WAIT_CHAN_COUNT = 1000
)

var (
	ErrDestroyedTokenBucket = errors.New("token bucket is destroyed")

	//token bucket management

	tokenBucketManagerStarted atomic.Bool
	tokenBuckets              = map[*tokenBucket]struct{}{}
	tokenBucketsLock          sync.Mutex

	waitChanPool = make(chan (chan (struct{})), MAX_WAIT_CHAN_COUNT)
)

func init() {
	startTokenBucketManagerGoroutine()
}

// A tokenBucket represents a thread-safe token bucket, a single goroutine manages the buckets (see startTokenBucketManagerGoroutine).
// The most important methods are Take(count) and GiveBack(count). Take may wait for the token to refill if there are not enough tokens.
type tokenBucket struct {
	lastDecrementTime time.Time

	lock                   *sync.Mutex
	capacity               ScaledTokenCount
	available              ScaledTokenCount
	increment              ScaledTokenCount
	decrementingStateCount atomic.Int32
	decrementFn            TokenDecrementationFn
	context                *Context

	chanListLock         sync.Mutex
	waitChans            []chan (struct{})
	neededTokenCountList []ScaledTokenCount
	useTokenCountList    []ScaledTokenCount

	cancelContextOnNegativeCount bool
	shouldBeDestroyed            bool
	destroyed                    bool
}

// Token count scaled by TOKEN_BUCKET_CAPACITY_SCALE.
type ScaledTokenCount int64

func (c ScaledTokenCount) RealCount() int64 {
	return int64(c) / TOKEN_BUCKET_CAPACITY_SCALE
}

type TokenDecrementationFn func(lastDecrementTime time.Time, decrementingStateCount int32) int64

type tokenBucketConfig struct {
	cap                          int64
	initialAvail                 int64
	fillRate                     int64 //tokens per second
	decrementFn                  TokenDecrementationFn
	cancelContextOnNegativeCount bool
}

// newBucket returns a new token bucket with the specified fillrate & capacity, the bucket is created full.
func newBucket(config tokenBucketConfig) *tokenBucket {
	if config.cap < 0 {
		panic(fmt.Sprintf("token bucket: capacity %v should be > 0", config.cap))
	}

	avail := config.initialAvail
	if avail < 0 {
		avail = config.cap
	}

	tb := &tokenBucket{
		lock:                         &sync.Mutex{},
		capacity:                     ScaledTokenCount(config.cap * TOKEN_BUCKET_CAPACITY_SCALE),
		available:                    ScaledTokenCount(avail * TOKEN_BUCKET_CAPACITY_SCALE),
		increment:                    ScaledTokenCount(config.fillRate),
		decrementFn:                  config.decrementFn,
		cancelContextOnNegativeCount: config.cancelContextOnNegativeCount,
		lastDecrementTime:            time.Now(),
	}

	tokenBucketsLock.Lock()
	tokenBuckets[tb] = struct{}{}
	tokenBucketsLock.Unlock()

	return tb
}

func (tb *tokenBucket) SetContext(ctx *Context) {
	tb.lock.Lock()
	defer tb.lock.Unlock()

	tb.context = ctx
}

func (tb *tokenBucket) Capacity() int64 {
	return tb.capacity.RealCount()
}

func (tb *tokenBucket) Available() int64 {
	tb.lock.Lock()
	defer tb.lock.Unlock()

	return tb.available.RealCount()
}

func (tb *tokenBucket) assertNotDestroyedNoLock() {
	if tb.destroyed || tb.shouldBeDestroyed {
		panic(ErrDestroyedTokenBucket)
	}
}

// TryTake trys to task specified count tokens from the bucket. if there are
// not enough tokens in the bucket, it will return false.
func (tb *tokenBucket) TryTake(count int64) bool {
	scaledCount := ScaledTokenCount(count * TOKEN_BUCKET_CAPACITY_SCALE)
	return tb.tryTake(scaledCount, scaledCount)
}

// Take tasks specified count tokens from the bucket, if there are
// not enough tokens in the bucket, it will keep waiting until count tokens are
// available and then take them.
func (tb *tokenBucket) Take(count int64) {
	tb.waitAndTake(count, count)
}

func (tb *tokenBucket) GiveBack(count int64) {
	tb.lock.Lock()
	defer tb.lock.Unlock()

	tb.assertNotDestroyedNoLock()

	tb.available += ScaledTokenCount(count * TOKEN_BUCKET_CAPACITY_SCALE)
	tb.available = min(tb.capacity, tb.available)
}

func (tb *tokenBucket) PauseOneStateDecrementation() {
	tb.decrementingStateCount.Add(-1)
}

func (tb *tokenBucket) ResumeOneStateDecrementation() {
	tb.decrementingStateCount.Add(1)
}

// TakeMaxDuration tasks specified count tokens from the bucket, if there are
// not enough tokens in the bucket, it will keep waiting until count tokens are
// available and then take them or just return false when max time has been spent waiting.
func (tb *tokenBucket) TakeMaxDuration(count int64, max time.Duration) bool {
	return tb.waitAndTakeMaxDuration(count, count, max)
}

// Wait will keep waiting until count tokens are available in the bucket.
func (tb *tokenBucket) Wait(count int64) {
	tb.waitAndTake(count, 0)
}

// WaitMaxDuration will keep waiting until count tokens are available in the
// bucket or just return false when max time has been spent waiting.
func (tb *tokenBucket) WaitMaxDuration(count int64, max time.Duration) bool {
	return tb.waitAndTakeMaxDuration(count, 0, max)
}

func (tb *tokenBucket) tryTake(need, use ScaledTokenCount) bool {
	tb.checkCount(need)

	tb.lock.Lock()
	defer tb.lock.Unlock()

	tb.assertNotDestroyedNoLock()

	if need <= tb.available {
		tb.available -= use

		return true
	}

	return false
}

func (tb *tokenBucket) addWaitChannel(need, use ScaledTokenCount) chan (struct{}) {
	var channel chan (struct{})

	select {
	case chanFromPool := <-waitChanPool:
		channel = chanFromPool
	default:
		channel = make(chan struct{}, 1)
	}

	tb.chanListLock.Lock()
	tb.waitChans = append(tb.waitChans, channel)
	tb.neededTokenCountList = append(tb.neededTokenCountList, need)
	tb.useTokenCountList = append(tb.useTokenCountList, use)
	tb.chanListLock.Unlock()
	return channel
}

func (tb *tokenBucket) waitAndTake(need, use int64) {
	needCount := ScaledTokenCount(need * TOKEN_BUCKET_CAPACITY_SCALE)
	useCount := ScaledTokenCount(use * TOKEN_BUCKET_CAPACITY_SCALE)

	if ok := tb.tryTake(needCount, useCount); ok {
		return
	}

	waitChan := tb.addWaitChannel(needCount, useCount)
	<-waitChan
}

func (tb *tokenBucket) waitAndTakeMaxDuration(need, use int64, max time.Duration) bool {
	needCount := ScaledTokenCount(need * TOKEN_BUCKET_CAPACITY_SCALE)
	useCount := ScaledTokenCount(use * TOKEN_BUCKET_CAPACITY_SCALE)

	if ok := tb.tryTake(needCount, useCount); ok {
		return true
	}

	waitChan := tb.addWaitChannel(needCount, useCount)

	select {
	case <-waitChan:
		return true
	case <-time.After(max):
		return false
	}
}

func (tb *tokenBucket) Destroy() {
	tb.lock.Lock()
	defer tb.lock.Unlock()
	tb.shouldBeDestroyed = true
}

func (tb *tokenBucket) checkCount(count ScaledTokenCount) {
	if count < 0 || count > tb.capacity {
		panic(fmt.Sprintf("token-bucket: count %v should be less than bucket's"+
			" capacity %v", count, tb.capacity))
	}
}

// startTokenBucketManagerGoroutine starts a goroutine that manages all token buckets.
// The goroutine periodically iterates over tokenBuckets and performs several operations for each bucket:
// - add tokens in the bucket if .decrementFn field is nil.
// - cancel the attached context if there are not tokens left and cancelContextOnNegativeCount field is set to true.
// - for each goroutine waiting for the bucket to refill: if the needed token count > available then remove the tokens and resume the goroutine.
// - remove destroyed buckets from tokenBuckets.
func startTokenBucketManagerGoroutine() {
	if !tokenBucketManagerStarted.CompareAndSwap(false, true) {
		return
	}

	updateTokenCount := func(tb *tokenBucket) {
		tb.lock.Lock()
		defer tb.lock.Unlock()

		if tb.decrementFn == nil {
			if tb.available < tb.capacity {
				increment := tb.increment
				tb.available = tb.available + increment
			}
		} else if count := tb.decrementingStateCount.Load(); count > 0 {
			tb.available -= ScaledTokenCount(tb.decrementFn(tb.lastDecrementTime, count) * TOKEN_BUCKET_CAPACITY_SCALE)
		}

		if tb.available < 0 && tb.cancelContextOnNegativeCount && tb.context != nil {
			tb.context.CancelGracefully() // add reason
			return
		}

		tb.available = max(0, tb.available)
		tb.lastDecrementTime = time.Now() //updated even if tb.decrementFn is not called

		func() {
			tb.chanListLock.Lock()
			defer tb.chanListLock.Unlock()

			if tb.shouldBeDestroyed {
				tb.shouldBeDestroyed = false
				tb.destroyed = true
				delete(tokenBuckets, tb)

				// resume all waiting goroutines
				// TODO: make sure this could not be used to momentarily bypass the limits.
				for _, waitChan := range tb.waitChans {
					select {
					case waitChan <- struct{}{}:
					default:
					}

					// put back the wait channel in the pool if possible
					select {
					case waitChanPool <- waitChan:
					default:
						close(waitChan)
					}
				}
				return
			}

			for len(tb.waitChans) >= 1 { // if at least one goroutine is waiting for the bucket to refill.
				waitChan := tb.waitChans[len(tb.waitChans)-1]
				neededCount := tb.neededTokenCountList[len(tb.waitChans)-1]
				useCount := tb.useTokenCountList[len(tb.waitChans)-1]

				//if there are enough tokens we remove the needed count and resume the goroutine.
				if tb.available >= neededCount {
					newLength := len(tb.waitChans) - 1
					tb.waitChans = tb.waitChans[:newLength]
					tb.neededTokenCountList = tb.neededTokenCountList[:newLength]
					tb.useTokenCountList = tb.useTokenCountList[:newLength]

					tb.available -= useCount

					//resume the waiting goroutine
					select {
					case waitChan <- struct{}{}:
					default:
					}

					// put back the wait channel in the pool if possible
					select {
					case waitChanPool <- waitChan:
					default:
						close(waitChan)
					}
				} else {
					//not enough tokens.
					break
				}
			}
		}()
	}

	go func() {
		ticks := time.Tick(TOKEN_BUCKET_MANAGEMENT_TICK_INTERVAL)

		for range ticks {
			func() {
				tokenBucketsLock.Lock()
				defer utils.Recover()
				defer tokenBucketsLock.Unlock()

				for bucket := range tokenBuckets {
					updateTokenCount(bucket)
				}
			}()
		}

	}()
}
