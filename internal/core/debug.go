package core

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
	"golang.org/x/exp/maps"
)

const (
	FUNCTION_FRAME_PREFIX = "(fn) "

	//starts at 1 for compatibility with the Debug Adapter Protocol
	INITIAL_BREAKPOINT_ID = 1

	SECONDARY_EVENT_CHAN_CAP = 1000
)

var (
	ErrDebuggerAlreadyAttached = errors.New("debugger already attached")
)

// A Debugger enables the debugging of a running Inox program by handling debug commands
// sent to its control channel (ControlChan()). Events should be continuously read from
// StoppedChan() and SecondaryEventsChan() by the user of Debugger.
//
// Commands are handled in a separate goroutine that is created by the AttachAndStart method.
type Debugger struct {
	ctx                       *Context
	controlChan               chan any
	secondaryEventChan        chan SecondaryDebugEvent
	stoppedProgramCommandChan chan any
	stoppedProgram            atomic.Bool
	stopBeforeNextStatement   atomic.Value //non-breakpoint ProgramStopReason
	stackFrameDepth           atomic.Int32
	steppingDepth             atomic.Int32

	shared              *sharedDebuggerFields
	resumeExecutionChan chan struct{}

	evaluationState evaluationState
	globalState     *GlobalState //evaluationState's GlobalState
	logger          zerolog.Logger

	parent *Debugger //nil if root debugger

	closed atomic.Bool //closed debugger
}

// evaluationState is implemented by the state of Inox interpreters supporting debugging.
type evaluationState interface {
	//AttachDebugger should be called before starting the evaluation.
	AttachDebugger(*Debugger)

	//DetachDebugger should be called by the evaluation's goroutine.
	DetachDebugger()

	CurrentLocalScope() map[string]Value

	GetGlobalState() *GlobalState
}

// sharedDebuggerFields represents the state shared by a debugger and its descendant debuggers.
type sharedDebuggerFields struct {
	nextBreakpointId       int32
	breakpointsLock        sync.Mutex
	breakpoints            map[parse.NodeSpan]BreakpointInfo
	exceptionBreakpointsId atomic.Int32

	stackFrameId       atomic.Int32 //incremented by debuggees
	threadIdToFrameIds map[StateId]*[]int32
	frameIdMappingLock sync.Mutex

	stoppedEventChan chan ProgramStoppedEvent

	debuggers     map[StateId]*Debugger
	debuggersLock sync.Mutex
}

func (f *sharedDebuggerFields) getDebuggerOfThread(id StateId) *Debugger {
	f.debuggersLock.Lock()
	defer f.debuggersLock.Unlock()

	return f.debuggers[id]
}

func (f *sharedDebuggerFields) getNextStackFrameId() int32 {
	return f.stackFrameId.Add(1)
}

func (f *sharedDebuggerFields) updateFrameIdMapping(threadId StateId, trace []StackFrameInfo) {
	f.frameIdMappingLock.Lock()
	defer f.frameIdMappingLock.Unlock()

	frameIds := f.threadIdToFrameIds[threadId]

	if frameIds == nil {
		frameIds = new([]int32)
		f.threadIdToFrameIds[threadId] = frameIds
	}

	for i, frame := range trace {
		if i >= len(*frameIds) {
			*frameIds = append(*frameIds, frame.Id)
		} else {
			(*frameIds)[i] = frame.Id
		}
	}
}

type DebuggerArgs struct {
	Logger             zerolog.Logger //ok if not set
	InitialBreakpoints []BreakpointInfo

	//if not set exception breakpoints are not enabled,
	// this argument is ignored if parent is set
	ExceptionBreakpointId int32

	//cancelling this context will cause the debugger to close.
	//the debugger uses this context's filesystem.
	Context *Context

	parent *Debugger
}

func NewDebugger(args DebuggerArgs) *Debugger {

	initialBreakpoints := map[parse.NodeSpan]BreakpointInfo{}
	nextBreakpointId := int32(1)

	for _, breakpoint := range args.InitialBreakpoints {
		nextBreakpointId = max(nextBreakpointId, breakpoint.Id+1)
		if breakpoint.NodeSpan != (parse.NodeSpan{}) {
			initialBreakpoints[breakpoint.NodeSpan] = breakpoint
		}
	}

	debugger := &Debugger{
		ctx:                       args.Context,
		controlChan:               make(chan any),
		secondaryEventChan:        make(chan SecondaryDebugEvent, SECONDARY_EVENT_CHAN_CAP),
		stoppedProgramCommandChan: make(chan any),
		resumeExecutionChan:       make(chan struct{}),
		logger:                    args.Logger,
	}
	debugger.steppingDepth.Store(1)

	if args.parent != nil {
		debugger.shared = args.parent.shared
		debugger.parent = args.parent
	} else { //root debugger
		if args.ExceptionBreakpointId > nextBreakpointId {
			nextBreakpointId = args.ExceptionBreakpointId + 1
		}

		debugger.shared = &sharedDebuggerFields{
			nextBreakpointId:   nextBreakpointId,
			breakpoints:        initialBreakpoints,
			stoppedEventChan:   make(chan ProgramStoppedEvent, runtime.NumCPU()),
			debuggers:          make(map[StateId]*Debugger),
			threadIdToFrameIds: make(map[StateId]*[]int32, 0),
		}

		if args.ExceptionBreakpointId >= INITIAL_BREAKPOINT_ID {
			debugger.shared.exceptionBreakpointsId.Store(args.ExceptionBreakpointId)
		}
	}

	return debugger
}

func (d *Debugger) threadId() StateId {
	return d.globalState.id
}

func (d *Debugger) isRoot() bool {
	return d.parent == nil
}

func (d *Debugger) NewChild() *Debugger {
	d.shared.breakpointsLock.Lock()
	breakpoints := maps.Values(d.shared.breakpoints)
	defer d.shared.breakpointsLock.Unlock()

	child := NewDebugger(DebuggerArgs{
		Logger:             d.logger,
		InitialBreakpoints: breakpoints,
		Context:            d.ctx,
		parent:             d,
	})

	return child
}

// StoppedChan returns a channel that sends an item each time the program stops.
func (d *Debugger) StoppedChan() chan ProgramStoppedEvent {
	return d.shared.stoppedEventChan
}

// ControlChan returns a channel to which debug commands should be sent.
func (d *Debugger) ControlChan() chan any {
	return d.controlChan
}

// SecondaryEventsChan returns a channel that sends secondary events received by the debugger.
func (d *Debugger) SecondaryEventsChan() chan SecondaryDebugEvent {
	return d.secondaryEventChan
}

// ControlChan returns a channel to which debug commands should be sent.
func (d *Debugger) Closed() bool {
	return d.closed.Load()
}

func (d *Debugger) ExceptionBreakpointsId() (_ int32, enabled bool) {
	id := d.shared.exceptionBreakpointsId.Load()
	if id >= INITIAL_BREAKPOINT_ID {
		return id, true
	}
	return 0, false
}

func (d *Debugger) ThreadIfOfStackFrame(stackFrameId int32) (StateId, bool) {
	d.shared.frameIdMappingLock.Lock()
	defer d.shared.frameIdMappingLock.Unlock()

	for threadId, frameIds := range d.shared.threadIdToFrameIds {
		for _, frameId := range *frameIds {
			if frameId == stackFrameId {
				return threadId, true
			}
		}
	}
	return 0, false
}

// AttachAndStart attaches the debugger to state & starts the debugging goroutine.
func (d *Debugger) AttachAndStart(state evaluationState) {
	state.AttachDebugger(d)
	d.globalState = state.GetGlobalState()
	d.evaluationState = state

	d.shared.debuggersLock.Lock()
	d.shared.debuggers[d.threadId()] = d
	d.shared.debuggersLock.Unlock()

	d.startGoroutine()
}

func (d *Debugger) broadcastCommand(cmd any) {
	d.shared.debuggersLock.Lock()
	defer d.shared.debuggersLock.Unlock()

	for _, debugger := range d.shared.debuggers {
		if debugger == d {
			continue
		}
		debugger.controlChan <- cmd
	}
}

func (d *Debugger) sendCommandToTargetDebugger(cmd any, threadId StateId) {
	if threadId < MINIMAL_STATE_ID {
		return
	}

	d.shared.debuggersLock.Lock()
	defer d.shared.debuggersLock.Unlock()

	for _, debugger := range d.shared.debuggers {
		if debugger.threadId() == threadId {
			debugger.controlChan <- cmd
			break
		}
	}
}

func (d *Debugger) sendCommandToRootDebugger(cmd any) {
	d.shared.debuggersLock.Lock()
	defer d.shared.debuggersLock.Unlock()

	for _, debugger := range d.shared.debuggers {
		if debugger.isRoot() {
			debugger.controlChan <- cmd
			break
		}
	}
}

func (d *Debugger) Threads() (threads []ThreadInfo) {
	d.shared.debuggersLock.Lock()
	defer d.shared.debuggersLock.Unlock()

	for _, debugger := range d.shared.debuggers {
		threads = append(threads, ThreadInfo{
			Name: debugger.globalState.Module.Name(),
			Id:   debugger.threadId(),
		})
	}

	sort.Slice(threads, func(i, j int) bool {
		return threads[i].Id < threads[j].Id
	})

	return
}

func (d *Debugger) startGoroutine() {
	d.logger.Info().Msgf("start debugging thread %d (%s)", d.threadId(), d.globalState.Module.Name())

	go func() {
		var done func()
		cancelExecution := false

		defer func() {
			d.logger.Info().Msgf("stop debugging thread %d (%s)", d.threadId(), d.globalState.Module.Name())
			d.closed.Store(true)

			if d.isRoot() {
				d.shared.breakpointsLock.Lock()
				d.shared.breakpoints = nil
				d.shared.breakpointsLock.Unlock()
			}

			close(d.stoppedProgramCommandChan)
			close(d.secondaryEventChan)
			close(d.resumeExecutionChan)

			d.shared.debuggersLock.Lock()
			delete(d.shared.debuggers, d.threadId())
			d.shared.debuggersLock.Unlock()

			if cancelExecution {
				d.logger.Info().Msg("cancel execution of debuggee")
				go d.globalState.Ctx.CancelGracefully()
			}
			if done != nil {
				done()
			}
		}()

		d.loop(&done, &cancelExecution)
	}()
}

func (d *Debugger) loop(done *func(), cancelExecution *bool) {
	for {
		//TODO: empty stoppedProgramCommandChan if program not stopped

		select {
		case <-d.globalState.Ctx.Done():
			return
		case cmd := <-d.controlChan:
			if d.isRoot() {
				switch c := cmd.(type) {
				case DebugCommandContinue:
					if c.ResumeAllThreads {
						d.broadcastCommand(c)
					} else if c.ThreadId != d.threadId() {
						d.sendCommandToTargetDebugger(c, c.ThreadId)
						continue
					}
				case DebugCommandGetStackTrace:
					if c.ThreadId != d.threadId() {
						d.sendCommandToTargetDebugger(c, c.ThreadId)
						continue
					}
				case DebugCommandGetScopes:
					if c.ThreadId != d.threadId() {
						d.sendCommandToTargetDebugger(c, c.ThreadId)
						continue
					}
				case DebugCommandNextStep:
					if c.ResumeAllThreads {
						d.broadcastCommand(c)
					} else if c.ThreadId != d.threadId() {
						d.sendCommandToTargetDebugger(c, c.ThreadId)
						continue
					}
				case DebugCommandStepIn:
					if c.ResumeAllThreads {
						d.broadcastCommand(c)
					} else if c.ThreadId != d.threadId() {
						d.sendCommandToTargetDebugger(c, c.ThreadId)
						continue
					}
				case DebugCommandStepOut:
					if c.ResumeAllThreads {
						d.broadcastCommand(c)
					} else if c.ThreadId != d.threadId() {
						d.sendCommandToTargetDebugger(c, c.ThreadId)
						continue
					}
				}
			}

			switch c := cmd.(type) {
			case DebugCommandCloseDebugger:
				*done = c.Done
				*cancelExecution = c.CancelExecution
				return
			case DebugCommandSetBreakpoints:
				var (
					breakpoints          = map[parse.NodeSpan]BreakpointInfo{}
					breakpointsSetByLine []BreakpointInfo
					chunk                = c.Chunk
				)

				func() {
					d.shared.breakpointsLock.Lock()
					defer d.shared.breakpointsLock.Unlock()

					for node := range c.BreakpointsAtNode {
						id := d.shared.nextBreakpointId
						d.shared.nextBreakpointId++

						line, col := chunk.GetLineColumn(node)

						breakpoints[node.Base().Span] = BreakpointInfo{
							NodeSpan:    node.Base().Span,
							Chunk:       chunk,
							Id:          id,
							StartLine:   line,
							StartColumn: col,
						}
					}

					breakpointsFromLines, err := GetBreakpointsFromLines(c.BreakPointsByLine, chunk, &d.shared.nextBreakpointId)

					if err == nil {
						for _, breakpoint := range breakpointsFromLines {
							if breakpoint.NodeSpan != (parse.NodeSpan{}) {
								breakpoints[breakpoint.NodeSpan] = breakpoint
							}
						}
						breakpointsSetByLine = append(breakpointsSetByLine, breakpointsFromLines...)
					} else {
						d.logger.Err(err).Msg("failed to get breakpoints from lines")
					}

					d.shared.breakpoints = breakpoints
				}()

				if c.GetBreakpointsSetByLine != nil {
					c.GetBreakpointsSetByLine(breakpointsSetByLine)
				}

			case DebugCommandSetExceptionBreakpoints:
				if c.Disable {
					d.shared.exceptionBreakpointsId.Store(0)
					continue
				}
				//enable

				d.shared.breakpointsLock.Lock()
				id := d.shared.nextBreakpointId
				d.shared.nextBreakpointId++
				d.shared.breakpointsLock.Unlock()

				d.shared.exceptionBreakpointsId.Store(id)

				if c.GetExceptionBreakpointId != nil {
					c.GetExceptionBreakpointId(id)
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
					depth := d.stackFrameDepth.Load()
					d.steppingDepth.Store(depth)
					d.resumeExecutionChan <- struct{}{}
				}
			case DebugCommandNextStep:
				if !d.stoppedProgram.Load() {
					continue
				}
				depth := d.stackFrameDepth.Load()
				d.steppingDepth.Store(depth)
				d.stopBeforeNextStatement.Store(NextStepStop)
				d.resumeExecutionChan <- struct{}{}
			case DebugCommandStepIn:
				if !d.stoppedProgram.Load() {
					continue
				}
				d.stopBeforeNextStatement.Store(StepInStop)
				depth := d.stackFrameDepth.Load()
				d.steppingDepth.Store(depth + 1)
				d.resumeExecutionChan <- struct{}{}
			case DebugCommandStepOut:
				if !d.stoppedProgram.Load() {
					continue
				}
				d.stopBeforeNextStatement.Store(StepOutStop)
				depth := d.stackFrameDepth.Load()
				d.steppingDepth.Store(depth - 1)
				d.resumeExecutionChan <- struct{}{}
			case DebugCommandGetScopes, DebugCommandGetStackTrace:
				if d.stoppedProgram.Load() {
					d.stoppedProgramCommandChan <- c
				}
			case DebugCommandInformAboutSecondaryEvent:
				if !d.isRoot() {
					d.sendCommandToRootDebugger(c)
					continue
				}
				//if the channel is full we drop the event.
				//note: this kind of check can be done because:
				// - there is a single piece of code that write to this channel.
				// - if the channels happens to be read just after the check it's okay.
				if len(d.secondaryEventChan) == cap(d.secondaryEventChan) {
					continue
				}

				d.secondaryEventChan <- c.Event
			}
		}
	}
}

func (d *Debugger) beforeInstruction(n parse.Node, trace []StackFrameInfo, exceptionError error) {
	if d.closed.Load() {
		return
	}

	trace = utils.ReversedSlice(trace)

	depthIncrease := false
	depthDecrease := false

	{
		prevDepth := d.stackFrameDepth.Swap(int32(len(trace)))
		if prevDepth != 0 {
			depthIncrease = prevDepth < int32(len(trace))
			depthDecrease = prevDepth > int32(len(trace))
		}
	}

	d.shared.updateFrameIdMapping(d.threadId(), trace)

	var (
		stopReason     ProgramStopReason
		breakpointInfo BreakpointInfo
		hasBreakpoint  bool
	)

	if exceptionError == nil {
		d.shared.breakpointsLock.Lock()
		breakpointInfo, hasBreakpoint = d.shared.breakpoints[n.Base().Span]
		d.shared.breakpointsLock.Unlock()

		if hasBreakpoint {
			stopReason = BreakpointStop
		} else {
			steppingDepth := d.steppingDepth.Load()

			stopReason, _ = d.stopBeforeNextStatement.Swap(ProgramStopReason(0)).(ProgramStopReason)

			switch stopReason {
			case StepOutStop:
				if !depthDecrease || len(trace) > int(steppingDepth) {
					stopReason = 0
					d.stopBeforeNextStatement.CompareAndSwap(ProgramStopReason(0), StepOutStop) //okay if fail
				}
			case NextStepStop:
				if depthIncrease || len(trace) > int(steppingDepth) {
					stopReason = 0
					d.stopBeforeNextStatement.CompareAndSwap(ProgramStopReason(0), NextStepStop) //okay if fail
				}
			}
		}
	} else if id, enabled := d.ExceptionBreakpointsId(); enabled {
		stopReason = ExceptionBreakpointStop

		chunk := trace[len(trace)-1].Chunk
		line, col := chunk.GetLineColumn(n)

		hasBreakpoint = true

		breakpointInfo = BreakpointInfo{
			Id:          id,
			NodeSpan:    n.Base().Span,
			Chunk:       chunk,
			StartLine:   line,
			StartColumn: col,
		}
	}

	if stopReason > 0 {
		d.stoppedProgram.Store(true)
		event := ProgramStoppedEvent{
			ThreadId:       d.threadId(),
			Reason:         stopReason,
			ExceptionError: exceptionError,
		}
		if hasBreakpoint {
			event.Breakpoint = &breakpointInfo
		}
		d.shared.stoppedEventChan <- event

		//while the program is stopped we handle commands
		//such as DebugCommandGetScopes or DebugCommandGetStackTrace.
		for {
			select {
			case <-d.globalState.Ctx.Done():
				panic(d.globalState.Ctx.Err())
			case _, ok := <-d.resumeExecutionChan:
				d.stoppedProgram.Store(false)
				if !ok { //debugger closed
					d.evaluationState.DetachDebugger()
					if d.isRoot() {
						close(d.shared.stoppedEventChan)
					}
				}

				return
			case cmd, ok := <-d.stoppedProgramCommandChan:
				if !ok { //debugger closed
					d.evaluationState.DetachDebugger()
					if d.isRoot() {
						close(d.shared.stoppedEventChan)
					}
					return
				}

				switch c := cmd.(type) {
				case DebugCommandGetScopes:
					globals := d.globalState.Globals.Entries()
					locals := maps.Clone(d.evaluationState.CurrentLocalScope())
					c.Get(globals, locals)
				case DebugCommandGetStackTrace:
					c.Get(trace)
				}
			}
		}
	}

}

func ParseFileChunk(absoluteSourcePath string, fls afs.Filesystem, opts ...parse.ParserOptions) (*parse.ParsedChunkSource, error) {
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

	chunk, parsingErr := parse.ParseChunkSource(src, opts...)

	if parsingErr != nil {
		return chunk, fmt.Errorf("failed to parse %s: %w", absoluteSourcePath, parsingErr)
	}
	return chunk, nil
}

func GetBreakpointsFromLines(lines []int, chunk *parse.ParsedChunkSource, nextBreakpointId *int32) ([]BreakpointInfo, error) {
	var breakpointsSetByLine []BreakpointInfo

	for _, line := range lines {
		stmt, _, _ := chunk.FindFirstStatementAndChainOnLine(line)

		id := *nextBreakpointId
		*nextBreakpointId = *nextBreakpointId + 1

		breakpointInfo := BreakpointInfo{
			Chunk: chunk,
			Id:    id,
		}

		if stmt != nil {
			line, col := chunk.GetLineColumn(stmt)
			breakpointInfo.NodeSpan = stmt.Base().Span
			breakpointInfo.StartLine = line
			breakpointInfo.StartColumn = col
		}

		breakpointsSetByLine = append(breakpointsSetByLine, breakpointInfo)
	}
	return breakpointsSetByLine, nil
}
