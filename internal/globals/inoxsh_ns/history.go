package inoxsh_ns

type commandHistory struct {
	Commands []string `json:"commands"`
	index    int
}

func (history commandHistory) current() string {
	return history.Commands[history.index]
}

func (history *commandHistory) scroll(n int) {
	history.index += n
	if history.index < 0 {
		history.index = len(history.Commands) - 1
	} else if history.index >= len(history.Commands) {
		history.index = 0
	}
}

func (history *commandHistory) resetIndex() {
	history.index = len(history.Commands) - 1
}
