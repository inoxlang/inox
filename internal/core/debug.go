package core

import (
	"sync"
	"sync/atomic"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

type Breakpoint struct {
	node  parse.Node
	chunk *parse.ParsedChunk
}

type DebugCommandSetBreakpoints struct {
	Breakpoints map[parse.Node]struct{}
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

type Debugger struct {
	controlChan               chan any
	stoppedProgramCommandChan chan any
	stoppedProgramChan        chan struct{}
	stoppedProgram            atomic.Bool
	stopBeforeNextStatement   atomic.Bool
	breakpoints               map[parse.Node]struct{}
	breakpointsLock           sync.Mutex
	resumeExecutionChan       chan struct{}

	globalState *GlobalState
	logger      zerolog.Logger

	closed atomic.Bool //closed debugger
}

type DebuggerArgs struct {
	Logger zerolog.Logger //ok if not set
}

func NewDebugger(args DebuggerArgs) *Debugger {
	return &Debugger{
		controlChan:               make(chan any),
		stoppedProgramCommandChan: make(chan any),
		stoppedProgramChan:        make(chan struct{}, 1),
		resumeExecutionChan:       make(chan struct{}),
		logger:                    args.Logger,
	}
}

// StoppedChan returns a channel that sends an item each time the program stops.
func (d *Debugger) StoppedChan() chan struct{} {
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
					d.breakpointsLock.Lock()
					d.breakpoints = c.Breakpoints
					d.breakpointsLock.Unlock()
				case DebugCommandPause:
					if d.stoppedProgram.Load() {
						continue
					}
					d.stopBeforeNextStatement.Store(true)
				case DebugCommandContinue:
					if d.stoppedProgram.Load() {
						d.resumeExecutionChan <- struct{}{}
					}
				case DebugCommandNextStep:
					if !d.stoppedProgram.Load() {
						continue
					}
					d.stopBeforeNextStatement.Store(true)
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

func (d *Debugger) beforeInstruction(n parse.Node, state *TreeWalkState) {
	if d.closed.Load() {
		return
	}

	d.breakpointsLock.Lock()
	_, ok := d.breakpoints[n]
	d.breakpointsLock.Unlock()

	if ok || d.stopBeforeNextStatement.CompareAndSwap(true, false) {
		d.stoppedProgram.Store(true)
		d.stoppedProgramChan <- struct{}{}
		for {
			select {
			case <-d.globalState.Ctx.Done():
				panic(d.globalState.Ctx.Err())
			case _, ok := <-d.resumeExecutionChan:
				d.stoppedProgram.Store(false)
				if !ok { //debugger closed
					state.debug = nil
					close(d.stoppedProgramChan)
				}

				return
			case cmd, ok := <-d.stoppedProgramCommandChan:
				if !ok { //debugger closed
					state.debug = nil
					close(d.stoppedProgramChan)
					return
				}

				switch c := cmd.(type) {
				case DebugCommandGetScopes:
					globals := state.Global.Globals.Entries()
					locals := utils.CopyMap(state.CurrentLocalScope())
					c.Get(globals, locals)
				}
			}
		}
	}

}
