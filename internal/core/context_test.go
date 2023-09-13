package core

import (
	"testing"
	"time"

	permkind "github.com/inoxlang/inox/internal/permkind"
	"github.com/stretchr/testify/assert"
)

func TestContextBuckets(t *testing.T) {

	t.Run("buckets for lim of kind 'total' do not fill over time", func(t *testing.T) {
		const LIMIT_NAME = "foo"
		ctx := NewContext(ContextConfig{
			Limits: []Limit{{Name: LIMIT_NAME, Kind: TotalLimit, Value: 1}},
		})

		ctx.Take(LIMIT_NAME, 1)

		//we check that the total has decreased
		total, err := ctx.GetTotal(LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)

		//we check that the total has not increased after a wait
		time.Sleep(2 * TOKEN_BUCKET_MANAGEMENT_TICK_INTERVAL)
		total, err = ctx.GetTotal(LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)

		//we check that the total has increased after we gave back tokens
		ctx.GiveBack(LIMIT_NAME, 1)
		total, err = ctx.GetTotal(LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), total)
	})
}

// func TestContextResourceManagement(t *testing.T) {

// 	t.Run("resources should be released after the context is cancelled", func(t *testing.T) {
// 		t.Run("no transaction", func(t *testing.T) {
// 			ResetResourceMap()

// 			ctx := NewContext(ContextConfig{})
// 			resource := URL("https://example.com/users/1")

// 			assert.NoError(t, ctx.AcquireResource(resource))
// 			ctx.Cancel()
// 			time.Sleep(10 * time.Millisecond)
// 			assert.True(t, TryAcquireConcreteResource(resource))
// 		})

// 		t.Run("transaction", func(t *testing.T) {
// 			ResetResourceMap()

// 			ctx := NewContext(ContextConfig{})
// 			StartNewTransaction(ctx)
// 			resource := URL("https://example.com/users/1")

// 			assert.NoError(t, ctx.AcquireResource(resource))
// 			ctx.Cancel()
// 			time.Sleep(10 * time.Millisecond)
// 			assert.True(t, TryAcquireConcreteResource(resource))
// 		})
// 	})
// }

func TestContextForbiddenPermissions(t *testing.T) {

	readGoFiles := FilesystemPermission{permkind.Read, PathPattern("./*.go")}
	readFile := FilesystemPermission{permkind.Read, Path("./file.go")}

	ctx := NewContext(ContextConfig{
		Permissions:          []Permission{readGoFiles},
		ForbiddenPermissions: []Permission{readFile},
	})

	assert.True(t, ctx.HasPermission(readGoFiles))
	assert.False(t, ctx.HasPermission(readFile))
}

func TestContextDropPermissions(t *testing.T) {
	readGoFiles := FilesystemPermission{permkind.Read, PathPattern("./*.go")}
	readFile := FilesystemPermission{permkind.Read, Path("./file.go")}

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
			Limits: []Limit{
				{Name: "fs/read", Kind: ByteRateLimit, Value: 1_000},
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
			Limits: []Limit{
				{Name: "fs/read-file", Kind: SimpleRateLimit, Value: 1},
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
			Limits: []Limit{
				{Name: "fs/total-read-file", Kind: TotalLimit, Value: 1},
			},
		})

		ctx.Take("fs/total-read-file", 1)

		assert.Panics(t, func() {
			ctx.Take("fs/total-read-file", 1)
		})
	})

	t.Run("auto decrement", func(t *testing.T) {
		ctx := NewContext(ContextConfig{
			Limits: []Limit{
				{
					Name:  "test",
					Kind:  TotalLimit,
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

	t.Run("child should share limiters of common limits with parent", func(t *testing.T) {
		parentCtx := NewContext(ContextConfig{
			Limits: []Limit{
				{Name: "fs/read", Kind: ByteRateLimit, Value: 1_000},
			},
		})
		ctx := NewContext(ContextConfig{
			Limits: []Limit{
				{Name: "fs/read", Kind: ByteRateLimit, Value: 1_000},
				{Name: "fs/write", Kind: ByteRateLimit, Value: 1_000},
			},
			ParentContext: parentCtx,
		})

		assert.Same(t, parentCtx.limiters["fs/read"], ctx.limiters["fs/read"])
		assert.NotSame(t, parentCtx.limiters["fs/write"], ctx.limiters["fs/write"])
	})
}

func TestContextSetProtocolClientForURLForURL(t *testing.T) {
	// const PROFILE_NAME = core.Identifier("myprofile")

	// ctx := core.NewContext(core.ContextConfig{
	// 	Permissions: []core.Permission{
	// 		permkind.Httpission{Kind_: permkind.Read, Entity: URL},
	// 	},
	// 	Limits: []core.Limit{},
	// })

	// assert.NoError(t, ctx.SetProtocolClientForURL(PROFILE_NAME, core.NewObject()))
	// profile, _ := ctx.GetProtolClient(PROFILE_NAME.UnderlyingString())
	// assert.NotNil(t, profile)
}
