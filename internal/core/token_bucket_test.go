package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenBucket(t *testing.T) {

	//TODO: add more tests.

	t.Run("TryTake", func(t *testing.T) {
		tb := newBucket(tokenBucketConfig{
			cap:                          100,
			initialAvail:                 100,
			fillRate:                     100,
			depleteFn:                    nil,
			cancelContextOnNegativeCount: false,
		})

		assert.True(t, tb.TryTake(10), "Should be able to take 10 tokens")
		assert.EqualValues(t, 90, tb.Available(), "Available tokens should be 90 after taking 10")
	})

	t.Run("Take", func(t *testing.T) {
		tb := newBucket(tokenBucketConfig{
			cap:                          100,
			initialAvail:                 100,
			fillRate:                     100,
			depleteFn:                    nil,
			cancelContextOnNegativeCount: false,
		})

		tb.Take(10)
		assert.EqualValues(t, 90, tb.Available(), "Available tokens should be 90 after taking 10")
	})

	t.Run("TakeMaxDuration", func(t *testing.T) {
		//TODO
	})

	t.Run("GiveBack", func(t *testing.T) {
		tb := newBucket(tokenBucketConfig{
			cap:                          100,
			initialAvail:                 100,
			fillRate:                     100,
			depleteFn:                    nil,
			cancelContextOnNegativeCount: false,
		})

		tb.Take(10)
		tb.GiveBack(5)
		assert.EqualValues(t, 95, tb.Available(), "Available tokens should be 95 after giving back 5")
	})

	t.Run("Wait", func(t *testing.T) {
		tb := newBucket(tokenBucketConfig{
			cap:                          100,
			initialAvail:                 100,
			fillRate:                     100,
			depleteFn:                    nil,
			cancelContextOnNegativeCount: false,
		})

		tb.Take(100)
		tb.Wait(10)
		assert.EqualValues(t, 10, tb.Available())
	})

	t.Run("WaitMaxDuration", func(t *testing.T) {
		//TODO
	})

	t.Run("Destroy", func(t *testing.T) {
		tb := newBucket(tokenBucketConfig{
			cap:                          100,
			initialAvail:                 100,
			fillRate:                     100,
			depleteFn:                    nil,
			cancelContextOnNegativeCount: false,
		})

		tb.Destroy()
		assert.PanicsWithError(t, ErrDestroyedTokenBucket.Error(), func() { tb.Take(10) })
	})
}
