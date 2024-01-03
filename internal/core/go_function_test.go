package core

import "testing"

func BenchmarkGoFunctionCallSingleArgSingleRet(b *testing.B) {

	goFunc := WrapGoFunction(func(a Int) Int {
		return 9
	})

	ctx := NewContext(ContextConfig{})
	defer ctx.CancelGracefully()
	state := NewGlobalState(ctx)

	_, err := goFunc.Call([]any{Int(1)}, state, nil, false, false)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		goFunc.Call([]any{Int(1)}, state, nil, false, false)
	}
}

func BenchmarkGoFunctionCallCtxAndSingleArgSingleRet(b *testing.B) {
	goFunc := WrapGoFunction(func(ctx *Context, a Int) Int {
		return 9
	})

	ctx := NewContext(ContextConfig{})
	defer ctx.CancelGracefully()
	state := NewGlobalState(ctx)

	_, err := goFunc.Call([]any{Int(1)}, state, nil, false, false)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		goFunc.Call([]any{Int(1)}, state, nil, false, false)
	}
}

func BenchmarkGoFunctionCallCtxAndTwoArgsSingleRet(b *testing.B) {
	goFunc := WrapGoFunction(func(ctx *Context, a, b Int) Int {
		return 9
	})

	ctx := NewContext(ContextConfig{})
	defer ctx.CancelGracefully()
	state := NewGlobalState(ctx)

	_, err := goFunc.Call([]any{Int(1), Int(2)}, state, nil, false, false)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		goFunc.Call([]any{Int(1), Int(2)}, state, nil, false, false)
	}
}
