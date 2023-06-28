package core

import (
	"errors"
	"sync"
	"sync/atomic"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

var (
	ErrDebuggerAlreadyAttached = errors.New("debugger already attached")
)

type BreakpointInfo struct {
	Node  parse.Node //can be nil
	Chunk *parse.ParsedChunk
}

func (i BreakpointInfo) Verified() bool {
	return i.Node != nil
}

type DebugCommandSetBreakpoints struct {
	Breakpoints       map[parse.Node]struct{}
	BreakPointsByLine []int

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

type DebugCommandCloseDebugger struct {
}

type ProgramStoppedEvent struct {
	Reason ProgramStopReason
}

type ProgramStopReason int

const (
	PauseStop ProgramStopReason = 1 + iota
	StepStop
	BreakpointStop
)

type Debugger struct {
	controlChan               chan any
	stoppedProgramCommandChan chan any
	stoppedProgramChan        chan ProgramStoppedEvent
	stoppedProgram            atomic.Bool
	stopBeforeNextStatement   atomic.Value //non-breakpoint ProgramStopReason
	breakpoints               map[parse.Node]struct{}
	breakpointsLock           sync.Mutex
	resumeExecutionChan       chan struct{}

	evaluationState EvaluationState
	globalState     *GlobalState
	module          *Module
	logger          zerolog.Logger

	closed atomic.Bool //closed debugger
}

type EvaluationState interface {
	//AttachDebugger should be called before starting the evaluation.
	AttachDebugger(*Debugger)

	//DetachDebugger should be called by the evaluation's goroutine.
	DetachDebugger()

	CurrentLocalScope() map[string]Value

	GetGlobalState() *GlobalState
}

type DebuggerArgs struct {
	Logger zerolog.Logger //ok if not set
}

func NewDebugger(args DebuggerArgs) *Debugger {
	return &Debugger{
		controlChan:               make(chan any),
		stoppedProgramCommandChan: make(chan any),
		stoppedProgramChan:        make(chan ProgramStoppedEvent, 1),
		resumeExecutionChan:       make(chan struct{}),
		logger:                    args.Logger,
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
		defer func() {
			d.logger.Info().Msg("stop debugging")
			d.closed.Store(true)

			d.breakpointsLock.Lock()
			d.breakpoints = nil
			d.breakpointsLock.Unlock()

			close(d.stoppedProgramCommandChan)
			close(d.resumeExecutionChan)
		}()

		for {
			//TODO: empty stoppedProgramCommandChan if program not stopped

			select {
			case <-d.globalState.Ctx.Done():
				return
			case cmd := <-d.controlChan:
				switch c := cmd.(type) {
				case DebugCommandCloseDebugger:
					return
				case DebugCommandSetBreakpoints:
					breakpointLocations := utils.CopyMap(c.Breakpoints)

					var breakpointsSetByLine []BreakpointInfo
					mainChunk := d.globalState.Module.MainChunk

					for _, line := range c.BreakPointsByLine {
						stmt, _, _ := mainChunk.FindFirstStatementAndChainOnLine(line)

						breakpointsSetByLine = append(breakpointsSetByLine, BreakpointInfo{
							Node:  stmt,
							Chunk: mainChunk,
						})

						if stmt != nil {
							breakpointLocations[stmt] = struct{}{}
						}
					}

					d.breakpointsLock.Lock()
					d.breakpoints = breakpointLocations
					d.breakpointsLock.Unlock()

					if c.GetBreakpointsSetByLine != nil {
						c.GetBreakpointsSetByLine(breakpointsSetByLine)
					}

				case DebugCommandPause:
					if d.stoppedProgram.Load() {
						continue
					}
					d.stopBeforeNextStatement.Store(PauseStop)
				case DebugCommandContinue:
					if d.stoppedProgram.Load() {
						d.resumeExecutionChan <- struct{}{}
					}
				case DebugCommandNextStep:
					if !d.stoppedProgram.Load() {
						continue
					}
					d.stopBeforeNextStatement.Store(StepStop)
					d.resumeExecutionChan <- struct{}{}
				case DebugCommandGetScopes:
					if d.stoppedProgram.Load() {
						d.stoppedProgramCommandChan <- c
					}
				}
			}
		}
	}()
}

func (d *Debugger) beforeInstruction(n parse.Node) {
	if d.closed.Load() {
		return
	}

	d.breakpointsLock.Lock()
	_, breakpoint := d.breakpoints[n]
	d.breakpointsLock.Unlock()

	var stopReason ProgramStopReason
	if breakpoint {
		stopReason = BreakpointStop
	} else {
		stopReason, _ = d.stopBeforeNextStatement.Swap(ProgramStopReason(0)).(ProgramStopReason)
	}

	if stopReason > 0 {
		d.stoppedProgram.Store(true)
		d.stoppedProgramChan <- ProgramStoppedEvent{Reason: stopReason}

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
				}
			}
		}
	}

}
