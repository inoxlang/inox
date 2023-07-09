package core

import (
	"sync"
	"sync/atomic"
	"time"
)

// A PeriodicWatcher is Watcher that periodically checks if it has a value.
type PeriodicWatcher struct {
	NotClonableMixin

	config WatcherConfiguration
	next   Value
	period time.Duration
	tick   chan struct{}

	lock    sync.Mutex
	waiting atomic.Bool
	stopped atomic.Bool
}

func NewPeriodicWatcher(config WatcherConfiguration, period time.Duration) *PeriodicWatcher {
	w := &PeriodicWatcher{
		config: config,
		period: period,
	}

	spawnPeriodicWatcherGoroutine()

	// TODO: simplify
	subscriptionAckWaitGroupLock.Lock()
	defer subscriptionAckWaitGroupLock.Unlock()
	subscriptionAckWaitGroup.Wait()
	subscriptionAckWaitGroup.Add(1)
	periodicWatcherSubscribeChan <- w
	subscriptionAckWaitGroup.Wait() //we wait for the goroutine managing periodic watcher to set .tick.

	return w
}

// InformAboutAsync sets the next value the watcher will "see", on a tick the watcher only sees the last value set.
func (w *PeriodicWatcher) InformAboutAsync(ctx *Context, v Value) {
	w.lock.Lock()
	defer w.lock.Unlock()

	w.next = v
}

func (w *PeriodicWatcher) WaitNext(ctx *Context, additionalFilter Pattern, timeout time.Duration) (Value, error) {
	stopped := w.stopped.Load()
	if stopped {
		return nil, ErrStoppedWatcher
	}

	start := time.Now()

	for {
		w.lock.Lock()
		tick := w.tick
		w.lock.Unlock()

		if tick == nil {
			return nil, ErrStoppedWatcher
		}

		w.waiting.Store(true)
		select {
		case <-tick:
			w.waiting.Store(false)
		case <-ctx.Done():
			w.waiting.Store(false)
			w.Stop()
			return nil, ErrStoppedWatcher
		}

		if w.stopped.Load() {
			return nil, ErrStoppedWatcher
		}

		// if there is a value matching the filters we return it
		w.lock.Lock()
		if w.next != nil {
			next := w.next
			w.next = nil

			if w.config.Filter.Test(ctx, next) && (additionalFilter == nil || additionalFilter.Test(ctx, next)) {
				w.lock.Unlock()
				return next, nil
			}
		}
		w.lock.Unlock()

		if time.Since(start) >= timeout {
			return nil, ErrWatchTimeout
		}

		//if there is no value we wait again
	}
}

func (w *PeriodicWatcher) Stop() {
	if w.stopped.CompareAndSwap(false, true) {
		periodicWatcherUnsuscribeChan <- w
	}
}

func (w *PeriodicWatcher) IsStopped() bool {
	return w.stopped.Load()
}

func (w *PeriodicWatcher) Config() WatcherConfiguration {
	return w.config
}

func spawnPeriodicWatcherGoroutine() {

	if !periodicWatcherGoroutineStarted.CompareAndSwap(false, true) {
		return
	}

	go func() {
		lastWatchMoments := make(map[*PeriodicWatcher]time.Time)
		tickChannels := map[chan struct{}]bool{} //true if available
		ticks := time.Tick(PERIODIC_WATCHER_GOROUTINE_TICK_INTERVAL)

		for {
			select {
			case w := <-periodicWatcherSubscribeChan:
				lastWatchMoments[w] = time.Now()

				//find or create an available channel for the watcher
				for channel, available := range tickChannels {
					if available {
						tickChannels[channel] = false
						w.lock.Lock()
						w.tick = channel
						w.lock.Unlock()
						break
					}
				}

				w.lock.Lock()
				if w.tick == nil {
					channel := make(chan struct{}, 1)
					tickChannels[channel] = false //not available
					w.tick = channel
				}
				w.lock.Unlock()
				subscriptionAckWaitGroup.Done()

			case w := <-periodicWatcherUnsuscribeChan:
				delete(lastWatchMoments, w)
				w.lock.Lock()
				tick := w.tick
				w.tick = nil
				w.lock.Unlock()
				if w.waiting.Load() {
					tick <- struct{}{}
				}

				// mark channel as available
				tickChannels[w.tick] = true
			case now := <-ticks:

				// we iterate over the watchers and we write to the tick channel of watchers having their period ellapsed
				for watcher, lastWatchMoment := range lastWatchMoments {

					if now.Sub(lastWatchMoment) >= watcher.period {
						lastWatchMoments[watcher] = now
						if len(watcher.tick) == 0 {
							watcher.tick <- struct{}{}
						}
					}
				}
			}
		}
	}()
}

func WatchReceivedMessages(ctx *Context, v Watchable) Watcher {
	return v.Watcher(ctx, WatcherConfiguration{
		Filter: MSG_PATTERN,
		Depth:  ShallowWatching,
	})
}
