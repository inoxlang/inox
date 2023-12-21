package core

import (
	"github.com/inoxlang/inox/internal/parse"
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
	Name string

	//can be nil, current *Chunk | *FunctionExpression or statement (current statement if we are stopped at a breakpoint exception)
	Node parse.Node

	Chunk       *parse.ParsedChunk
	Id          int32 //set if debugging, unique for a given debugger tree (~ session)
	StartLine   int32
	StartColumn int32

	StatementStartLine   int32
	StatementStartColumn int32
}

type ThreadInfo struct {
	Name string
	Id   StateId
}

// Primary Events

type ProgramStoppedEvent struct {
	ThreadId       StateId
	Reason         ProgramStopReason
	Breakpoint     *BreakpointInfo
	ExceptionError error
}

type ProgramStopReason int

const (
	PauseStop ProgramStopReason = 1 + iota
	NextStepStop
	StepInStop
	StepOutStop
	BreakpointStop
	ExceptionBreakpointStop
)

// Secondary Events

type SecondaryDebugEvent interface {
	SecondaryDebugEventType() SecondaryDebugEventType
}

type SecondaryDebugEventType int

const (
	IncomingMessageReceivedEventType = iota + 1
	LThreadSpawnedEventType
)

func (t SecondaryDebugEventType) String() string {
	switch t {
	case IncomingMessageReceivedEventType:
		return "incomingMessageReceived"
	case LThreadSpawnedEventType:
		return "routineSpawnedEvent"
	default:
		panic(ErrUnreachable)
	}
}

type IncomingMessageReceivedEvent struct {
	MessageType string `json:"messageType"` // examples: http/request, websocket/message
	Url         string `json:"url,omitempty"`
}

func (e IncomingMessageReceivedEvent) SecondaryDebugEventType() SecondaryDebugEventType {
	return IncomingMessageReceivedEventType
}

type LThreadSpawnedEvent struct {
	StateId StateId `json:"threadId,omitempty"`
}

func (e LThreadSpawnedEvent) SecondaryDebugEventType() SecondaryDebugEventType {
	return LThreadSpawnedEventType
}

// Commands

type DebugCommandSetBreakpoints struct {
	//nodes where we want to set a breakpoint, this can be set independently from .BreakPointsByLine
	BreakpointsAtNode map[parse.Node]struct{}

	//lines where we want to set a breakpoint, this can be set independently from .BreakpointsAtNode.
	//GetBreakpointsSetByLine is invoked with the resulting breakpoints, some of them can be disabled.
	BreakPointsByLine []int

	Chunk *parse.ParsedChunk

	GetBreakpointsSetByLine func(breakpoints []BreakpointInfo)
}

type DebugCommandSetExceptionBreakpoints struct {
	Disable                  bool
	GetExceptionBreakpointId func(int32)
}

type DebugCommandPause struct {
	ThreadId StateId
}

func (c DebugCommandPause) GetThreadId() StateId {
	return c.ThreadId
}

type DebugCommandContinue struct {
	ThreadId         StateId
	ResumeAllThreads bool
}

type DebugCommandNextStep struct {
	ThreadId         StateId
	ResumeAllThreads bool
}

type DebugCommandStepIn struct {
	ThreadId         StateId
	ResumeAllThreads bool
}

type DebugCommandStepOut struct {
	ThreadId         StateId
	ResumeAllThreads bool
}

type DebugCommandGetScopes struct {
	Get      func(globalScope map[string]Value, localScope map[string]Value)
	ThreadId StateId
}

type DebugCommandGetStackTrace struct {
	Get      func(trace []StackFrameInfo)
	ThreadId StateId
}

type DebugCommandCloseDebugger struct {
	CancelExecution bool
	Done            func()
}

type DebugCommandInformAboutSecondaryEvent struct {
	Event SecondaryDebugEvent
}
