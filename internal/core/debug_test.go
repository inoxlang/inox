package core_test

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/inoxmod"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestTreeWalkDebug(t *testing.T) {
	//TODO: add test with included chunks

	testDebugModeEval(t, func(code string, opts ...debugTestOptions) (any, *core.Context, *parse.ParsedChunkSource, *core.Debugger) {
		state := core.NewGlobalState(NewDefaultTestContext())
		treeWalkState := core.NewTreeWalkStateWithGlobal(state)

		chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "core-test",
			CodeString: code,
		}))

		nextBreakPointId := int32(core.INITIAL_BREAKPOINT_ID)
		var options debugTestOptions
		if len(opts) == 1 {
			options = opts[0]
		}

		breakpoints, err := core.GetBreakpointsFromLines(options.breakpointLines, chunk, &nextBreakPointId)
		if !assert.NoError(t, err) {
			assert.Fail(t, "failed to get breakpoints from lines "+err.Error())
		}

		debugger := core.NewDebugger(core.DebuggerArgs{
			Logger:                zerolog.New(io.Discard),
			InitialBreakpoints:    breakpoints,
			ExceptionBreakpointId: options.exceptionBreakpointId,
		})

		state.Module = core.WrapLowerModule(&inoxmod.Module{MainChunk: chunk, TopLevelNode: chunk.Node})
		debugger.AttachAndStart(treeWalkState)

		return treeWalkState, state.Ctx, chunk, debugger
	}, func(n parse.Node, state any) (core.Value, error) {
		result, err := core.TreeWalkEval(n, state.(*core.TreeWalkState))

		return result, err
	})
}

type debugTestOptions struct {
	breakpointLines       []int
	exceptionBreakpointId int32
}

func testDebugModeEval(
	t *testing.T,
	setup func(code string, opts ...debugTestOptions) (any, *core.Context, *parse.ParsedChunkSource, *core.Debugger),
	eval func(n parse.Node, state any) (core.Value, error),
) {

	t.Run("single instruction module", func(t *testing.T) {
		t.Run("breakpoint", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`1`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.CancelGracefully()

			controlChan <- core.DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					chunk.Node.Statements[0]: {}, //1
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []core.ProgramStoppedEvent
			var globalScopes []map[string]core.Value
			var localScopes []map[string]core.Value
			var stackTraces [][]core.StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandContinue{ThreadId: debugger.ThreadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Int(1), result)

			assert.Equal(t, []core.ProgramStoppedEvent{
				{Reason: core.BreakpointStop, ThreadId: debugger.ThreadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]core.Value{{}}, globalScopes)

			assert.Equal(t, []map[string]core.Value{{}}, localScopes)

			assert.Equal(t, [][]core.StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[0],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   1,
						StatementStartColumn: 1,
					},
				},
			}, stackTraces)
		})

	})

	t.Run("shallow", func(t *testing.T) {

		t.Run("successive breakpoints", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`
				a = 1
				a = 2
				a = 3
				return a
			`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.CancelGracefully()

			controlChan <- core.DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					chunk.Node.Statements[1]: {}, //a = 2
					chunk.Node.Statements[2]: {}, //a = 3
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []core.ProgramStoppedEvent
			var globalScopes []map[string]core.Value
			var localScopes []map[string]core.Value
			var stackTraces [][]core.StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 2'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandContinue{ThreadId: debugger.ThreadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandContinue{ThreadId: debugger.ThreadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Int(3), result)

			assert.Equal(t, []core.ProgramStoppedEvent{
				{Reason: core.BreakpointStop, ThreadId: debugger.ThreadId()},
				{Reason: core.BreakpointStop, ThreadId: debugger.ThreadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]core.Value{{}, {}}, globalScopes)

			assert.Equal(t, []map[string]core.Value{
				{"a": core.Int(1)}, {"a": core.Int(2)},
			}, localScopes)

			assert.Equal(t, [][]core.StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[1],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   3,
						StatementStartColumn: 5,
					},
				},
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[2],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   4,
						StatementStartColumn: 5,
					},
				},
			}, stackTraces)
		})

		t.Run("successive breakpoints set by line", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(
				`a = 1
				a = 2
				a = 3
				return a
			`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.CancelGracefully()

			controlChan <- core.DebugCommandSetBreakpoints{
				Chunk:             chunk,
				BreakPointsByLine: []int{2, 3}, //a = 2 & a = 3
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []core.ProgramStoppedEvent
			var globalScopes []map[string]core.Value
			var localScopes []map[string]core.Value
			var stackTraces [][]core.StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 2'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandContinue{ThreadId: debugger.ThreadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandContinue{ThreadId: debugger.ThreadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Int(3), result)

			assert.Equal(t, []core.ProgramStoppedEvent{
				{Reason: core.BreakpointStop, ThreadId: debugger.ThreadId()},
				{Reason: core.BreakpointStop, ThreadId: debugger.ThreadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]core.Value{{}, {}}, globalScopes)

			assert.Equal(t, []map[string]core.Value{
				{"a": core.Int(1)}, {"a": core.Int(2)},
			}, localScopes)

			assert.Equal(t, [][]core.StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[1],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   2,
						StatementStartColumn: 5,
					},
				},
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[2],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   3,
						StatementStartColumn: 5,
					},
				},
			}, stackTraces)
		})

		t.Run("successive breakpoints set by line with equal but not same chunk", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(
				`a = 1
				a = 2
				a = 3
				return a
			`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.CancelGracefully()

			equalChunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
				NameString: "core-test",
				CodeString: chunk.Source.Code(),
			}))

			controlChan <- core.DebugCommandSetBreakpoints{
				Chunk:             equalChunk,
				BreakPointsByLine: []int{2, 3}, //a = 2 & a = 3
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []core.ProgramStoppedEvent
			var globalScopes []map[string]core.Value
			var localScopes []map[string]core.Value
			var stackTraces [][]core.StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 2'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandContinue{ThreadId: debugger.ThreadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandContinue{ThreadId: debugger.ThreadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Int(3), result)

			assert.Equal(t, []core.ProgramStoppedEvent{
				{Reason: core.BreakpointStop, ThreadId: debugger.ThreadId()},
				{Reason: core.BreakpointStop, ThreadId: debugger.ThreadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]core.Value{{}, {}}, globalScopes)

			assert.Equal(t, []map[string]core.Value{
				{"a": core.Int(1)}, {"a": core.Int(2)},
			}, localScopes)

			assert.Equal(t, [][]core.StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[1],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   2,
						StatementStartColumn: 5,
					},
				},
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[2],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   3,
						StatementStartColumn: 5,
					},
				},
			}, stackTraces)
		})

		t.Run("breakpoint set on empty line", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(
				`a = 1

				return 3
			`)

			controlChan := debugger.ControlChan()

			defer ctx.CancelGracefully()

			breakpointsChan := make(chan []core.BreakpointInfo)

			controlChan <- core.DebugCommandSetBreakpoints{
				Chunk:             chunk,
				BreakPointsByLine: []int{2},
				GetBreakpointsSetByLine: func(breakpoints []core.BreakpointInfo) {
					breakpointsChan <- breakpoints
				},
			}

			breakpoints := <-breakpointsChan
			assert.Equal(t, []core.BreakpointInfo{
				{
					Chunk: chunk,
					Id:    core.INITIAL_BREAKPOINT_ID,
				},
			}, breakpoints)

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Int(3), result)
		})

		t.Run("successive breakpoints set by line during initialization", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(
				`a = 1
				a = 2
				a = 3
				return a
			`, debugTestOptions{
					breakpointLines: []int{2, 3}, //a = 2 & a = 3
				})

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.CancelGracefully()

			var stoppedEvents []core.ProgramStoppedEvent
			var globalScopes []map[string]core.Value
			var localScopes []map[string]core.Value
			var stackTraces [][]core.StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 2'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandContinue{ThreadId: debugger.ThreadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandContinue{ThreadId: debugger.ThreadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Int(3), result)

			assert.Equal(t, []core.ProgramStoppedEvent{
				{Reason: core.BreakpointStop, ThreadId: debugger.ThreadId()},
				{Reason: core.BreakpointStop, ThreadId: debugger.ThreadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]core.Value{{}, {}}, globalScopes)

			assert.Equal(t, []map[string]core.Value{
				{"a": core.Int(1)}, {"a": core.Int(2)},
			}, localScopes)

			assert.Equal(t, [][]core.StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[1],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   2,
						StatementStartColumn: 5,
					},
				},
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[2],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   3,
						StatementStartColumn: 5,
					},
				},
			}, stackTraces)
		})

		t.Run("breakpoint & two steps", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`
				a = 1
				a = 2
				a = 3
				return a
			`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.CancelGracefully()

			controlChan <- core.DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					chunk.Node.Statements[0]: {}, //a = 1
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []core.ProgramStoppedEvent
			var globalScopes []map[string]core.Value
			var localScopes []map[string]core.Value
			var stackTraces [][]core.StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				controlChan <- core.DebugCommandNextStep{ThreadId: debugger.ThreadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 2'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandNextStep{ThreadId: debugger.ThreadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandContinue{ThreadId: debugger.ThreadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Int(3), result)

			assert.Equal(t, []core.ProgramStoppedEvent{
				{Reason: core.BreakpointStop, ThreadId: debugger.ThreadId()},
				{Reason: core.NextStepStop, ThreadId: debugger.ThreadId()},
				{Reason: core.NextStepStop, ThreadId: debugger.ThreadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]core.Value{{}, {}}, globalScopes)

			assert.Equal(t, []map[string]core.Value{
				{"a": core.Int(1)}, {"a": core.Int(2)},
			}, localScopes)

			assert.Equal(t, [][]core.StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[1],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   3,
						StatementStartColumn: 5,
					},
				},
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[2],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   4,
						StatementStartColumn: 5,
					},
				},
			}, stackTraces)
		})

		t.Run("breakpoint & two steps in", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`
				a = 1
				a = 2
				a = 3
				return a
			`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.CancelGracefully()

			controlChan <- core.DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					chunk.Node.Statements[0]: {}, //a = 1
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []core.ProgramStoppedEvent
			var globalScopes []map[string]core.Value
			var localScopes []map[string]core.Value
			var stackTraces [][]core.StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				controlChan <- core.DebugCommandStepIn{
					ThreadId: debugger.ThreadId(),
				}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 2'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandStepIn{ThreadId: debugger.ThreadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandContinue{ThreadId: debugger.ThreadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Int(3), result)

			if !assert.Equal(t, []core.ProgramStoppedEvent{
				{Reason: core.BreakpointStop, ThreadId: debugger.ThreadId()},
				{Reason: core.StepInStop, ThreadId: debugger.ThreadId()},
				{Reason: core.StepInStop, ThreadId: debugger.ThreadId()},
			}, stoppedEvents) {
				return
			}

			assert.Equal(t, []map[string]core.Value{
				{"a": core.Int(1)}, {"a": core.Int(2)},
			}, localScopes)

			assert.Equal(t, [][]core.StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[1],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   3,
						StatementStartColumn: 5,
					},
				},
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[2],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   4,
						StatementStartColumn: 5,
					},
				},
			}, stackTraces)
		})

		t.Run("exceptions breakpoints set", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`
				a = 1
				overflow = (10_000_000_000 * 10_000_000_000)
				return a
			`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.CancelGracefully()

			var breakpointId int32

			controlChan <- core.DebugCommandSetExceptionBreakpoints{
				GetExceptionBreakpointId: func(id int32) {
					breakpointId = id
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []core.ProgramStoppedEvent
			var globalScopes []map[string]core.Value
			var localScopes []map[string]core.Value
			var stackTraces [][]core.StackFrameInfo

			go func() {
				event := <-stoppedChan

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'overflow = ...'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'overflow = ...'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandContinue{
					ThreadId: debugger.ThreadId(),
				}
			}()

			_, err := eval(chunk.Node, state)

			if !assert.Error(t, err) {
				return
			}

			if !assert.Len(t, stoppedEvents, 1) {
				return
			}

			event := stoppedEvents[0]
			assert.Equal(t, core.ExceptionBreakpointStop, event.Reason)
			assert.ErrorIs(t, event.ExceptionError, core.ErrIntOverflow)
			assert.Equal(t, breakpointId, event.Breakpoint.Id)

			assert.Equal(t, []map[string]core.Value{{}}, globalScopes)

			assert.Equal(t, []map[string]core.Value{
				{"a": core.Int(1)},
			}, localScopes)

			assert.Equal(t, [][]core.StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[1],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   3,
						StatementStartColumn: 5,
					},
				},
			}, stackTraces)
		})

		t.Run("exceptions breakpoints set during initialization", func(t *testing.T) {
			exceptionBreakpointId := int32(core.INITIAL_BREAKPOINT_ID + 3)
			state, ctx, chunk, debugger := setup(`
				a = 1
				overflow = (10_000_000_000 * 10_000_000_000)
				return a
			`, debugTestOptions{exceptionBreakpointId: exceptionBreakpointId})

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.CancelGracefully()

			var stoppedEvents []core.ProgramStoppedEvent
			var globalScopes []map[string]core.Value
			var localScopes []map[string]core.Value
			var stackTraces [][]core.StackFrameInfo

			go func() {
				event := <-stoppedChan

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'overflow = ...'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'overflow = ...'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandContinue{ThreadId: debugger.ThreadId()}
			}()

			_, err := eval(chunk.Node, state)

			if !assert.Error(t, err) {
				return
			}

			if !assert.Len(t, stoppedEvents, 1) {
				return
			}

			event := stoppedEvents[0]
			assert.Equal(t, core.ExceptionBreakpointStop, event.Reason)
			assert.ErrorIs(t, event.ExceptionError, core.ErrIntOverflow)
			assert.Equal(t, exceptionBreakpointId, event.Breakpoint.Id)

			assert.Equal(t, []map[string]core.Value{{}}, globalScopes)

			assert.Equal(t, []map[string]core.Value{
				{"a": core.Int(1)},
			}, localScopes)

			assert.Equal(t, [][]core.StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[1],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   3,
						StatementStartColumn: 5,
					},
				},
			}, stackTraces)
		})

		t.Run("pause", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`
				a = 1
				sleep 0.3s
				a = 2
				return a
			`)

			global := core.WrapGoFunction(core.Sleep)
			ctx.MustGetClosestState().Globals.Set("sleep", global)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.CancelGracefully()

			var stoppedEvents []core.ProgramStoppedEvent
			var globalScopes []map[string]core.Value
			var localScopes []map[string]core.Value
			var stackTraces [][]core.StackFrameInfo

			go func() {
				//wait to make sure the pause command will be sent during the sleep(0.3s) call
				time.Sleep(10 * time.Millisecond)

				controlChan <- core.DebugCommandPause{
					ThreadId: debugger.ThreadId(),
				}

				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 2'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandContinue{ThreadId: debugger.ThreadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Int(2), result)

			assert.Equal(t, []core.ProgramStoppedEvent{
				{Reason: core.PauseStop, ThreadId: debugger.ThreadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]core.Value{
				{"sleep": global},
			}, globalScopes)

			assert.Equal(t, []map[string]core.Value{
				{"a": core.Int(1)},
			}, localScopes)

			assert.Equal(t, [][]core.StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[2],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   4,
						StatementStartColumn: 5,
					},
				},
			}, stackTraces)
		})

		t.Run("close debugger while program stopped at breakpoint", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`
				a = 1
				a = 2
				return a
			`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.CancelGracefully()

			controlChan <- core.DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					chunk.Node.Statements[0]: {}, //a = 1

					//this breakpoint should be ignored because the debugger should be closed when it is reached
					chunk.Node.Statements[1]: {}, //a = 2
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			go func() {
				<-stoppedChan
				//a = 1

				controlChan <- core.DebugCommandCloseDebugger{}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Int(2), result)
		})
	})

	t.Run("function call", func(t *testing.T) {

		t.Run("breakpoint & two steps in function call (step after return)", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`
				fn f(a){
					b = 3
					return b
				}
				result = f(2)
				return result
			`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.CancelGracefully()

			assignments := parse.FindNodes(chunk.Node, (*parse.Assignment)(nil), nil)
			returnStmts := parse.FindNodes(chunk.Node, (*parse.ReturnStatement)(nil), nil)

			controlChan <- core.DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					assignments[0]: {}, //b = 3
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []core.ProgramStoppedEvent
			var globalScopes []map[string]core.Value
			var localScopes []map[string]core.Value
			var stackTraces [][]core.StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				controlChan <- core.DebugCommandNextStep{ThreadId: debugger.ThreadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'return b'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'return b'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandNextStep{ThreadId: debugger.ThreadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'return result'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'return result'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandContinue{ThreadId: debugger.ThreadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Int(3), result)

			assert.Equal(t, []core.ProgramStoppedEvent{
				{Reason: core.BreakpointStop, ThreadId: debugger.ThreadId()},
				{Reason: core.NextStepStop, ThreadId: debugger.ThreadId()},
				{Reason: core.NextStepStop, ThreadId: debugger.ThreadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]core.Value{
				{"a": core.Int(2), "b": core.Int(3)},
				{"result": core.Int(3)},
			}, localScopes)

			assert.Equal(t, [][]core.StackFrameInfo{
				{
					{
						Name:                 "(fn) core-test:2:5:",
						Node:                 returnStmts[0],
						Chunk:                chunk,
						Id:                   2,
						StartLine:            2,
						StartColumn:          5,
						StatementStartLine:   4,
						StatementStartColumn: 6,
					},
					{
						Name:                 "core-test",
						Node:                 assignments[1],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   6,
						StatementStartColumn: 5,
					},
				},
				{
					{
						Name:                 "core-test",
						Node:                 returnStmts[1],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   7,
						StatementStartColumn: 5,
					},
				},
			}, stackTraces)
		})

		t.Run("breakpoint & step over function call", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`
				fn f(a){
					b = 3
					return b
				}
				result = f(2)
				return result
			`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.CancelGracefully()

			returnStmts := parse.FindNodes(chunk.Node, (*parse.ReturnStatement)(nil), nil)

			controlChan <- core.DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					chunk.Node.Statements[1]: {}, //result = f(2)
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []core.ProgramStoppedEvent
			var globalScopes []map[string]core.Value
			var localScopes []map[string]core.Value
			var stackTraces [][]core.StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				controlChan <- core.DebugCommandNextStep{ThreadId: debugger.ThreadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'return result'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'return result'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandContinue{ThreadId: debugger.ThreadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Int(3), result)

			assert.Equal(t, []core.ProgramStoppedEvent{
				{Reason: core.BreakpointStop, ThreadId: debugger.ThreadId()},
				{Reason: core.NextStepStop, ThreadId: debugger.ThreadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]core.Value{{"result": core.Int(3)}}, localScopes)

			assert.Equal(t, [][]core.StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 returnStmts[1],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   7,
						StatementStartColumn: 5,
					},
				},
			}, stackTraces)
		})

		t.Run("breakpoint & step in function call", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`
				fn f(a){
					b = 3
					return b
				}
				result = f(2)
				return result
			`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.CancelGracefully()

			controlChan <- core.DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					chunk.Node.Statements[1]: {}, //result = f()
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []core.ProgramStoppedEvent
			var globalScopes []map[string]core.Value
			var localScopes []map[string]core.Value
			var stackTraces [][]core.StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				controlChan <- core.DebugCommandStepIn{ThreadId: debugger.ThreadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 1'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'a = 1'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandContinue{ThreadId: debugger.ThreadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Int(3), result)

			assert.Equal(t, []core.ProgramStoppedEvent{
				{Reason: core.BreakpointStop, ThreadId: debugger.ThreadId()},
				{Reason: core.StepInStop, ThreadId: debugger.ThreadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]core.Value{{"a": core.Int(2)}}, localScopes)

			assignments := parse.FindNodes(chunk.Node, (*parse.Assignment)(nil), nil)

			assert.Equal(t, [][]core.StackFrameInfo{
				{
					{
						Name:                 "(fn) core-test:2:5:",
						Node:                 assignments[0],
						Chunk:                chunk,
						Id:                   2,
						StartLine:            2,
						StartColumn:          5,
						StatementStartLine:   3,
						StatementStartColumn: 6,
					},
					{
						Name:                 "core-test",
						Node:                 assignments[1],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   6,
						StatementStartColumn: 5,
					},
				},
			}, stackTraces)
		})

		t.Run("breakpoint & step in function call then step out", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`
				fn f(a){
					b = 3
					return b
				}
				result = f(2)
				return result
			`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.CancelGracefully()

			returnStmts := parse.FindNodes(chunk.Node, (*parse.ReturnStatement)(nil), nil)

			controlChan <- core.DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					chunk.Node.Statements[1]: {}, //result = f()
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []core.ProgramStoppedEvent
			var globalScopes []map[string]core.Value
			var localScopes []map[string]core.Value
			var stackTraces [][]core.StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				controlChan <- core.DebugCommandStepIn{ThreadId: debugger.ThreadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'b = 3'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'b = 3'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandStepOut{ThreadId: debugger.ThreadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'return result'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'return result'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandContinue{ThreadId: debugger.ThreadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Int(3), result)

			assert.Equal(t, []core.ProgramStoppedEvent{
				{Reason: core.BreakpointStop, ThreadId: debugger.ThreadId()},
				{Reason: core.StepInStop, ThreadId: debugger.ThreadId()},
				{Reason: core.StepOutStop, ThreadId: debugger.ThreadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]core.Value{
				{"a": core.Int(2)},
				{"result": core.Int(3)},
			}, localScopes)

			assignments := parse.FindNodes(chunk.Node, (*parse.Assignment)(nil), nil)

			assert.Equal(t, [][]core.StackFrameInfo{
				{
					{
						Name:                 "(fn) core-test:2:5:",
						Node:                 assignments[0],
						Chunk:                chunk,
						Id:                   2,
						StartLine:            2,
						StartColumn:          5,
						StatementStartLine:   3,
						StatementStartColumn: 6,
					},
					{
						Name:                 "core-test",
						Node:                 assignments[1],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   6,
						StatementStartColumn: 5,
					},
				},
				{
					{
						Name:                 "core-test",
						Node:                 returnStmts[1],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   7,
						StatementStartColumn: 5,
					},
				},
			}, stackTraces)
		})
	})

	t.Run("function call within function call", func(t *testing.T) {

		t.Run("breakpoint & step in deepest function call", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`
				fn g(x){
					a = 3
					return a
				}
				fn f(a){
					b = g(a)
					return b
				}
				result = f(2)
				return result
			`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.CancelGracefully()

			assignments := parse.FindNodes(chunk.Node, (*parse.Assignment)(nil), nil)

			controlChan <- core.DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					assignments[1]: {}, //b = g()
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []core.ProgramStoppedEvent
			var globalScopes []map[string]core.Value
			var localScopes []map[string]core.Value
			var stackTraces [][]core.StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				controlChan <- core.DebugCommandStepIn{ThreadId: debugger.ThreadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandContinue{ThreadId: debugger.ThreadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Int(3), result)

			assert.Equal(t, []core.ProgramStoppedEvent{
				{Reason: core.BreakpointStop, ThreadId: debugger.ThreadId()},
				{Reason: core.StepInStop, ThreadId: debugger.ThreadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]core.Value{
				{"x": core.Int(2)},
			}, localScopes)

			assert.Equal(t, [][]core.StackFrameInfo{
				{
					{
						Name:                 "(fn) core-test:2:5:",
						Node:                 assignments[0],
						Chunk:                chunk,
						Id:                   3,
						StartLine:            2,
						StartColumn:          5,
						StatementStartLine:   3,
						StatementStartColumn: 6,
					},
					{
						Name:                 "(fn) core-test:6:5:",
						Node:                 assignments[1],
						Chunk:                chunk,
						Id:                   2,
						StartLine:            6,
						StartColumn:          5,
						StatementStartLine:   7,
						StatementStartColumn: 6,
					},
					{
						Name:                 "core-test",
						Node:                 assignments[2],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   10,
						StatementStartColumn: 5,
					},
				},
			}, stackTraces)
		})

		t.Run("breakpoint & step in deepest function call then step out", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`
				fn g(x){
					a = 3
					return a
				}
				fn f(a){
					b = g(a)
					return b
				}
				result = f(2)
				return result
			`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.CancelGracefully()

			assignments := parse.FindNodes(chunk.Node, (*parse.Assignment)(nil), nil)
			returnStmts := parse.FindNodes(chunk.Node, (*parse.ReturnStatement)(nil), nil)

			controlChan <- core.DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					assignments[1]: {}, //b = g()
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []core.ProgramStoppedEvent
			var globalScopes []map[string]core.Value
			var localScopes []map[string]core.Value
			var stackTraces [][]core.StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				controlChan <- core.DebugCommandStepIn{ThreadId: debugger.ThreadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandStepOut{ThreadId: debugger.ThreadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'return b'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'return b'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandContinue{ThreadId: debugger.ThreadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Int(3), result)

			assert.Equal(t, []core.ProgramStoppedEvent{
				{Reason: core.BreakpointStop, ThreadId: debugger.ThreadId()},
				{Reason: core.StepInStop, ThreadId: debugger.ThreadId()},
				{Reason: core.StepOutStop, ThreadId: debugger.ThreadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]core.Value{
				{"x": core.Int(2)},
				{"a": core.Int(2), "b": core.Int(3)},
			}, localScopes)

			assert.Equal(t, [][]core.StackFrameInfo{
				{
					{
						Name:                 "(fn) core-test:2:5:",
						Node:                 assignments[0],
						Chunk:                chunk,
						Id:                   3,
						StartLine:            2,
						StartColumn:          5,
						StatementStartLine:   3,
						StatementStartColumn: 6,
					},
					{
						Name:                 "(fn) core-test:6:5:",
						Node:                 assignments[1],
						Chunk:                chunk,
						Id:                   2,
						StartLine:            6,
						StartColumn:          5,
						StatementStartLine:   7,
						StatementStartColumn: 6,
					},
					{
						Name:                 "core-test",
						Node:                 assignments[2],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   10,
						StatementStartColumn: 5,
					},
				},
				{
					{
						Name:                 "(fn) core-test:6:5:",
						Node:                 returnStmts[1],
						Chunk:                chunk,
						Id:                   2,
						StartLine:            6,
						StartColumn:          5,
						StatementStartLine:   8,
						StatementStartColumn: 6,
					},
					{
						Name:                 "core-test",
						Node:                 assignments[2],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   10,
						StatementStartColumn: 5,
					},
				},
			}, stackTraces)
		})

		t.Run("breakpoint & step out before deepest function call", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`
				fn g(x){
					a = 3
					return a
				}
				fn f(a){
					b = g(a)
					return b
				}
				result = f(2)
				return result
			`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.CancelGracefully()

			assignments := parse.FindNodes(chunk.Node, (*parse.Assignment)(nil), nil)
			returnStatements := parse.FindNodes(chunk.Node, (*parse.ReturnStatement)(nil), nil)

			controlChan <- core.DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					assignments[1]: {}, //b = g()
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []core.ProgramStoppedEvent
			var globalScopes []map[string]core.Value
			var localScopes []map[string]core.Value
			var stackTraces [][]core.StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				controlChan <- core.DebugCommandStepOut{ThreadId: debugger.ThreadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'return result'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.ThreadId(),
				}

				//get stack trace while stopped at 'return result'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandContinue{ThreadId: debugger.ThreadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Int(3), result)

			assert.Equal(t, []core.ProgramStoppedEvent{
				{Reason: core.BreakpointStop, ThreadId: debugger.ThreadId()},
				{Reason: core.StepOutStop, ThreadId: debugger.ThreadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]core.Value{
				{"result": core.Int(3)},
			}, localScopes)

			assert.Equal(t, [][]core.StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 returnStatements[2],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   11,
						StatementStartColumn: 5,
					},
				},
			}, stackTraces)
		})

	})

	t.Run("secondary event", func(t *testing.T) {
		state, ctx, chunk, debugger := setup(`
			send_secondary_debug_event()

			# add a delay in order for the debugger to receive the command while the program is running.
			# otherwise it will receive the command after the call to eval().
			sleep 300ms 

			return 1
		`)

		ctx.MustGetClosestState().Globals.Set("sleep", core.WrapGoFunction(core.Sleep))

		ctx.MustGetClosestState().Globals.Set("send_secondary_debug_event", core.WrapGoFunction(func(ctx *core.Context) {
			debugger.ControlChan() <- core.DebugCommandInformAboutSecondaryEvent{
				Event: core.IncomingMessageReceivedEvent{
					MessageType: "x",
				},
			}
		}))

		var events []core.SecondaryDebugEvent
		var eventsLock sync.Mutex

		goroutineStarted := make(chan struct{})

		go func() {
			goroutineStarted <- struct{}{}
			for event := range debugger.SecondaryEventsChan() {
				eventsLock.Lock()
				events = append(events, event)
				eventsLock.Unlock()
			}
		}()

		<-goroutineStarted

		defer ctx.CancelGracefully()

		result, err := eval(chunk.Node, state)

		debugger.ControlChan() <- core.DebugCommandCloseDebugger{}

		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, core.Int(1), result)

		eventsLock.Lock()
		defer eventsLock.Unlock()

		assert.Equal(t, []core.SecondaryDebugEvent{
			core.IncomingMessageReceivedEvent{
				MessageType: "x",
			},
		}, events)

		_, notClosed := <-debugger.SecondaryEventsChan()
		assert.False(t, notClosed)
	})

	t.Run("lthread", func(t *testing.T) {
		t.Run("successive breakpoints", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`
				r = go do {
					a = 1
					a = 2
					a = 3
					return a
				}

				return r.wait_result!()
			`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.CancelGracefully()

			assignments := parse.FindNodes(chunk.Node, (*parse.Assignment)(nil), nil)
			var routineChunk atomic.Value
			var routineDebugger_ atomic.Value

			controlChan <- core.DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					assignments[2]: {}, //a = 2
					assignments[3]: {}, //a = 3
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []core.ProgramStoppedEvent
			var globalScopes []map[string]core.Value
			var localScopes []map[string]core.Value
			var stackTraces [][]core.StackFrameInfo
			var threads atomic.Value

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				secondaryEvent := <-debugger.SecondaryEventsChan()

				threadId := secondaryEvent.(core.LThreadSpawnedEvent).StateId
				lthreadDebugger := debugger.GetDebuggerOfThread(threadId)
				routineChunk.Store(lthreadDebugger.ThreadModule().MainChunk)
				routineDebugger_.Store(lthreadDebugger)
				threads.Store(debugger.Threads())

				//get scopes while stopped at 'a = 2'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: threadId,
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: threadId,
				}

				controlChan <- core.DebugCommandContinue{ThreadId: threadId}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: threadId,
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: threadId,
				}

				controlChan <- core.DebugCommandContinue{ThreadId: threadId}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Int(3), result)

			time.Sleep(time.Millisecond)
			routineDebugger := routineDebugger_.Load().(*core.Debugger)
			assert.True(t, routineDebugger.Closed())

			routineThreadId := routineDebugger.ThreadId()

			assert.ElementsMatch(t, []core.ThreadInfo{
				{Name: "core-test", Id: debugger.ThreadId()},
				{Name: "core-test", Id: routineThreadId},
			}, threads.Load())

			assert.Equal(t, []core.ProgramStoppedEvent{
				{Reason: core.BreakpointStop, ThreadId: routineThreadId},
				{Reason: core.BreakpointStop, ThreadId: routineThreadId},
			}, stoppedEvents)

			assert.Equal(t, []map[string]core.Value{{}, {}}, globalScopes)

			assert.Equal(t, []map[string]core.Value{
				{"a": core.Int(1)}, {"a": core.Int(2)},
			}, localScopes)

			assert.Equal(t, [][]core.StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 assignments[2],
						Chunk:                routineChunk.Load().(*parse.ParsedChunkSource),
						Id:                   2,
						StartLine:            2,
						StartColumn:          15,
						StatementStartLine:   4,
						StatementStartColumn: 6,
					},
				},
				{
					{
						Name:                 "core-test",
						Node:                 assignments[3],
						Chunk:                routineChunk.Load().(*parse.ParsedChunkSource),
						Id:                   2,
						StartLine:            2,
						StartColumn:          15,
						StatementStartLine:   5,
						StatementStartColumn: 6,
					},
				},
			}, stackTraces)
		})

		t.Run("successive breakpoints in & after lthread", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`
				r = go do {
					a = 1
					return a
				}

				result = r.wait_result!()
				return result
			`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.CancelGracefully()

			returnStmts := parse.FindNodes(chunk.Node, (*parse.ReturnStatement)(nil), nil)
			var routineChunk atomic.Value
			var routineDebugger_ atomic.Value

			controlChan <- core.DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					returnStmts[0]: {}, //return a
					returnStmts[1]: {}, //return result
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []core.ProgramStoppedEvent
			var stackTraces [][]core.StackFrameInfo
			var threads atomic.Value
			var lthreadDebuggerClosed atomic.Bool

			go func() {
				event := <-stoppedChan
				//stopped at 'return a'
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				var threadsList [][]core.ThreadInfo

				secondaryEvent := <-debugger.SecondaryEventsChan()

				routineThreadId := secondaryEvent.(core.LThreadSpawnedEvent).StateId
				lthreadDebugger := debugger.GetDebuggerOfThread(routineThreadId)
				routineChunk.Store(lthreadDebugger.ThreadModule().MainChunk)
				routineDebugger_.Store(lthreadDebugger)
				threadsList = append(threadsList, debugger.Threads())

				//get stack trace while stopped at  'return a'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: routineThreadId,
				}

				controlChan <- core.DebugCommandContinue{ThreadId: routineThreadId}

				event = <-stoppedChan
				//stopped at 'return result'

				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				time.Sleep(time.Millisecond)
				//lthread debugger should be closed
				threadsList = append(threadsList, debugger.Threads())
				threads.Store(threadsList)
				lthreadDebuggerClosed.Store(lthreadDebugger.Closed())

				//get stack trace while stopped at  'return result'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.ThreadId(),
				}

				controlChan <- core.DebugCommandContinue{ThreadId: debugger.ThreadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Int(1), result)

			routineDebugger := routineDebugger_.Load().(*core.Debugger)
			routineThreadId := routineDebugger.ThreadId()

			assert.ElementsMatch(t, [][]core.ThreadInfo{
				{
					{Name: "core-test", Id: debugger.ThreadId()},
					{Name: "core-test", Id: routineThreadId},
				},
				{
					{Name: "core-test", Id: debugger.ThreadId()},
				},
			}, threads.Load())

			assert.True(t, lthreadDebuggerClosed.Load())

			assert.Equal(t, []core.ProgramStoppedEvent{
				{Reason: core.BreakpointStop, ThreadId: routineThreadId},
				{Reason: core.BreakpointStop, ThreadId: debugger.ThreadId()},
			}, stoppedEvents)

			assert.Equal(t, [][]core.StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 returnStmts[0],
						Chunk:                routineChunk.Load().(*parse.ParsedChunkSource),
						Id:                   2,
						StartLine:            2,
						StartColumn:          15,
						StatementStartLine:   4,
						StatementStartColumn: 6,
					},
				},
				{
					{
						Name:                 "core-test",
						Node:                 returnStmts[1],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   8,
						StatementStartColumn: 5,
					},
				},
			}, stackTraces)
		})

		t.Run("breakpoint & two steps", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`
				r = go do {
					a = 1
					a = 2
					a = 3
					return a
				}

				return r.wait_result!()
			`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.CancelGracefully()

			assignments := parse.FindNodes(chunk.Node, (*parse.Assignment)(nil), nil)
			var routineChunk atomic.Value
			var routineDebugger_ atomic.Value
			var threads atomic.Value

			controlChan <- core.DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					assignments[1]: {}, //a = 1
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []core.ProgramStoppedEvent
			var globalScopes []map[string]core.Value
			var localScopes []map[string]core.Value
			var stackTraces [][]core.StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				secondaryEvent := <-debugger.SecondaryEventsChan()
				routineThreadId := secondaryEvent.(core.LThreadSpawnedEvent).StateId
				routineDebugger := debugger.GetDebuggerOfThread(routineThreadId)
				routineChunk.Store(routineDebugger.ThreadModule().MainChunk)
				routineDebugger_.Store(routineDebugger)
				threads.Store(debugger.Threads())

				controlChan <- core.DebugCommandNextStep{
					ThreadId: routineThreadId,
				}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 2'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: routineThreadId,
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: routineThreadId,
				}

				controlChan <- core.DebugCommandNextStep{ThreadId: routineThreadId}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: routineThreadId,
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: routineThreadId,
				}

				controlChan <- core.DebugCommandContinue{ThreadId: routineThreadId}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Int(3), result)

			routineThreadId := routineDebugger_.Load().(*core.Debugger).ThreadId()

			assert.ElementsMatch(t, []core.ThreadInfo{
				{Name: "core-test", Id: debugger.ThreadId()},
				{Name: "core-test", Id: routineThreadId},
			}, threads.Load())

			assert.Equal(t, []core.ProgramStoppedEvent{
				{Reason: core.BreakpointStop, ThreadId: routineThreadId},
				{Reason: core.NextStepStop, ThreadId: routineThreadId},
				{Reason: core.NextStepStop, ThreadId: routineThreadId},
			}, stoppedEvents)

			assert.Equal(t, []map[string]core.Value{{}, {}}, globalScopes)

			assert.Equal(t, []map[string]core.Value{
				{"a": core.Int(1)}, {"a": core.Int(2)},
			}, localScopes)

			assert.Equal(t, [][]core.StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 assignments[2],
						Chunk:                routineChunk.Load().(*parse.ParsedChunkSource),
						Id:                   2,
						StartLine:            2,
						StartColumn:          15,
						StatementStartLine:   4,
						StatementStartColumn: 6,
					},
				},
				{
					{
						Name:                 "core-test",
						Node:                 assignments[3],
						Chunk:                routineChunk.Load().(*parse.ParsedChunkSource),
						Id:                   2,
						StartLine:            2,
						StartColumn:          15,
						StatementStartLine:   5,
						StatementStartColumn: 6,
					},
				},
			}, stackTraces)
		})

		t.Run("lthread should properly stop on root debugger closing with cancellation", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`
				r = go {globals: {sleep: sleep}} do {
					sleep 2s
					return 0
				}

				return r.wait_result!()
			`)

			ctx.MustGetClosestState().Globals.Set("sleep", core.WrapGoFunction(core.Sleep))

			controlChan := debugger.ControlChan()
			var callbackCalled atomic.Bool

			var routineDebugger_ atomic.Value

			defer ctx.CancelGracefully()

			go func() {
				secondaryEvent := <-debugger.SecondaryEventsChan()

				threadId := secondaryEvent.(core.LThreadSpawnedEvent).StateId
				lthreadDebugger := debugger.GetDebuggerOfThread(threadId)
				routineDebugger_.Store(lthreadDebugger)

				//Stop the root debugger.

				controlChan <- core.DebugCommandCloseDebugger{
					CancelExecution: true,
					Done: func() {
						callbackCalled.Store(true)
					},
				}
			}()

			_, err := eval(chunk.Node, state)

			if !assert.True(t, callbackCalled.Load()) {
				return
			}

			if !assert.ErrorIs(t, err, context.Canceled) {
				return
			}

			routineDebugger := routineDebugger_.Load().(*core.Debugger)
			assert.True(t, routineDebugger.Closed())
		})
	})

	t.Run("lthread creation inside lthread", func(t *testing.T) {
		t.Run("successive breakpoints", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`
				r1 = go {allow: {create: {threads: {}}}} do {
					r2 = go do {
						a = 1
						a = 2
						a = 3
						return a
					}
					return r2.wait_result!()
				}

				return r1.wait_result!()
			`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.CancelGracefully()

			var (
				assignments     = parse.FindNodes(chunk.Node, (*parse.Assignment)(nil), nil)
				routineChunk    atomic.Value
				parentThreadId_ atomic.Value
				threadId_       atomic.Value
				threads         atomic.Value
			)

			controlChan <- core.DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					assignments[3]: {}, //a = 2
					assignments[4]: {}, //a = 3
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []core.ProgramStoppedEvent
			var globalScopes []map[string]core.Value
			var localScopes []map[string]core.Value
			var stackTraces [][]core.StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				secondaryEvent := <-debugger.SecondaryEventsChan()
				parentThreadId := secondaryEvent.(core.LThreadSpawnedEvent).StateId
				parentThreadId_.Store(parentThreadId)

				secondaryEvent = <-debugger.SecondaryEventsChan()
				routineThreadId := secondaryEvent.(core.LThreadSpawnedEvent).StateId
				routineDebugger := debugger.GetDebuggerOfThread(routineThreadId)
				routineChunk.Store(routineDebugger.ThreadModule().MainChunk)
				threadId_.Store(routineThreadId)
				threads.Store(debugger.Threads())

				//get scopes while stopped at 'a = 2'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: routineThreadId,
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: routineThreadId,
				}

				controlChan <- core.DebugCommandContinue{ThreadId: routineThreadId}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: routineThreadId,
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: routineThreadId,
				}

				controlChan <- core.DebugCommandContinue{ThreadId: routineThreadId}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Int(3), result)

			routineThreadId := threadId_.Load().(core.StateId)

			assert.ElementsMatch(t, []core.ThreadInfo{
				{Name: "core-test", Id: debugger.ThreadId()},
				{Name: "core-test", Id: parentThreadId_.Load().(core.StateId)},
				{Name: "core-test", Id: routineThreadId},
			}, threads.Load())

			assert.Equal(t, []core.ProgramStoppedEvent{
				{Reason: core.BreakpointStop, ThreadId: routineThreadId},
				{Reason: core.BreakpointStop, ThreadId: routineThreadId},
			}, stoppedEvents)

			assert.Equal(t, []map[string]core.Value{{}, {}}, globalScopes)

			assert.Equal(t, []map[string]core.Value{
				{"a": core.Int(1)}, {"a": core.Int(2)},
			}, localScopes)

			assert.Equal(t, [][]core.StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 assignments[3],
						Chunk:                routineChunk.Load().(*parse.ParsedChunkSource),
						Id:                   3,
						StartLine:            3,
						StartColumn:          17,
						StatementStartLine:   5,
						StatementStartColumn: 7,
					},
				},
				{
					{
						Name:                 "core-test",
						Node:                 assignments[4],
						Chunk:                routineChunk.Load().(*parse.ParsedChunkSource),
						Id:                   3,
						StartLine:            3,
						StartColumn:          17,
						StatementStartLine:   6,
						StatementStartColumn: 7,
					},
				},
			}, stackTraces)
		})

		t.Run("breakpoint & two steps", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`
				r1 = go {allow: {create: {threads: {}}}} do {
					r2 = go do {
						a = 1
						a = 2
						a = 3
						return a
					}
					return r2.wait_result!()
				}

				return r1.wait_result!()
			`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.CancelGracefully()

			var (
				assignments     = parse.FindNodes(chunk.Node, (*parse.Assignment)(nil), nil)
				routineChunk    atomic.Value
				parentThreadId_ atomic.Value
				threadId_       atomic.Value
				threads         atomic.Value
			)

			controlChan <- core.DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					assignments[2]: {}, //a = 1
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []core.ProgramStoppedEvent
			var globalScopes []map[string]core.Value
			var localScopes []map[string]core.Value
			var stackTraces [][]core.StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				secondaryEvent := <-debugger.SecondaryEventsChan()
				parentThreadId := secondaryEvent.(core.LThreadSpawnedEvent).StateId
				parentThreadId_.Store(parentThreadId)

				secondaryEvent = <-debugger.SecondaryEventsChan()
				routineThreadId := secondaryEvent.(core.LThreadSpawnedEvent).StateId
				routineDebugger := debugger.GetDebuggerOfThread(routineThreadId)
				routineChunk.Store(routineDebugger.ThreadModule().MainChunk)
				threadId_.Store(routineThreadId)
				threads.Store(debugger.Threads())

				controlChan <- core.DebugCommandNextStep{
					ThreadId: routineThreadId,
				}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 2'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: routineThreadId,
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: routineThreadId,
				}

				controlChan <- core.DebugCommandNextStep{ThreadId: routineThreadId}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- core.DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]core.Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: routineThreadId,
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- core.DebugCommandGetStackTrace{
					Get: func(trace []core.StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: routineThreadId,
				}

				controlChan <- core.DebugCommandContinue{ThreadId: routineThreadId}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Int(3), result)

			routineThreadId := threadId_.Load().(core.StateId)

			assert.ElementsMatch(t, []core.ThreadInfo{
				{Name: "core-test", Id: debugger.ThreadId()},
				{Name: "core-test", Id: parentThreadId_.Load().(core.StateId)},
				{Name: "core-test", Id: routineThreadId},
			}, threads.Load())

			assert.Equal(t, []core.ProgramStoppedEvent{
				{Reason: core.BreakpointStop, ThreadId: routineThreadId},
				{Reason: core.NextStepStop, ThreadId: routineThreadId},
				{Reason: core.NextStepStop, ThreadId: routineThreadId},
			}, stoppedEvents)

			assert.Equal(t, []map[string]core.Value{{}, {}}, globalScopes)

			assert.Equal(t, []map[string]core.Value{
				{"a": core.Int(1)}, {"a": core.Int(2)},
			}, localScopes)

			assert.Equal(t, [][]core.StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 assignments[3],
						Chunk:                routineChunk.Load().(*parse.ParsedChunkSource),
						Id:                   3,
						StartLine:            3,
						StartColumn:          17,
						StatementStartLine:   5,
						StatementStartColumn: 7,
					},
				},
				{
					{
						Name:                 "core-test",
						Node:                 assignments[4],
						Chunk:                routineChunk.Load().(*parse.ParsedChunkSource),
						Id:                   3,
						StartLine:            3,
						StartColumn:          17,
						StatementStartLine:   6,
						StatementStartColumn: 7,
					},
				},
			}, stackTraces)
		})

	})
}
