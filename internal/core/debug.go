package core

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/inoxlang/inox/internal/afs"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

const (
	FUNCTION_FRAME_PREFIX = "(fn) "
	INITIAL_BREAKPOINT_ID = 1 //starts at 1 for compatibility with the Debug Adapter Protocol
)

var (
	ErrDebuggerAlreadyAttached = errors.New("debugger already attached")
)

type BreakpointInfo struct {
	NodeSpan    parse.NodeSpan //zero if the breakpoint is not set
	Chunk       *parse.ParsedChunk
	Id          int32 //unique for a given debugger
	StartLine   int32
	StartColumn int32
}

func (i BreakpointInfo) Verified() bool {
	return i.NodeSpan != parse.NodeSpan{}
}

type StackFrameInfo struct {
	Name        string
	Node        parse.Node //can be nil, it's either a Chunk or the current statement
	Chunk       *parse.ParsedChunk
	Id          int32 //set if debugging, unique for a given debugger
	StartLine   int32
	StartColumn int32

	StatementStartLine   int32
	StatementStartColumn int32
}

type DebugCommandSetBreakpoints struct {
	//nodes where we want to set a breakpoint, this can be set independently from .BreakPointsByLine
	BreakpointsAtNode map[parse.Node]struct{}

	//lines where we want to set a breakpoint, this can be set independently from .BreakpointsAtNode.
	//GetBreakpointsSetByLine is invoked with the resulting breakpoints, some of them can be disabled.
	BreakPointsByLine []int

	Chunk *parse.ParsedChunk

	GetBreakpointsSetByLine func(breakpoints []BreakpointInfo)
}

type DebugCommandPause struct {
}

type DebugCommandContinue struct {
}

type DebugCommandNextStep struct {
}

type DebugCommandGetScopes struct {
	Get func(globalScope map[string]Value, localScope map[string]Value)
}

type DebugCommandGetStackTrace struct {
	Get func(trace []StackFrameInfo)
}

type DebugCommandCloseDebugger struct {
	CancelExecution bool
	Done            func()
}

type ProgramStoppedEvent struct {
	Reason     ProgramStopReason
	Breakpoint *BreakpointInfo
}

type ProgramStopReason int

const (
	PauseStop ProgramStopReason = 1 + iota
	StepStop
	BreakpointStop
)

type EvaluationState interface {
	//AttachDebugger should be called before starting the evaluation.
	AttachDebugger(*Debugger)

	//DetachDebugger should be called by the evaluation's goroutine.
	DetachDebugger()

	CurrentLocalScope() map[string]Value

	GetGlobalState() *GlobalState
}

type Debugger struct {
	ctx                       *Context
	controlChan               chan any
	stoppedProgramCommandChan chan any
	stoppedProgramChan        chan ProgramStoppedEvent
	stoppedProgram            atomic.Bool
	stopBeforeNextStatement   atomic.Value //non-breakpoint ProgramStopReason
	breakpoints               map[parse.NodeSpan]BreakpointInfo
	nextBreakpointId          int32
	breakpointsLock           sync.Mutex
	resumeExecutionChan       chan struct{}

	stackFrameId atomic.Int32 //incremented by debuggees

	evaluationState EvaluationState
	globalState     *GlobalState
	logger          zerolog.Logger

	closed atomic.Bool //closed debugger
}

type DebuggerArgs struct {
	Logger             zerolog.Logger //ok if not set
	InitialBreakpoints []BreakpointInfo

	//cancelling this context will cause the debugger to close.
	//the debugger uses this context's filesystem.
	Context *Context
}

func NewDebugger(args DebuggerArgs) *Debugger {

	initialBreakpoints := map[parse.NodeSpan]BreakpointInfo{}
	nextBreakpointId := 1

	for _, breakpoint := range args.InitialBreakpoints {
		nextBreakpointId = utils.Max(nextBreakpointId, int(breakpoint.Id)+1)
		if breakpoint.NodeSpan != (parse.NodeSpan{}) {
			initialBreakpoints[breakpoint.NodeSpan] = breakpoint
		}
	}

	return &Debugger{
		ctx:                       args.Context,
		controlChan:               make(chan any),
		stoppedProgramCommandChan: make(chan any),
		stoppedProgramChan:        make(chan ProgramStoppedEvent, 1),
		resumeExecutionChan:       make(chan struct{}),
		logger:                    args.Logger,

		nextBreakpointId: INITIAL_BREAKPOINT_ID,
		breakpoints:      initialBreakpoints,
	}
}

// StoppedChan returns a channel that sends an item each time the program stops.
func (d *Debugger) StoppedChan() chan ProgramStoppedEvent {
	return d.stoppedProgramChan
}

// ControlChan returns a channel to which debug commands should be sent.
func (d *Debugger) ControlChan() chan any {
	return d.controlChan
}

// ControlChan returns a channel to which debug commands should be sent.
func (d *Debugger) Closed() bool {
	return d.closed.Load()
}

// AttachAndStart attaches the debugger to state & starts the debugging goroutine.
func (d *Debugger) AttachAndStart(state EvaluationState) {
	state.AttachDebugger(d)
	d.globalState = state.GetGlobalState()
	d.evaluationState = state
	d.startGoroutine()
}

func (d *Debugger) startGoroutine() {
	d.logger.Info().Msg("start debugging")

	go func() {
		var done func()
		cancelExecution := false

		defer func() {
			d.logger.Info().Msg("stop debugging")
			d.closed.Store(true)

			d.breakpointsLock.Lock()
			d.breakpoints = nil
			d.breakpointsLock.Unlock()

			close(d.stoppedProgramCommandChan)
			close(d.resumeExecutionChan)

			if cancelExecution {
				d.logger.Info().Msg("cancel execution of debuggee")
				go d.globalState.Ctx.Cancel()
			}
			if done != nil {
				done()
			}
		}()

		for {
			//TODO: empty stoppedProgramCommandChan if program not stopped

			select {
			case <-d.globalState.Ctx.Done():
				return
			case cmd := <-d.controlChan:
				switch c := cmd.(type) {
				case DebugCommandCloseDebugger:
					done = c.Done
					cancelExecution = c.CancelExecution
					return
				case DebugCommandSetBreakpoints:
					var (
						breakpoints          = map[parse.NodeSpan]BreakpointInfo{}
						breakpointsSetByLine []BreakpointInfo
						chunk                = c.Chunk
					)

					func() {
						d.breakpointsLock.Lock()
						defer d.breakpointsLock.Unlock()

						for node := range c.BreakpointsAtNode {
							id := d.nextBreakpointId
							d.nextBreakpointId++

							line, col := chunk.GetLineColumn(node)

							breakpoints[node.Base().Span] = BreakpointInfo{
								NodeSpan:    node.Base().Span,
								Chunk:       chunk,
								Id:          id,
								StartLine:   line,
								StartColumn: col,
							}
						}

						breakpointsFromLines, err := GetBreakpointsFromLines(c.BreakPointsByLine, chunk, &d.nextBreakpointId)

						if err == nil {
							for _, breakpoint := range breakpointsFromLines {
								if breakpoint.NodeSpan != (parse.NodeSpan{}) {
									breakpoints[breakpoint.NodeSpan] = breakpoint
								}
							}
						} else {
							d.logger.Err(err).Msg("failed to get breakpoints from lines")
						}

						d.breakpoints = breakpoints
					}()

					if c.GetBreakpointsSetByLine != nil {
						c.GetBreakpointsSetByLine(breakpointsSetByLine)
					}

				case DebugCommandPause:
					if d.stoppedProgram.Load() {
						continue
					}
					d.logger.Info().Msg("pause")
					d.stopBeforeNextStatement.Store(PauseStop)
				case DebugCommandContinue:
					if d.stoppedProgram.Load() {
						d.logger.Info().Msg("continue")
						d.resumeExecutionChan <- struct{}{}
					}
				case DebugCommandNextStep:
					if !d.stoppedProgram.Load() {
						continue
					}
					d.stopBeforeNextStatement.Store(StepStop)
					d.resumeExecutionChan <- struct{}{}
				case DebugCommandGetScopes, DebugCommandGetStackTrace:
					if d.stoppedProgram.Load() {
						d.stoppedProgramCommandChan <- c
					}
				}
			}
		}
	}()
}

func (d *Debugger) beforeInstruction(n parse.Node, trace []StackFrameInfo) {
	if d.closed.Load() {
		return
	}

	trace = utils.CopySlice(trace)

	d.breakpointsLock.Lock()
	breakpointInfo, hasBreakpoint := d.breakpoints[n.Base().Span]
	d.breakpointsLock.Unlock()

	var stopReason ProgramStopReason
	if hasBreakpoint {
		stopReason = BreakpointStop
	} else {
		stopReason, _ = d.stopBeforeNextStatement.Swap(ProgramStopReason(0)).(ProgramStopReason)
	}

	if stopReason > 0 {
		d.stoppedProgram.Store(true)
		event := ProgramStoppedEvent{Reason: stopReason}
		if hasBreakpoint {
			event.Breakpoint = &breakpointInfo
		}
		d.stoppedProgramChan <- event

		for {
			select {
			case <-d.globalState.Ctx.Done():
				panic(d.globalState.Ctx.Err())
			case _, ok := <-d.resumeExecutionChan:
				d.stoppedProgram.Store(false)
				if !ok { //debugger closed
					d.evaluationState.DetachDebugger()
					close(d.stoppedProgramChan)
				}

				return
			case cmd, ok := <-d.stoppedProgramCommandChan:
				if !ok { //debugger closed
					d.evaluationState.DetachDebugger()
					close(d.stoppedProgramChan)
					return
				}

				switch c := cmd.(type) {
				case DebugCommandGetScopes:
					globals := d.globalState.Globals.Entries()
					locals := utils.CopyMap(d.evaluationState.CurrentLocalScope())
					c.Get(globals, locals)
				case DebugCommandGetStackTrace:
					c.Get(trace)
				}
			}
		}
	}

}

func ParseFileChunk(absoluteSourcePath string, fls afs.Filesystem) (*parse.ParsedChunk, error) {
	content, err := ReadFileInFS(fls, absoluteSourcePath, -1)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", absoluteSourcePath, err)
	}

	src := parse.SourceFile{
		NameString:    absoluteSourcePath,
		Resource:      absoluteSourcePath,
		ResourceDir:   filepath.Dir(absoluteSourcePath),
		IsResourceURL: false,
		CodeString:    string(content),
	}

	chunk, parsingErr := parse.ParseChunkSource(src)
	if parsingErr != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", absoluteSourcePath, parsingErr)
	}
	return chunk, nil
}

func GetBreakpointsFromLines(lines []int, chunk *parse.ParsedChunk, nextBreakpointId *int32) ([]BreakpointInfo, error) {
	var breakpointsSetByLine []BreakpointInfo

	for _, line := range lines {
		stmt, _, _ := chunk.FindFirstStatementAndChainOnLine(line)

		id := *nextBreakpointId
		*nextBreakpointId = *nextBreakpointId + 1

		breakpointInfo := BreakpointInfo{
			NodeSpan: stmt.Base().Span,
			Chunk:    chunk,
			Id:       id,
		}

		if stmt != nil {
			line, col := chunk.GetLineColumn(stmt)
			breakpointInfo.StartLine = line
			breakpointInfo.StartColumn = col
		}

		breakpointsSetByLine = append(breakpointsSetByLine, breakpointInfo)
	}
	return breakpointsSetByLine, nil
}
