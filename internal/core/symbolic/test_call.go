package symbolic

import "github.com/inoxlang/inox/internal/parse"

// RecTestCallState represents the state of a recursive Value.Test or Pattern.TestValue call.
// It is NOT thread safe.
type RecTestCallState struct {
	depth     int64
	evalState *State

	currentTesterName string
	partNameLen       int

	//Location buffer that we only append to. See the methods ForProperty and ForIndex.
	//The length may be higher for child states. This is fine because a parent state
	//never accesses the buffer while a child state is active (same goroutine call).
	locationBuf []byte
}

func (s *RecTestCallState) StartCall() {
	s.depth++
	s.check()
	if s.locationBuf == nil && s.evalState != nil {
		s.locationBuf = s.evalState.testCallLocationBuffer
	}
}

func (s *RecTestCallState) StartCallWithName(name string) {
	s.depth++
	s.check()
	s.currentTesterName = name
}

func (s *RecTestCallState) FinishCall() {
	s.depth--
}

// ForProperty returns a new RecTestCallState with `.` $name or `.(` $name `)` added to the location buffer.
func (s RecTestCallState) ForProperty(name string) RecTestCallState {
	new := s

	if s.evalState != nil {
		if parse.IsValidIdent(name) {
			new.partNameLen = 1 /* '.' */ + len(name)

			new.locationBuf = append(new.locationBuf, '.')
			new.locationBuf = append(new.locationBuf, name...)

		} else {
			new.partNameLen = 3 /* '.(' and ')' */ + len(name)

			new.locationBuf = append(new.locationBuf, '.', '(')
			new.locationBuf = append(new.locationBuf, name...)

			new.locationBuf = append(new.locationBuf, ')')
		}

	}
	return new
}

// ForProperty returns a new RecTestCallState with '[' <$name> ']' added to the location buffer.
func (s RecTestCallState) ForIndex(index string) RecTestCallState {
	new := s

	if s.evalState != nil {
		new.partNameLen = 2 /* '[' and ']' */ + len(index)

		new.locationBuf = append(new.locationBuf, '[')
		new.locationBuf = append(new.locationBuf, index...)
		new.locationBuf = append(new.locationBuf, ']')
	}
	return new
}

func (s RecTestCallState) WriteMismatch(messageParts ...string) {
	if s.evalState == nil || s.evalState.testCallMessageBuffer.Len() != 0 {
		return
	}
	msgBuf := s.evalState.testCallMessageBuffer

	if len(s.locationBuf) > 0 {
		msgBuf.WriteString(s.currentTesterName) //not an issue if empty

		msgBuf.WriteString(" at `")
		msgBuf.Write(s.locationBuf)
		msgBuf.WriteString("` : ")
	}

	for _, part := range messageParts {
		msgBuf.WriteString(part)
	}
}

func (s RecTestCallState) check() {
	if s.depth > MAX_RECURSIVE_TEST_CALL_DEPTH {
		panic(ErrMaximumSymbolicTestCallDepthReached)
	}
}
