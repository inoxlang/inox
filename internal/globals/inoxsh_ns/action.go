package inoxsh_ns

type termAction int

const (
	CTRL_C_CODE = 3
	ENTER_CODE  = 13

	NoAction termAction = iota
	Up
	Down
	Right
	Left
	BackwardWord
	ForwardWord
	End
	Home
	Back
	DeleteWordBackward
	DeleteWordForward
	Enter
	Stop
	SuggestComplete
	Escape
	EscapeNext
	Delete
)

func (code termAction) String() string {
	mp := map[termAction]string{
		NoAction:           "NoAction",
		Up:                 "Up",
		Down:               "Down",
		Left:               "Left",
		BackwardWord:       "BackwardWord",
		ForwardWord:        "ForwardWord",
		End:                "End",
		Home:               "Home",
		Back:               "Back",
		DeleteWordBackward: "DeleteWordBackward",
		DeleteWordForward:  "DeleteWordForward",
		Enter:              "Enter",
		Stop:               "Stop",
		SuggestComplete:    "SuggestComplete",
		Escape:             "Escape",
		EscapeNext:         "EscapeNext",
	}
	return mp[code]
}

// getTermAction reads a sequence of runes and determinates an action (enter, stop, delete word, ...).
// TODO: handle sequences from most terminal emulators. https://www.xfree86.org/current/ctlseqs.html
func getTermAction(runeSlice []rune) termAction {

	const (
		BACKSPACE_CODE      = 8
		DEL_CODE            = 127
		CTRL_BACKSPACE_CODE = 23
		TAB_CODE            = 9
		ESCAPE_CODE         = 27

		ARROW_UP_FINAL_CODE    = 65
		ARROW_DOWN_FINAL_CODE  = 66
		ARROW_RIGHT_FINAL_CODE = 67
		ARROW_LEFT_FINAL_CODE  = 68
		END_FINAL_CODE         = 70
		HOME_FINAL_CODE        = 72
		DELETE_FINAL_CODE      = 126
		CTRL_LEFT_FINAL_CODE   = 68
		CTRL_RIGHT_FINAL_CODE  = 67
	)

	if len(runeSlice) == 1 {
		switch runeSlice[0] {
		case DEL_CODE, BACKSPACE_CODE:
			return Back
		case CTRL_BACKSPACE_CODE:
			return DeleteWordBackward
		case ENTER_CODE:
			return Enter
		case CTRL_C_CODE:
			return Stop
		case TAB_CODE:
			return SuggestComplete
		case ESCAPE_CODE:
			return Escape
		}
	}

	if runeSlice[0] != ESCAPE_CODE {
		return NoAction
	}

	if runeSlice[1] == 91 {

		switch len(runeSlice) {
		case 2:
			return EscapeNext
		case 3:
			switch runeSlice[2] {
			case ARROW_UP_FINAL_CODE:
				return Up
			case ARROW_DOWN_FINAL_CODE:
				return Down
			case ARROW_RIGHT_FINAL_CODE:
				return Right
			case ARROW_LEFT_FINAL_CODE:
				return Left
			case END_FINAL_CODE:
				return End
			case HOME_FINAL_CODE:
				return Home
			case 49, 51:
				return EscapeNext
			}
		case 4:
			switch runeSlice[3] {
			case DELETE_FINAL_CODE:
				return Delete
			case 59:
				return EscapeNext
			}
		case 5:
			switch runeSlice[4] {
			case 53:
				return EscapeNext
			}
		case 6:
			switch runeSlice[5] {
			case CTRL_RIGHT_FINAL_CODE:
				return ForwardWord
			case CTRL_LEFT_FINAL_CODE:
				return BackwardWord
			}
		}
	}

	if len(runeSlice) == 2 && runeSlice[1] == 100 {
		return DeleteWordForward
	}

	return NoAction
}
