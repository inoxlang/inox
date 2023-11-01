package inoxsh_ns

import (
	"slices"
	"strings"
)

type commandHistory struct {
	Commands                          []string `json:"commands"` //example: ["ls;", "mkdir ./a/", "ls;", "ls;"]
	commandsEmptySubsequentDuplicates []string ``                //example: ["ls;", "mkdir ./a/", "ls;", ""   ]
	index                             int
}

func (history commandHistory) currentNoDuplicate() string {
	if len(history.commandsEmptySubsequentDuplicates) == 0 {
		return ""
	}

	i := history.index
	cmd := history.commandsEmptySubsequentDuplicates[i]
	for cmd == "" && i > 0 {
		i--
		cmd = history.commandsEmptySubsequentDuplicates[i]
	}
	return cmd
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

func (history *commandHistory) addCommand(cmd string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}

	history.Commands = append(history.Commands, cmd)
	addAsDuplicateOfPrevious := false

	for i := len(history.commandsEmptySubsequentDuplicates) - 1; i >= 0; i-- {
		historyCmd := history.commandsEmptySubsequentDuplicates[i]
		if cmd == historyCmd {
			addAsDuplicateOfPrevious = true
			break
		}
		if historyCmd != "" {
			break
		}
	}

	if addAsDuplicateOfPrevious {
		history.commandsEmptySubsequentDuplicates = append(history.commandsEmptySubsequentDuplicates, "")
	} else {
		history.commandsEmptySubsequentDuplicates = append(history.commandsEmptySubsequentDuplicates, cmd)
	}

	if history.Commands[0] == "" {
		history.Commands = slices.Clone(history.Commands[1:])
	} else {
		history.scroll(+1)
	}
}

func (history *commandHistory) isLastCommandSameAsPrevious() bool {
	cmds := history.commandsEmptySubsequentDuplicates

	return len(cmds) > 1 && cmds[len(cmds)-1] == ""
}
