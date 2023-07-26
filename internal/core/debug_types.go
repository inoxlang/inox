package core

import (
	parse "github.com/inoxlang/inox/internal/parse"
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
	Id          int32 //set if debugging, unique for a given debugger
	StartLine   int32
	StartColumn int32

	StatementStartLine   int32
	StatementStartColumn int32
}

// Primary Events

type ProgramStoppedEvent struct {
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
)

func (t SecondaryDebugEventType) String() string {
	switch t {
	case IncomingMessageReceivedEventType:
		return "incomingMessageReceived"
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
}

type DebugCommandContinue struct {
}

type DebugCommandNextStep struct {
}

type DebugCommandStepIn struct {
}

type DebugCommandStepOut struct {
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

type DebugCommandInformAboutSecondaryEvent struct {
	Event SecondaryDebugEvent
}
