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

type Debugger struct {
	controlChan               chan any
	stoppedProgramCommandChan chan any
	stoppedChan               chan struct{}
	stopped                   atomic.Bool
	stopBeforeNextStatement   atomic.Bool
	breakpoints               map[parse.Node]struct{}
	breakpointsLock           sync.Mutex
	resumeExecutionChan       chan struct{}

	globalState *GlobalState
	logger      zerolog.Logger
}

type DebuggerArgs struct {
	Logger zerolog.Logger //ok if not set
}

func NewDebugger(args DebuggerArgs) *Debugger {
	return &Debugger{
		controlChan:               make(chan any),
		stoppedProgramCommandChan: make(chan any),
		stoppedChan:               make(chan struct{}, 1),
		resumeExecutionChan:       make(chan struct{}),
		logger:                    args.Logger,
	}
}

// StoppedChan returns a channel that sends an item each time the program stops.
func (d *Debugger) StoppedChan() chan struct{} {
	return d.stoppedChan
}

// ControlChan returns a channel to which debug commands should be sent.
func (d *Debugger) ControlChan() chan any {
	return d.controlChan
}

func (d *Debugger) startGoroutine() {
	d.logger.Info().Msg("start debugging")

	go func() {
		for {
			//TODO: empty stoppedProgramCommandChan if program not stopped

			select {
			case <-d.globalState.Ctx.Done():
				return
			case cmd := <-d.controlChan:
				switch c := cmd.(type) {
				case DebugCommandSetBreakpoints:
					d.breakpointsLock.Lock()
					d.breakpoints = c.Breakpoints
					d.breakpointsLock.Unlock()
				case DebugCommandPause:
					if d.stopped.Load() {
						continue
					}
					d.stopBeforeNextStatement.Store(true)
				case DebugCommandContinue:
					if d.stopped.Load() {
						d.resumeExecutionChan <- struct{}{}
					}
				case DebugCommandNextStep:
					if !d.stopped.Load() {
						continue
					}
					d.stopBeforeNextStatement.Store(true)
					d.resumeExecutionChan <- struct{}{}
				case DebugCommandGetScopes:
					if d.stopped.Load() {
						d.stoppedProgramCommandChan <- c
					}
				}
			}
		}
	}()
}

func (d *Debugger) beforeInstruction(n parse.Node, state *TreeWalkState) {
	d.breakpointsLock.Lock()
	_, ok := d.breakpoints[n]
	d.breakpointsLock.Unlock()

	if ok || d.stopBeforeNextStatement.CompareAndSwap(true, false) {
		d.stopped.Store(true)
		d.stoppedChan <- struct{}{}
	loop:
		for {
			select {
			case <-d.globalState.Ctx.Done():
				panic(d.globalState.Ctx.Err())
			case <-d.resumeExecutionChan:
				d.stopped.Store(false)
				break loop
			case cmd := <-d.stoppedProgramCommandChan:
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
