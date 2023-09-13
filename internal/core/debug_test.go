package core

import (
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestTreeWalkDebug(t *testing.T) {
	//TODO: add test with included chunks

	testDebugModeEval(t, func(code string, opts ...debugTestOptions) (any, *Context, *parse.ParsedChunk, *Debugger) {
		state := NewGlobalState(NewDefaultTestContext())
		treeWalkState := NewTreeWalkStateWithGlobal(state)

		chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "core-test",
			CodeString: code,
		}))

		nextBreakPointId := int32(INITIAL_BREAKPOINT_ID)
		var options debugTestOptions
		if len(opts) == 1 {
			options = opts[0]
		}

		breakpoints, err := GetBreakpointsFromLines(options.breakpointLines, chunk, &nextBreakPointId)
		if !assert.NoError(t, err) {
			assert.Fail(t, "failed to get breakpoints from lines "+err.Error())
		}

		debugger := NewDebugger(DebuggerArgs{
			Logger:                zerolog.New(io.Discard),
			InitialBreakpoints:    breakpoints,
			ExceptionBreakpointId: options.exceptionBreakpointId,
		})

		state.Module = &Module{MainChunk: chunk}
		debugger.AttachAndStart(treeWalkState)

		return treeWalkState, state.Ctx, chunk, debugger
	}, func(n parse.Node, state any) (Value, error) {
		result, err := TreeWalkEval(n, state.(*TreeWalkState))

		return result, err
	})
}

type debugTestOptions struct {
	breakpointLines       []int
	exceptionBreakpointId int32
}

func testDebugModeEval(
	t *testing.T,
	setup func(code string, opts ...debugTestOptions) (any, *Context, *parse.ParsedChunk, *Debugger),
	eval func(n parse.Node, state any) (Value, error),
) {

	t.Run("single instruction module", func(t *testing.T) {
		t.Run("breakpoint", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`1`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.Cancel()

			controlChan <- DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					chunk.Node.Statements[0]: {}, //1
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandContinue{ThreadId: debugger.threadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(1), result)

			assert.Equal(t, []ProgramStoppedEvent{
				{Reason: BreakpointStop, ThreadId: debugger.threadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]Value{{}}, globalScopes)

			assert.Equal(t, []map[string]Value{{}}, localScopes)

			assert.Equal(t, [][]StackFrameInfo{
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

			defer ctx.Cancel()

			controlChan <- DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					chunk.Node.Statements[1]: {}, //a = 2
					chunk.Node.Statements[2]: {}, //a = 3
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 2'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandContinue{ThreadId: debugger.threadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandContinue{ThreadId: debugger.threadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(3), result)

			assert.Equal(t, []ProgramStoppedEvent{
				{Reason: BreakpointStop, ThreadId: debugger.threadId()},
				{Reason: BreakpointStop, ThreadId: debugger.threadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]Value{{}, {}}, globalScopes)

			assert.Equal(t, []map[string]Value{
				{"a": Int(1)}, {"a": Int(2)},
			}, localScopes)

			assert.Equal(t, [][]StackFrameInfo{
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

			defer ctx.Cancel()

			controlChan <- DebugCommandSetBreakpoints{
				Chunk:             chunk,
				BreakPointsByLine: []int{2, 3}, //a = 2 & a = 3
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 2'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandContinue{ThreadId: debugger.threadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandContinue{ThreadId: debugger.threadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(3), result)

			assert.Equal(t, []ProgramStoppedEvent{
				{Reason: BreakpointStop, ThreadId: debugger.threadId()},
				{Reason: BreakpointStop, ThreadId: debugger.threadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]Value{{}, {}}, globalScopes)

			assert.Equal(t, []map[string]Value{
				{"a": Int(1)}, {"a": Int(2)},
			}, localScopes)

			assert.Equal(t, [][]StackFrameInfo{
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

			defer ctx.Cancel()

			equalChunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
				NameString: "core-test",
				CodeString: chunk.Source.Code(),
			}))

			controlChan <- DebugCommandSetBreakpoints{
				Chunk:             equalChunk,
				BreakPointsByLine: []int{2, 3}, //a = 2 & a = 3
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 2'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandContinue{ThreadId: debugger.threadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandContinue{ThreadId: debugger.threadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(3), result)

			assert.Equal(t, []ProgramStoppedEvent{
				{Reason: BreakpointStop, ThreadId: debugger.threadId()},
				{Reason: BreakpointStop, ThreadId: debugger.threadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]Value{{}, {}}, globalScopes)

			assert.Equal(t, []map[string]Value{
				{"a": Int(1)}, {"a": Int(2)},
			}, localScopes)

			assert.Equal(t, [][]StackFrameInfo{
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

			defer ctx.Cancel()

			breakpointsChan := make(chan []BreakpointInfo)

			controlChan <- DebugCommandSetBreakpoints{
				Chunk:             chunk,
				BreakPointsByLine: []int{2},
				GetBreakpointsSetByLine: func(breakpoints []BreakpointInfo) {
					breakpointsChan <- breakpoints
				},
			}

			breakpoints := <-breakpointsChan
			assert.Equal(t, []BreakpointInfo{
				{
					Chunk: chunk,
					Id:    INITIAL_BREAKPOINT_ID,
				},
			}, breakpoints)

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(3), result)
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

			defer ctx.Cancel()

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 2'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandContinue{ThreadId: debugger.threadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandContinue{ThreadId: debugger.threadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(3), result)

			assert.Equal(t, []ProgramStoppedEvent{
				{Reason: BreakpointStop, ThreadId: debugger.threadId()},
				{Reason: BreakpointStop, ThreadId: debugger.threadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]Value{{}, {}}, globalScopes)

			assert.Equal(t, []map[string]Value{
				{"a": Int(1)}, {"a": Int(2)},
			}, localScopes)

			assert.Equal(t, [][]StackFrameInfo{
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

			defer ctx.Cancel()

			controlChan <- DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					chunk.Node.Statements[0]: {}, //a = 1
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				controlChan <- DebugCommandNextStep{ThreadId: debugger.threadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 2'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandNextStep{ThreadId: debugger.threadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandContinue{ThreadId: debugger.threadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(3), result)

			assert.Equal(t, []ProgramStoppedEvent{
				{Reason: BreakpointStop, ThreadId: debugger.threadId()},
				{Reason: NextStepStop, ThreadId: debugger.threadId()},
				{Reason: NextStepStop, ThreadId: debugger.threadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]Value{{}, {}}, globalScopes)

			assert.Equal(t, []map[string]Value{
				{"a": Int(1)}, {"a": Int(2)},
			}, localScopes)

			assert.Equal(t, [][]StackFrameInfo{
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

			defer ctx.Cancel()

			controlChan <- DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					chunk.Node.Statements[0]: {}, //a = 1
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				controlChan <- DebugCommandStepIn{
					ThreadId: debugger.threadId(),
				}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 2'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandStepIn{ThreadId: debugger.threadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandContinue{ThreadId: debugger.threadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(3), result)

			if !assert.Equal(t, []ProgramStoppedEvent{
				{Reason: BreakpointStop, ThreadId: debugger.threadId()},
				{Reason: StepInStop, ThreadId: debugger.threadId()},
				{Reason: StepInStop, ThreadId: debugger.threadId()},
			}, stoppedEvents) {
				return
			}

			assert.Equal(t, []map[string]Value{
				{"a": Int(1)}, {"a": Int(2)},
			}, localScopes)

			assert.Equal(t, [][]StackFrameInfo{
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

			defer ctx.Cancel()

			var breakpointId int32

			controlChan <- DebugCommandSetExceptionBreakpoints{
				GetExceptionBreakpointId: func(id int32) {
					breakpointId = id
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				event := <-stoppedChan

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'overflow = ...'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'overflow = ...'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandContinue{
					ThreadId: debugger.threadId(),
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
			assert.Equal(t, ExceptionBreakpointStop, event.Reason)
			assert.ErrorIs(t, event.ExceptionError, ErrIntOverflow)
			assert.Equal(t, breakpointId, event.Breakpoint.Id)

			assert.Equal(t, []map[string]Value{{}}, globalScopes)

			assert.Equal(t, []map[string]Value{
				{"a": Int(1)},
			}, localScopes)

			assert.Equal(t, [][]StackFrameInfo{
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
			exceptionBreakpointId := int32(INITIAL_BREAKPOINT_ID + 3)
			state, ctx, chunk, debugger := setup(`
				a = 1
				overflow = (10_000_000_000 * 10_000_000_000)
				return a
			`, debugTestOptions{exceptionBreakpointId: exceptionBreakpointId})

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.Cancel()

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				event := <-stoppedChan

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'overflow = ...'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'overflow = ...'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandContinue{ThreadId: debugger.threadId()}
			}()

			_, err := eval(chunk.Node, state)

			if !assert.Error(t, err) {
				return
			}

			if !assert.Len(t, stoppedEvents, 1) {
				return
			}

			event := stoppedEvents[0]
			assert.Equal(t, ExceptionBreakpointStop, event.Reason)
			assert.ErrorIs(t, event.ExceptionError, ErrIntOverflow)
			assert.Equal(t, exceptionBreakpointId, event.Breakpoint.Id)

			assert.Equal(t, []map[string]Value{{}}, globalScopes)

			assert.Equal(t, []map[string]Value{
				{"a": Int(1)},
			}, localScopes)

			assert.Equal(t, [][]StackFrameInfo{
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
				sleep 1s
				a = 2
				return a
			`)

			global := WrapGoFunction(Sleep)
			ctx.GetClosestState().Globals.Set("sleep", global)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.Cancel()

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				//wait to make sure the pause command will be sent during the sleep(1s) call
				time.Sleep(10 * time.Millisecond)

				controlChan <- DebugCommandPause{
					ThreadId: debugger.threadId(),
				}

				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 2'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandContinue{ThreadId: debugger.threadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(2), result)

			assert.Equal(t, []ProgramStoppedEvent{
				{Reason: PauseStop, ThreadId: debugger.threadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]Value{
				{"sleep": global},
			}, globalScopes)

			assert.Equal(t, []map[string]Value{
				{"a": Int(1)},
			}, localScopes)

			assert.Equal(t, [][]StackFrameInfo{
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

			defer ctx.Cancel()

			controlChan <- DebugCommandSetBreakpoints{
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

				controlChan <- DebugCommandCloseDebugger{}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(2), result)
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

			defer ctx.Cancel()

			assignments := parse.FindNodes(chunk.Node, (*parse.Assignment)(nil), nil)
			returnStmts := parse.FindNodes(chunk.Node, (*parse.ReturnStatement)(nil), nil)

			controlChan <- DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					assignments[0]: {}, //b = 3
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				controlChan <- DebugCommandNextStep{ThreadId: debugger.threadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'return b'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'return b'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandNextStep{ThreadId: debugger.threadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'return result'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'return result'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandContinue{ThreadId: debugger.threadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(3), result)

			assert.Equal(t, []ProgramStoppedEvent{
				{Reason: BreakpointStop, ThreadId: debugger.threadId()},
				{Reason: NextStepStop, ThreadId: debugger.threadId()},
				{Reason: NextStepStop, ThreadId: debugger.threadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]Value{
				{"a": Int(2), "b": Int(3)},
				{"result": Int(3)},
			}, localScopes)

			assert.Equal(t, [][]StackFrameInfo{
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

			defer ctx.Cancel()

			returnStmts := parse.FindNodes(chunk.Node, (*parse.ReturnStatement)(nil), nil)

			controlChan <- DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					chunk.Node.Statements[1]: {}, //result = f(2)
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				controlChan <- DebugCommandNextStep{ThreadId: debugger.threadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'return result'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'return result'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandContinue{ThreadId: debugger.threadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(3), result)

			assert.Equal(t, []ProgramStoppedEvent{
				{Reason: BreakpointStop, ThreadId: debugger.threadId()},
				{Reason: NextStepStop, ThreadId: debugger.threadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]Value{{"result": Int(3)}}, localScopes)

			assert.Equal(t, [][]StackFrameInfo{
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

			defer ctx.Cancel()

			controlChan <- DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					chunk.Node.Statements[1]: {}, //result = f()
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				controlChan <- DebugCommandStepIn{ThreadId: debugger.threadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 1'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'a = 1'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandContinue{ThreadId: debugger.threadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(3), result)

			assert.Equal(t, []ProgramStoppedEvent{
				{Reason: BreakpointStop, ThreadId: debugger.threadId()},
				{Reason: StepInStop, ThreadId: debugger.threadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]Value{{"a": Int(2)}}, localScopes)

			assignments := parse.FindNodes(chunk.Node, (*parse.Assignment)(nil), nil)

			assert.Equal(t, [][]StackFrameInfo{
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

			defer ctx.Cancel()

			returnStmts := parse.FindNodes(chunk.Node, (*parse.ReturnStatement)(nil), nil)

			controlChan <- DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					chunk.Node.Statements[1]: {}, //result = f()
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				controlChan <- DebugCommandStepIn{ThreadId: debugger.threadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'b = 3'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'b = 3'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandStepOut{ThreadId: debugger.threadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'return result'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'return result'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandContinue{ThreadId: debugger.threadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(3), result)

			assert.Equal(t, []ProgramStoppedEvent{
				{Reason: BreakpointStop, ThreadId: debugger.threadId()},
				{Reason: StepInStop, ThreadId: debugger.threadId()},
				{Reason: StepOutStop, ThreadId: debugger.threadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]Value{
				{"a": Int(2)},
				{"result": Int(3)},
			}, localScopes)

			assignments := parse.FindNodes(chunk.Node, (*parse.Assignment)(nil), nil)

			assert.Equal(t, [][]StackFrameInfo{
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

			defer ctx.Cancel()

			assignments := parse.FindNodes(chunk.Node, (*parse.Assignment)(nil), nil)

			controlChan <- DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					assignments[1]: {}, //b = g()
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				controlChan <- DebugCommandStepIn{ThreadId: debugger.threadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandContinue{ThreadId: debugger.threadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(3), result)

			assert.Equal(t, []ProgramStoppedEvent{
				{Reason: BreakpointStop, ThreadId: debugger.threadId()},
				{Reason: StepInStop, ThreadId: debugger.threadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]Value{
				{"x": Int(2)},
			}, localScopes)

			assert.Equal(t, [][]StackFrameInfo{
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

			defer ctx.Cancel()

			assignments := parse.FindNodes(chunk.Node, (*parse.Assignment)(nil), nil)
			returnStmts := parse.FindNodes(chunk.Node, (*parse.ReturnStatement)(nil), nil)

			controlChan <- DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					assignments[1]: {}, //b = g()
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				controlChan <- DebugCommandStepIn{ThreadId: debugger.threadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandStepOut{ThreadId: debugger.threadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'return b'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'return b'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandContinue{ThreadId: debugger.threadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(3), result)

			assert.Equal(t, []ProgramStoppedEvent{
				{Reason: BreakpointStop, ThreadId: debugger.threadId()},
				{Reason: StepInStop, ThreadId: debugger.threadId()},
				{Reason: StepOutStop, ThreadId: debugger.threadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]Value{
				{"x": Int(2)},
				{"a": Int(2), "b": Int(3)},
			}, localScopes)

			assert.Equal(t, [][]StackFrameInfo{
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

			defer ctx.Cancel()

			assignments := parse.FindNodes(chunk.Node, (*parse.Assignment)(nil), nil)
			returnStatements := parse.FindNodes(chunk.Node, (*parse.ReturnStatement)(nil), nil)

			controlChan <- DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					assignments[1]: {}, //b = g()
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				controlChan <- DebugCommandStepOut{ThreadId: debugger.threadId()}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'return result'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: debugger.threadId(),
				}

				//get stack trace while stopped at 'return result'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandContinue{ThreadId: debugger.threadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(3), result)

			assert.Equal(t, []ProgramStoppedEvent{
				{Reason: BreakpointStop, ThreadId: debugger.threadId()},
				{Reason: StepOutStop, ThreadId: debugger.threadId()},
			}, stoppedEvents)

			assert.Equal(t, []map[string]Value{
				{"result": Int(3)},
			}, localScopes)

			assert.Equal(t, [][]StackFrameInfo{
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
			sleep 500ms 

			return 1
		`)

		ctx.GetClosestState().Globals.Set("sleep", WrapGoFunction(Sleep))

		ctx.GetClosestState().Globals.Set("send_secondary_debug_event", WrapGoFunction(func(ctx *Context) {
			debugger.ControlChan() <- DebugCommandInformAboutSecondaryEvent{
				Event: IncomingMessageReceivedEvent{
					MessageType: "x",
				},
			}
		}))

		var events []SecondaryDebugEvent
		var eventsLock sync.Mutex

		goroutineStarted := make(chan struct{})

		go func() {
			goroutineStarted <- struct{}{}
			for event := range debugger.SecondaryEvents() {
				eventsLock.Lock()
				events = append(events, event)
				eventsLock.Unlock()
			}
		}()

		<-goroutineStarted

		defer ctx.Cancel()

		result, err := eval(chunk.Node, state)

		debugger.ControlChan() <- DebugCommandCloseDebugger{}

		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, Int(1), result)

		eventsLock.Lock()
		defer eventsLock.Unlock()

		assert.Equal(t, []SecondaryDebugEvent{
			IncomingMessageReceivedEvent{
				MessageType: "x",
			},
		}, events)

		_, notClosed := <-debugger.SecondaryEvents()
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

			defer ctx.Cancel()

			assignments := parse.FindNodes(chunk.Node, (*parse.Assignment)(nil), nil)
			var routineChunk atomic.Value
			var routineDebugger_ atomic.Value

			controlChan <- DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					assignments[2]: {}, //a = 2
					assignments[3]: {}, //a = 3
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo
			var threads atomic.Value

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				secondaryEvent := <-debugger.SecondaryEvents()

				threadId := secondaryEvent.(LThreadSpawnedEvent).StateId
				lthreadDebugger := debugger.shared.getDebuggerOfThread(threadId)
				routineChunk.Store(lthreadDebugger.globalState.Module.MainChunk)
				routineDebugger_.Store(lthreadDebugger)
				threads.Store(debugger.Threads())

				//get scopes while stopped at 'a = 2'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: threadId,
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: threadId,
				}

				controlChan <- DebugCommandContinue{ThreadId: threadId}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: threadId,
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: threadId,
				}

				controlChan <- DebugCommandContinue{ThreadId: threadId}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(3), result)

			time.Sleep(time.Millisecond)
			routineDebugger := routineDebugger_.Load().(*Debugger)
			assert.True(t, routineDebugger.Closed())

			routineThreadId := routineDebugger.threadId()

			assert.ElementsMatch(t, []ThreadInfo{
				{Name: "core-test", Id: debugger.threadId()},
				{Name: "core-test", Id: routineThreadId},
			}, threads.Load())

			assert.Equal(t, []ProgramStoppedEvent{
				{Reason: BreakpointStop, ThreadId: routineThreadId},
				{Reason: BreakpointStop, ThreadId: routineThreadId},
			}, stoppedEvents)

			assert.Equal(t, []map[string]Value{{}, {}}, globalScopes)

			assert.Equal(t, []map[string]Value{
				{"a": Int(1)}, {"a": Int(2)},
			}, localScopes)

			assert.Equal(t, [][]StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 assignments[2],
						Chunk:                routineChunk.Load().(*parse.ParsedChunk),
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
						Chunk:                routineChunk.Load().(*parse.ParsedChunk),
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

			defer ctx.Cancel()

			returnStmts := parse.FindNodes(chunk.Node, (*parse.ReturnStatement)(nil), nil)
			var routineChunk atomic.Value
			var routineDebugger_ atomic.Value

			controlChan <- DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					returnStmts[0]: {}, //return a
					returnStmts[1]: {}, //return result
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []ProgramStoppedEvent
			var stackTraces [][]StackFrameInfo
			var threads atomic.Value
			var lthreadDebuggerClosed atomic.Bool

			go func() {
				event := <-stoppedChan
				//stopped at 'return a'
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				var threadsList [][]ThreadInfo

				secondaryEvent := <-debugger.SecondaryEvents()

				routineThreadId := secondaryEvent.(LThreadSpawnedEvent).StateId
				lthreadDebugger := debugger.shared.getDebuggerOfThread(routineThreadId)
				routineChunk.Store(lthreadDebugger.globalState.Module.MainChunk)
				routineDebugger_.Store(lthreadDebugger)
				threadsList = append(threadsList, debugger.Threads())

				//get stack trace while stopped at  'return a'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: routineThreadId,
				}

				controlChan <- DebugCommandContinue{ThreadId: routineThreadId}

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
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: debugger.threadId(),
				}

				controlChan <- DebugCommandContinue{ThreadId: debugger.threadId()}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(1), result)

			routineDebugger := routineDebugger_.Load().(*Debugger)
			routineThreadId := routineDebugger.threadId()

			assert.ElementsMatch(t, [][]ThreadInfo{
				{
					{Name: "core-test", Id: debugger.threadId()},
					{Name: "core-test", Id: routineThreadId},
				},
				{
					{Name: "core-test", Id: debugger.threadId()},
				},
			}, threads.Load())

			assert.True(t, lthreadDebuggerClosed.Load())

			assert.Equal(t, []ProgramStoppedEvent{
				{Reason: BreakpointStop, ThreadId: routineThreadId},
				{Reason: BreakpointStop, ThreadId: debugger.threadId()},
			}, stoppedEvents)

			assert.Equal(t, [][]StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 returnStmts[0],
						Chunk:                routineChunk.Load().(*parse.ParsedChunk),
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

			defer ctx.Cancel()

			assignments := parse.FindNodes(chunk.Node, (*parse.Assignment)(nil), nil)
			var routineChunk atomic.Value
			var routineDebugger_ atomic.Value
			var threads atomic.Value

			controlChan <- DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					assignments[1]: {}, //a = 1
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				secondaryEvent := <-debugger.SecondaryEvents()
				routineThreadId := secondaryEvent.(LThreadSpawnedEvent).StateId
				routineDebugger := debugger.shared.getDebuggerOfThread(routineThreadId)
				routineChunk.Store(routineDebugger.globalState.Module.MainChunk)
				routineDebugger_.Store(routineDebugger)
				threads.Store(debugger.Threads())

				controlChan <- DebugCommandNextStep{
					ThreadId: routineThreadId,
				}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 2'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: routineThreadId,
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: routineThreadId,
				}

				controlChan <- DebugCommandNextStep{ThreadId: routineThreadId}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: routineThreadId,
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: routineThreadId,
				}

				controlChan <- DebugCommandContinue{ThreadId: routineThreadId}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(3), result)

			routineThreadId := routineDebugger_.Load().(*Debugger).threadId()

			assert.ElementsMatch(t, []ThreadInfo{
				{Name: "core-test", Id: debugger.threadId()},
				{Name: "core-test", Id: routineThreadId},
			}, threads.Load())

			assert.Equal(t, []ProgramStoppedEvent{
				{Reason: BreakpointStop, ThreadId: routineThreadId},
				{Reason: NextStepStop, ThreadId: routineThreadId},
				{Reason: NextStepStop, ThreadId: routineThreadId},
			}, stoppedEvents)

			assert.Equal(t, []map[string]Value{{}, {}}, globalScopes)

			assert.Equal(t, []map[string]Value{
				{"a": Int(1)}, {"a": Int(2)},
			}, localScopes)

			assert.Equal(t, [][]StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 assignments[2],
						Chunk:                routineChunk.Load().(*parse.ParsedChunk),
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
						Chunk:                routineChunk.Load().(*parse.ParsedChunk),
						Id:                   2,
						StartLine:            2,
						StartColumn:          15,
						StatementStartLine:   5,
						StatementStartColumn: 6,
					},
				},
			}, stackTraces)
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

			defer ctx.Cancel()

			var (
				assignments     = parse.FindNodes(chunk.Node, (*parse.Assignment)(nil), nil)
				routineChunk    atomic.Value
				parentThreadId_ atomic.Value
				threadId_       atomic.Value
				threads         atomic.Value
			)

			controlChan <- DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					assignments[3]: {}, //a = 2
					assignments[4]: {}, //a = 3
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				secondaryEvent := <-debugger.SecondaryEvents()
				parentThreadId := secondaryEvent.(LThreadSpawnedEvent).StateId
				parentThreadId_.Store(parentThreadId)

				secondaryEvent = <-debugger.SecondaryEvents()
				routineThreadId := secondaryEvent.(LThreadSpawnedEvent).StateId
				routineDebugger := debugger.shared.getDebuggerOfThread(routineThreadId)
				routineChunk.Store(routineDebugger.globalState.Module.MainChunk)
				threadId_.Store(routineThreadId)
				threads.Store(debugger.Threads())

				//get scopes while stopped at 'a = 2'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: routineThreadId,
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: routineThreadId,
				}

				controlChan <- DebugCommandContinue{ThreadId: routineThreadId}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: routineThreadId,
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: routineThreadId,
				}

				controlChan <- DebugCommandContinue{ThreadId: routineThreadId}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(3), result)

			routineThreadId := threadId_.Load().(StateId)

			assert.ElementsMatch(t, []ThreadInfo{
				{Name: "core-test", Id: debugger.threadId()},
				{Name: "core-test", Id: parentThreadId_.Load().(StateId)},
				{Name: "core-test", Id: routineThreadId},
			}, threads.Load())

			assert.Equal(t, []ProgramStoppedEvent{
				{Reason: BreakpointStop, ThreadId: routineThreadId},
				{Reason: BreakpointStop, ThreadId: routineThreadId},
			}, stoppedEvents)

			assert.Equal(t, []map[string]Value{{}, {}}, globalScopes)

			assert.Equal(t, []map[string]Value{
				{"a": Int(1)}, {"a": Int(2)},
			}, localScopes)

			assert.Equal(t, [][]StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 assignments[3],
						Chunk:                routineChunk.Load().(*parse.ParsedChunk),
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
						Chunk:                routineChunk.Load().(*parse.ParsedChunk),
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

			defer ctx.Cancel()

			var (
				assignments     = parse.FindNodes(chunk.Node, (*parse.Assignment)(nil), nil)
				routineChunk    atomic.Value
				parentThreadId_ atomic.Value
				threadId_       atomic.Value
				threads         atomic.Value
			)

			controlChan <- DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					assignments[2]: {}, //a = 1
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				secondaryEvent := <-debugger.SecondaryEvents()
				parentThreadId := secondaryEvent.(LThreadSpawnedEvent).StateId
				parentThreadId_.Store(parentThreadId)

				secondaryEvent = <-debugger.SecondaryEvents()
				routineThreadId := secondaryEvent.(LThreadSpawnedEvent).StateId
				routineDebugger := debugger.shared.getDebuggerOfThread(routineThreadId)
				routineChunk.Store(routineDebugger.globalState.Module.MainChunk)
				threadId_.Store(routineThreadId)
				threads.Store(debugger.Threads())

				controlChan <- DebugCommandNextStep{
					ThreadId: routineThreadId,
				}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 2'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: routineThreadId,
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: routineThreadId,
				}

				controlChan <- DebugCommandNextStep{ThreadId: routineThreadId}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- DebugCommandGetScopes{
					Get: func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
					ThreadId: routineThreadId,
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- DebugCommandGetStackTrace{
					Get: func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
					ThreadId: routineThreadId,
				}

				controlChan <- DebugCommandContinue{ThreadId: routineThreadId}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(3), result)

			routineThreadId := threadId_.Load().(StateId)

			assert.ElementsMatch(t, []ThreadInfo{
				{Name: "core-test", Id: debugger.threadId()},
				{Name: "core-test", Id: parentThreadId_.Load().(StateId)},
				{Name: "core-test", Id: routineThreadId},
			}, threads.Load())

			assert.Equal(t, []ProgramStoppedEvent{
				{Reason: BreakpointStop, ThreadId: routineThreadId},
				{Reason: NextStepStop, ThreadId: routineThreadId},
				{Reason: NextStepStop, ThreadId: routineThreadId},
			}, stoppedEvents)

			assert.Equal(t, []map[string]Value{{}, {}}, globalScopes)

			assert.Equal(t, []map[string]Value{
				{"a": Int(1)}, {"a": Int(2)},
			}, localScopes)

			assert.Equal(t, [][]StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 assignments[3],
						Chunk:                routineChunk.Load().(*parse.ParsedChunk),
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
						Chunk:                routineChunk.Load().(*parse.ParsedChunk),
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
