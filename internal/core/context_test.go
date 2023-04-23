package internal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestContextBuckets(t *testing.T) {

	t.Run("buckets for limitations of kind 'total' do not fill over time", func(t *testing.T) {
		const LIMITATION_NAME = "foo"
		ctx := NewContext(ContextConfig{
			Limitations: []Limitation{{Name: LIMITATION_NAME, Kind: TotalLimitation, Value: 1}},
		})

		ctx.Take(LIMITATION_NAME, 1)

		//we check that the total has decreased
		total, err := ctx.GetTotal(LIMITATION_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)

		//we check that the total has not increased after a wait
		time.Sleep(2 * TOKEN_BUCKET_MANAGEMENT_TICK_INTERVAL)
		total, err = ctx.GetTotal(LIMITATION_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)

		//we check that the total has increased after we gave back tokens
		ctx.GiveBack(LIMITATION_NAME, 1)
		total, err = ctx.GetTotal(LIMITATION_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), total)
	})
}

func TestContextResourceManagement(t *testing.T) {

	t.Run("resources should be released after the context is cancelled", func(t *testing.T) {
		t.Run("no transaction", func(t *testing.T) {
			ResetResourceMap()

			ctx := NewContext(ContextConfig{})
			resource := URL("https://example.com/users/1")

			assert.NoError(t, ctx.AcquireResource(resource))
			ctx.Cancel()
			time.Sleep(10 * time.Millisecond)
			assert.True(t, TryAcquireResource(resource))
		})

		t.Run("transaction", func(t *testing.T) {
			ResetResourceMap()

			ctx := NewContext(ContextConfig{})
			StartNewTransaction(ctx)
			resource := URL("https://example.com/users/1")

			assert.NoError(t, ctx.AcquireResource(resource))
			ctx.Cancel()
			time.Sleep(10 * time.Millisecond)
			assert.True(t, TryAcquireResource(resource))
		})
	})
}

func TestContextForbiddenPermissions(t *testing.T) {

	readGoFiles := FilesystemPermission{ReadPerm, PathPattern("./*.go")}
	readFile := FilesystemPermission{ReadPerm, Path("./file.go")}

	ctx := NewContext(ContextConfig{
		Permissions:          []Permission{readGoFiles},
		ForbiddenPermissions: []Permission{readFile},
	})

	assert.True(t, ctx.HasPermission(readGoFiles))
	assert.False(t, ctx.HasPermission(readFile))
}

func TestContextDropPermissions(t *testing.T) {
	readGoFiles := FilesystemPermission{ReadPerm, PathPattern("./*.go")}
	readFile := FilesystemPermission{ReadPerm, Path("./file.go")}

	ctx := NewContext(ContextConfig{
		Permissions:          []Permission{readGoFiles},
		ForbiddenPermissions: []Permission{readFile},
	})

	ctx.DropPermissions([]Permission{readGoFiles})

	assert.False(t, ctx.HasPermission(readGoFiles))
	assert.False(t, ctx.HasPermission(readFile))
}

func TestContextLimiters(t *testing.T) {

	t.Run("byte rate", func(t *testing.T) {
		ctx := NewContext(ContextConfig{
			Limitations: []Limitation{
				{Name: "fs/read", Kind: ByteRateLimitation, Value: 1_000},
			},
		})

		start := time.Now()

		//BYTE RATE

		//should not cause a wait
		ctx.Take("fs/read", 1_000)
		assert.WithinDuration(t, start, time.Now(), time.Millisecond)

		expectedTime := time.Now().Add(time.Second)

		//should cause a wait
		ctx.Take("fs/read", 1_000)
		assert.WithinDuration(t, expectedTime, time.Now(), 200*time.Millisecond)
	})

	t.Run("simple rate", func(t *testing.T) {
		ctx := NewContext(ContextConfig{
			Limitations: []Limitation{
				{Name: "fs/read-file", Kind: SimpleRateLimitation, Value: 1},
			},
		})

		start := time.Now()
		expectedTime := start.Add(time.Second)

		ctx.Take("fs/read-file", 1)
		assert.WithinDuration(t, start, time.Now(), time.Millisecond)

		//should cause a wait
		ctx.Take("fs/read-file", 1)
		assert.WithinDuration(t, expectedTime, time.Now(), 200*time.Millisecond)
	})

	t.Run("total", func(t *testing.T) {
		ctx := NewContext(ContextConfig{
			Limitations: []Limitation{
				{Name: "fs/total-read-file", Kind: TotalLimitation, Value: 1},
			},
		})

		ctx.Take("fs/total-read-file", 1)

		assert.Panics(t, func() {
			ctx.Take("fs/total-read-file", 1)
		})
	})

	t.Run("auto decrement", func(t *testing.T) {
		ctx := NewContext(ContextConfig{
			Limitations: []Limitation{
				{
					Name:  "test",
					Kind:  TotalLimitation,
					Value: int64(time.Second),
					DecrementFn: func(lastDecrementTime time.Time) int64 {
						return time.Since(lastDecrementTime).Nanoseconds()
					},
				},
			},
		})

		capacity := int64(time.Second)

		assert.Equal(t, capacity, ctx.limiters["test"].bucket.Available())
		time.Sleep(time.Second)
		assert.InDelta(t, int64(0), ctx.limiters["test"].bucket.Available(), float64(capacity/20))
	})

}

func TestContextSetProtocolClientForURLForURL(t *testing.T) {
	// const PROFILE_NAME = core.Identifier("myprofile")

	// ctx := core.NewContext(core.ContextConfig{
	// 	Permissions: []core.Permission{
	// 		core.HttpPermission{Kind_: core.ReadPerm, Entity: URL},
	// 	},
	// 	Limitations: []core.Limitation{},
	// })

	// assert.NoError(t, ctx.SetProtocolClientForURL(PROFILE_NAME, core.NewObject()))
	// profile, _ := ctx.GetProtolClient(PROFILE_NAME.UnderlyingString())
	// assert.NotNil(t, profile)
}
