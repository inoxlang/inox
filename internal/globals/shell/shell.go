package internal

import (
	//STANDARD LIBRARY

	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf8"

	core "github.com/inox-project/inox/internal/core"

	symbolic "github.com/inox-project/inox/internal/core/symbolic"
	symbolic_shell "github.com/inox-project/inox/internal/globals/shell/symbolic"

	sug "github.com/inox-project/inox/internal/globals/suggestion"
	"github.com/inox-project/inox/internal/utils"

	parse "github.com/inox-project/inox/internal/parse"

	//EXTERNAL

	"github.com/muesli/cancelreader"
	"golang.org/x/term"
)

const (
	DEFAULT_TERM_WIDTH      = 10
	DEFAULT_READ_CHUNK_SIZE = 1000
)

var (
	KEY_PRIORITY = map[string]int{
		"id":    -1000,
		"name":  -999,
		"title": -998,
	}

	ALLOWED_PROMPT_FUNCTION_NAMES = []string{"pwd", "whoami", "hostname"}
	SHELL_PROPNAMES               = []string{"write", "read", "start", "stop"}

	_ = []any{
		core.PotentiallySharable((*shell)(nil)), core.Readable((*shell)(nil)), core.Writable((*shell)(nil)),
		core.StreamSource((*shell)(nil)), core.StreamSink((*shell)(nil)),
	}
)

func init() {
	core.RegisterSymbolicGoFunction(_cd, func(ctx *symbolic.Context, path *symbolic.Path) *symbolic.Error {
		return nil
	})
	core.RegisterSymbolicGoFunction(_pwd, func(ctx *symbolic.Context) *symbolic.Path {
		return &symbolic.Path{}
	})
	core.RegisterSymbolicGoFunction(_whoami, func(ctx *symbolic.Context) *symbolic.String {
		return &symbolic.String{}
	})
	core.RegisterSymbolicGoFunction(_hostname, func(ctx *symbolic.Context) *symbolic.String {
		return &symbolic.String{}
	})
}

// a shell represents an instance of an Inox REPL, depending on its configuration it can behave like a real shell.
type shell struct {
	core.NoReprMixin
	core.NotClonableMixin

	config  REPLConfiguration
	state   *core.TreeWalkState
	started atomic.Bool
	shared  atomic.Bool

	ioLock sync.Mutex // prevents more than one goroutine to write the input|read the output
	in     io.ReadWriter
	inFd   int // >= 0 if .in is an *os.File
	out    io.ReadWriter
	outFd  int // >= 0 if .out is an *os.File
	//errOut     io.ReadWriter
	coreReader *core.Reader
	coreWriter *core.Writer

	stopping atomic.Bool

	//all the fields below are read & written by the main shell loop

	//terminal
	termWidth int

	//current input info
	cancelReader    cancelreader.CancelReader
	reader          *bufio.Reader
	input           []rune
	runeSequence    []rune
	backspaceCount  int //distance of the cursor from the end of the input
	pressedTabCount int //used for completions
	ignoreNextChar  bool
	promptLen       int

	runeInputChan      chan runeInput
	stopReadingInput   chan struct{}
	resumeReadingInput chan struct{}

	inputsToCheck     chan string
	stopCheckingInput chan struct{}
	inputCheckErrors  chan error

	//previous input info
	prevInputLineCount      int
	prevSuggestionLineCount int
	prevSuggestionCount     int
	prevRowIndex            int

	//
	foregroundTask *fgTask
	history        commandHistory
}

// starts the shell, the execution of this function ends when the shell is exited.
func StartShell(state *core.GlobalState, config REPLConfiguration) {
	shell := newShell(config, state, os.Stdin, os.Stdout /*os.Stderr*/)
	shell.runLoop()
}

func newShell(config REPLConfiguration, state *core.GlobalState, in io.ReadWriter, out io.ReadWriter /*err io.ReadWriter*/) *shell {
	sh := &shell{
		config: config,
		state:  core.NewTreeWalkStateWithGlobal(state),
		in:     in,
		inFd:   -1,
		out:    out,
		outFd:  -1,
		//errOut: err,

		//input

		input:           make([]rune, 0),
		backspaceCount:  0,
		pressedTabCount: 0,
		ignoreNextChar:  false,

		runeInputChan:      make(chan (runeInput), 4096),
		stopReadingInput:   make(chan struct{}, 1),
		resumeReadingInput: make(chan struct{}, 1),
		stopCheckingInput:  make(chan struct{}, 1),
		inputsToCheck:      make(chan string, 10),
		inputCheckErrors:   make(chan error, 10),

		//previous input

		prevInputLineCount:      1,
		prevSuggestionLineCount: 0,
		prevSuggestionCount:     0,
		prevRowIndex:            -1,

		//
		history: commandHistory{Commands: []string{""}, index: 0},
	}

	if inFile, ok := in.(*os.File); ok {
		sh.inFd = int(inFile.Fd())
	}
	if outFile, ok := out.(*os.File); ok {
		sh.outFd = int(outFile.Fd())
	}
	return sh
}

type fgTask struct {
	done    chan (bool)
	result  core.Value
	evalErr error
}

func newTask() *fgTask {
	return &fgTask{done: make(chan bool)}
}

type runeInput struct {
	r   rune
	err error
}

func (sh *shell) isInputFile() bool {
	return sh.inFd >= 0
}

func (sh *shell) isOutputFile() bool {
	return sh.outFd >= 0
}

func (sh *shell) createReader() {
	sh.cancelReader, _ = cancelreader.NewReader(sh.in)
	sh.reader = bufio.NewReader(sh.cancelReader)
}

// getNewLineCount returns the new number of lines.
func (sh *shell) getNewLineCount() int {
	return 1 + (len(sh.input)+sh.promptLen)/sh.termWidth
}

// resetInput reset fields holding data about the input text.
func (sh *shell) resetInput() {
	sh.input = nil
	sh.backspaceCount = 0
	sh.runeSequence = nil
	sh.pressedTabCount = 0
	sh.ignoreNextChar = false

	sh.prevInputLineCount = sh.getNewLineCount()
	sh.prevRowIndex = sh.prevInputLineCount - 1
}

func (sh *shell) moveCursorLineStart() {
	moveCursorBack(sh.out, len(sh.input)+sh.promptLen)
}

func (sh *shell) getCursorIndex() int {
	return len(sh.input) - sh.backspaceCount
}

// moves the cursor at the start of the prompt, prints the prompt and the input with colorizations and then moves the cursor at the right place
func (sh *shell) printPromptAndInput(inputGotReplaced bool, suggestions []string) {
	//we use a buffer to output most of the prompt+input in a single print, that reduces flickering on some terminals.
	buff := bytes.NewBuffer(nil)
	clearLine(buff)

	chunk, _ := parse.ParseChunk(string(sh.input), "")

	//terminal resizing is not supported yet
	lineCount := sh.getNewLineCount()

	rowIndex := (sh.getCursorIndex() + sh.promptLen) / sh.termWidth
	columnIndex := (sh.getCursorIndex() + sh.promptLen) % sh.termWidth

	if lineCount > sh.prevInputLineCount {
		if !inputGotReplaced {
			fmt.Fprintf(buff, "\n\r")
		}
	} else if lineCount == sh.prevInputLineCount && sh.prevInputLineCount > 1 && sh.prevRowIndex != 0 {
		moveCursorUp(buff, sh.prevRowIndex)
	}

	moveCursorBack(buff, sh.termWidth)

	//--------------------- actualy prints -----------------------

	var prompt string
	prompt, sh.promptLen = sprintPrompt(sh.state, sh.config)
	buff.WriteString(prompt)

	core.PrintColorizedChunk(buff, chunk, sh.input, sh.config.IsLight(), sh.config.defaultFgColorSequence)

	fmt.Fprint(sh.out, buff.String())

	//print suggestions

	suggestionString := strings.Join(suggestions, " ")
	suggestionLineCount := 1 + strings.Count(suggestionString, "\n") + utf8.RuneCountInString(suggestionString)/sh.termWidth

	if len(suggestions) != 0 || sh.prevSuggestionCount != 0 {
		sh.moveCursorLineStart()

		fmt.Fprintf(sh.out, "\n\r%s", suggestionString)

		if len(suggestions) == 0 {
			clearLine(sh.out)
		}

		//if the new suggestions are shorter than the previous ones we clear the additional lines of the previous suggestions
		if sh.prevSuggestionLineCount > suggestionLineCount {
			moveCursorDown(sh.out, sh.prevSuggestionLineCount-suggestionLineCount)
			clearLines(sh.out, sh.prevSuggestionLineCount-suggestionLineCount)
		}

		moveCursorUp(sh.out, suggestionLineCount)
		sh.prevSuggestionLineCount = suggestionLineCount
		sh.prevSuggestionCount = len(suggestions)
	}

	//move cursor
	if sh.prevInputLineCount > 1 {
		upCount := int(utils.Abs(sh.prevInputLineCount - 1 - rowIndex))

		moveCursorUp(sh.out, upCount)
	}

	moveCursorBack(sh.out, sh.termWidth)
	moveCursorForward(sh.out, columnIndex)

	sh.prevInputLineCount = lineCount
	sh.prevRowIndex = rowIndex
}

// applyConfiguration adds some global variables based on the configuration.
func (sh *shell) applyConfiguration(prevTermState *term.State) {

	config := sh.config
	for k, v := range config.additionalGlobals {
		if !sh.state.SetGlobal(k, v, core.GlobalVar) {
			panic(errors.New("configuration.globals cannot redefine global constants"))
		}
	}

	builtinCommands := map[string]core.Value{
		"cd":       core.ValOf(_cd),
		"pwd":      core.ValOf(_pwd),
		"whoami":   core.ValOf(_whoami),
		"hostname": core.ValOf(_hostname),
	}

	for name, builtinCommand := range builtinCommands {
		if utils.SliceContains(config.builtinCommands, name) {
			if !sh.state.SetGlobal(name, builtinCommand, core.GlobalConst) {
				panic(fmt.Errorf("failed to set global '%s'", name))
			}
		}
	}

	//add trusted commands to the global scope

	for _, cmd := range config.trustedCommands {
		if sh.state.HasGlobal(cmd) {
			panic(errors.New("trusted commands cannot override a global variable"))
		}

		if !sh.isInputFile() || !sh.isOutputFile() {
			panic(errors.New("cannot setup trusted commands if input & output are not both files"))
		}

		executeCmdFn := func(cmdName string) func(ctx *core.Context, args ...core.Value) (core.Value, error) {
			return func(ctx *core.Context, args ...core.Value) (core.Value, error) {
				defer func() {
					sh.resumeReadingInput <- struct{}{}
					if sh.isInputFile() {
						term.MakeRaw(sh.inFd)
					}
				}()
				var subcommandNameChain []string
				var cmdArgs []string

				//we remove the subcommand chain from <args>
				for len(args) != 0 {
					name, ok := args[0].(core.Identifier)
					if ok {
						subcommandNameChain = append(subcommandNameChain, string(name))
						args = args[1:]
					} else {
						break
					}
				}

				//we check that remaining args are simple values or options
				for _, arg := range args {
					if core.IsSimpleInoxValOrOption(arg) {
						if r, ok := arg.(core.Rune); ok {
							arg = core.Str(r)
						}
						cmdArgs = append(cmdArgs, fmt.Sprint(arg))
					} else {
						return core.Str(""), fmt.Errorf("ex: invalid argument %v of type %T, only simple values are allowed", arg, arg)
					}
				}

				//some subcommand identifiers could be "arguments" and not subcommand names, so we check the permissions
				//by removing subcommands from the end until it's okay
				var permErr error
				for i := len(subcommandNameChain); i >= 0; i-- {
					perm := core.CommandPermission{
						CommandName:         core.Str(cmdName),
						SubcommandNameChain: subcommandNameChain[:i],
					}

					permErr = ctx.CheckHasPermission(perm)
					if permErr == nil {
						break
					}
				}

				if permErr != nil {
					return core.Str(""), permErr
				}

				passedArgs := make([]string, 0)
				passedArgs = append(passedArgs, subcommandNameChain...)
				passedArgs = append(passedArgs, cmdArgs...)

				cmd := exec.Command(fmt.Sprint(cmdName), passedArgs...)

				cmd.Stdin = sh.in
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr

				sh.cancelReader.Cancel()
				term.Restore(sh.inFd, prevTermState)

				//execution

				err := cmd.Start()
				if err != nil {
					return nil, err
				}

				err = cmd.Wait()

				if _, ok := err.(*exec.ExitError); ok {
					status := cmd.ProcessState.Sys().(syscall.WaitStatus)
					if status.Signal().String() == "interrupt" {
						return nil, nil
					}
					return nil, err
				} else if err != nil {
					return nil, err
				}

				return nil, nil
			}
		}(cmd)

		if !core.IsSymbolicEquivalentOfGoFunctionRegistered(executeCmdFn) {
			core.RegisterSymbolicGoFunction(executeCmdFn,
				func(ctx *symbolic.Context, args ...symbolic.SymbolicValue) (symbolic.SymbolicValue, *symbolic.Error) {
					return &symbolic.Any{}, nil
				},
			)
		}

		executeCmd := core.ValOf(executeCmdFn)

		sh.state.SetGlobal(cmd, executeCmd, core.GlobalConst)
	}
}

// runLoop starts the shell loop in the current goroutine.
func (sh *shell) runLoop() {
	if !sh.started.CompareAndSwap(false, true) {
		return
	}

	var prevTermState *term.State

	if sh.isInputFile() {
		var err error
		prevTermState, err = term.MakeRaw(sh.inFd)
		if err != nil {
			panic(err)
		}
		defer func() {
			term.Restore(sh.inFd, prevTermState)
		}()
	}

	sh.applyConfiguration(prevTermState)

	if sh.config.handleSignals {
		signal.Reset()
		signalChan := make(chan os.Signal, 1)
		signal.Ignore(syscall.SIGTTOU, syscall.SIGTTIN, syscall.SIGTSTP)
		signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGWINCH, syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGQUIT)

		go func() {
			for s := range signalChan {
				switch s {
				case os.Interrupt:
					continue
				case syscall.SIGTERM:
					term.Restore(sh.inFd, prevTermState)
					os.Exit(0)
				case syscall.SIGWINCH:
					continue
				}
			}
		}()
	}

	//
	sh.createReader()
	if sh.isOutputFile() {
		sh.termWidth, _, _ = term.GetSize(sh.outFd)
	} else {
		sh.termWidth = DEFAULT_TERM_WIDTH
	}
	sh.promptLen = printPrompt(sh.out, sh.state, sh.config)

	defer func() {
		sh.stopReadingInput <- struct{}{}
	}()

	//we add a local scope in order to persist local variables across executions
	sh.state.PushScope()
	defer sh.state.PopScope()

	//This routine reads the input without interruption.
	//While there is a child process the read bytes are written to the child process's input
	go func() {
		for {
			select {
			case <-sh.stopReadingInput:
				close(sh.stopReadingInput)
				close(sh.runeInputChan)
				close(sh.resumeReadingInput)
				return
			default:
				r, n, err := sh.reader.ReadRune()

				if errors.Is(err, cancelreader.ErrCanceled) {
					<-sh.resumeReadingInput
					sh.cancelReader, _ = cancelreader.NewReader(sh.in)
					sh.reader = bufio.NewReader(sh.cancelReader)
					continue
				}
				if n == 0 { // nothing read
					time.Sleep(time.Millisecond / 2)
					continue
				}
				sh.runeInputChan <- runeInput{r, err}
			}
		}
	}()

	defer func() {
		sh.stopCheckingInput <- struct{}{}
	}()

	//This routine checks the read input.
	go func() {
		var lastInput string
		for {
			var input string
			select {
			case <-sh.stopCheckingInput:
				return
			case input = <-sh.inputsToCheck:
			}

			if input == lastInput {
				continue
			}

			dbg("check", input)
			lastInput = input
			// mod, err := sh.parseModule(input)

			// if err != nil {
			// 	continue
			// }

			// symbolicData, err := sh.checkModule(mod)
		}
	}()

	lastRuneTime := time.Now()

	//the shell loop gets one rune at a time from the reading goroutine and handles it.
shell_loop:
	for {
		// before reading the next rune we check the current foreground task.
		if sh.foregroundTask != nil {
			select {
			case <-sh.foregroundTask.done:
				sh.handleFgTask()
			case <-sh.state.Global.Ctx.Done():
				return
			default:
			}
		}

		if sh.stopping.CompareAndSwap(true, false) {
			return
		}

		var r rune
		var err error

		select {
		default:
			// TODO: fix data race
			if time.Since(lastRuneTime) > time.Second/2 && len(sh.runeInputChan)+sh.reader.Buffered() == 0 {
				sh.inputsToCheck <- string(sh.input)
			}
			time.Sleep(time.Millisecond / 2)
			continue
		case runeInput := <-sh.runeInputChan:
			r = runeInput.r
			err = runeInput.err
			lastRuneTime = time.Now()

			if err == io.EOF {
				sh.out.Write([]byte("EOF"))
			} else if err != nil {
				sh.out.Write([]byte(err.Error()))
				panic(err)
			}

			if sh.ignoreNextChar {
				sh.ignoreNextChar = false
				continue
			}

			sh.runeSequence = append(sh.runeSequence, r)
			action := getTermAction(sh.runeSequence)

			if sh.foregroundTask != nil {
				// we stop the foreground task and continue the shell loop
				if action == Stop {
					sh.state.Global.Ctx.Cancel()

					clone := sh.state.Global.Ctx.New()
					clone.SetClosestState(sh.state.Global)
					sh.state.Global.Ctx = clone

					fmt.Fprint(sh.out, "\n\r")
				}
				sh.runeSequence = nil
				continue
			}

			switch action {
			case Escape, EscapeNext: //while we the escape sequence is not complete we just continue reading
				continue
			default:
				sh.runeSequence = nil
			}

			switch {
			case action == NoAction && strconv.IsPrint(r):
				sh.addRuneToInput(r)
				sh.printPromptAndInput(false, nil)
			case action != NoAction:
				if stop := sh.handleAction(action); stop {
					break shell_loop
				}
			}
		}
	}
}

// addRuneToInput adds the rune at the right position in the input string based on the cursor's position.
// If the rune is an opening delimiter, the corresponding closing delimiter is also added.
func (sh *shell) addRuneToInput(r rune) {
	var left []rune
	var right []rune

	//if the rune is not special we add it to the input
	if len(sh.input) != 0 {
		cursorIndex := sh.getCursorIndex()

		_left := sh.input[0:cursorIndex]
		left = make([]rune, len(_left))
		copy(left, _left)

		_right := sh.input[cursorIndex:]
		right = make([]rune, len(_right))
		copy(right, _right)
	}

	sh.input = append(left, r)

	//we append the corresponding closing delimiter if the new rune is an opening delimiter and the input buffer is empty
	if len(sh.runeInputChan)+sh.reader.Buffered() == 0 {
		switch r {
		case '[', '{', '(':
			sh.input = append(sh.input, getClosingDelimiter(r))
			sh.backspaceCount++
		}
	}

	sh.input = append(sh.input, right...)
}

func (sh *shell) handleAction(action termAction) (stop bool) {

	moveHome := func() {
		prevBackspaceCount := sh.backspaceCount
		sh.backspaceCount = len(sh.input)
		if sh.backspaceCount == prevBackspaceCount {
			return
		}
		sh.printPromptAndInput(false, nil)
	}

	moveEnd := func() {
		if sh.backspaceCount == 0 {
			return
		}
		sh.backspaceCount = 0
		sh.printPromptAndInput(false, nil)
	}

	switch action {
	case Up:
		fallthrough
	case Down:
		prevCount := sh.prevInputLineCount
		clearLine(sh.out)

		sh.resetInput()
		diff := utils.Abs(prevCount - sh.prevInputLineCount)
		if diff != 0 {
			moveCursorUp(sh.out, diff)
		}

		sh.input = []rune(sh.history.current())

		if action == Up {
			sh.history.scroll(-1)
		} else {
			sh.history.scroll(+1)
		}

		sh.printPromptAndInput(true, nil)
		return
	case Escape:
	case Delete:
		if len(sh.input) == 0 || sh.backspaceCount == 0 {
			return
		}

		start := len(sh.input) - sh.backspaceCount
		right := utils.CopySlice(sh.input[start+1:])
		sh.input = append(sh.input[0:start], right...)

		saveCursorPosition(sh.out)

		fmt.Fprint(sh.out, string(right))
		clearLineRight(sh.out)
		restoreCursorPosition(sh.out)

		sh.backspaceCount -= 1
	case Back:

		if len(sh.input) == 0 || sh.backspaceCount >= len(sh.input) {
			return
		}

		start := len(sh.input) - sh.backspaceCount - 1
		right := utils.CopySlice(sh.input[start+1:])
		sh.input = append(sh.input[0:start], right...)

		moveCursorBack(sh.out, 1)
		saveCursorPosition(sh.out)

		sh.printPromptAndInput(false, nil)
		restoreCursorPosition(sh.out)
	case Home:
		moveHome()
	case End:
		moveEnd()
	case Left:
		if sh.backspaceCount < len(sh.input) {
			sh.backspaceCount += 1
			sh.printPromptAndInput(false, nil)
		}
	case Right:
		if sh.backspaceCount > 0 {
			sh.backspaceCount -= 1
			sh.printPromptAndInput(false, nil)
		}
	case DeleteWordBackward:

		if len(sh.input) == 0 || sh.backspaceCount >= len(sh.input) {
			return
		}

		chunk, _ := parse.ParseChunk(string(sh.input), "")
		tokens := parse.GetTokens(chunk, false)

		switch len(tokens) {
		case 0:
			return
		}

		//search for the last token starting before the cursor, the token can end after the cursor
		cursorIndex := sh.getCursorIndex()
		tokenIndex := 0

		for i, token := range tokens {
			if cursorIndex <= int(token.Span.Start) {
				break
			} else {
				tokenIndex = i
			}
		}

		lastToken := tokens[tokenIndex]

		//remove the part of the token that is before the cursor
		start := lastToken.Span.Start
		right := utils.CopySlice(sh.input[cursorIndex:])
		sh.input = append(sh.input[0:start], right...)

		sh.printPromptAndInput(false, nil)
	case DeleteWordForward:
		if len(sh.input) == 0 || sh.backspaceCount == 0 {
			return
		}

		chunk, _ := parse.ParseChunk(string(sh.input), "")
		tokens := parse.GetTokens(chunk, false)

		switch len(tokens) {
		case 0:
			return
		}

		//search for the first token ending after the cursor, the token can start before the cursor
		cursorIndex := sh.getCursorIndex()
		tokenIndex := 0

		for i, token := range tokens {
			if cursorIndex > int(token.Span.End) {
				if i < len(tokens)-1 {
					tokenIndex = i + 1
				}
				break
			}
		}

		lastToken := tokens[tokenIndex]

		//remove the part of the token that is after the cursor
		right := utils.CopySlice(sh.input[lastToken.Span.End:])
		sh.input = append(sh.input[0:cursorIndex], right...)
		sh.backspaceCount -= int(lastToken.Span.End - lastToken.Span.Start)

		sh.printPromptAndInput(false, nil)
	case BackwardWord:
		chunk, _ := parse.ParseChunk(string(sh.input), "")
		tokens := parse.GetTokens(chunk, false)

		switch len(tokens) {
		case 0:
			return
		case 1:
			//TODO: fix
			moveHome()
			return
		}

		//search for the last token starting before the cursor, the token can end after the cursor
		cursorIndex := sh.getCursorIndex()
		tokenIndex := 0
		var newCursorIndex int

		for i, token := range tokens {
			if cursorIndex < int(token.Span.Start) {
				break
			} else {
				tokenIndex = i
			}
		}

		if tokenIndex == 0 {
			moveHome()
			return
		}

		lastToken := tokens[tokenIndex]

		if cursorIndex >= int(lastToken.Span.End) {
			newCursorIndex = int(lastToken.Span.Start)
		} else if cursorIndex == int(lastToken.Span.Start) {
			newCursorIndex = int(tokens[tokenIndex-1].Span.Start)
		} else {
			newCursorIndex = int(lastToken.Span.Start)
		}

		backward := cursorIndex - newCursorIndex
		sh.backspaceCount += backward

		sh.printPromptAndInput(false, nil)
	case ForwardWord:
		chunk, _ := parse.ParseChunk(string(sh.input), "")

		tokens := parse.GetTokens(chunk, false)

		switch len(tokens) {
		case 0:
			return
		case 1:
			moveEnd()
			return
		}

		//search for the first token ending after the cursor, the token can start before the cursor
		cursorIndex := sh.getCursorIndex()
		tokenIndex := len(tokens) - 1
		var newCursorIndex int

		for i, token := range tokens {
			if cursorIndex < int(token.Span.Start) {
				break
			} else {
				tokenIndex = i
			}
		}

		lastToken := tokens[tokenIndex]

		if cursorIndex >= int(lastToken.Span.End) {
			if tokenIndex < len(tokens)-1 {
				newCursorIndex = int(tokens[tokenIndex+1].Span.End)
			} else {
				newCursorIndex = int(lastToken.Span.End)
			}
		} else {
			newCursorIndex = int(lastToken.Span.End)
		}

		forward := newCursorIndex - cursorIndex

		sh.backspaceCount -= forward
		sh.printPromptAndInput(false, nil)
		return
	case Stop:
		sh.resetInput()
		sh.printPromptAndInput(true, nil)
		return
	case SuggestComplete:
		sh.pressedTabCount++

		if sh.pressedTabCount == 1 {
			return
		} else {
			sh.pressedTabCount = 0
		}

		//if the input is empty we print all global variable names.
		if strings.Trim(string(sh.input), " ") == "" {
			var names []string

			sh.state.Global.Globals.Foreach(func(name string, v core.Value) {
				names = append(names, name)
			})

			sort.Strings(names)
			sh.resetInput()
			sh.printPromptAndInput(true, names)

			break
		}

		var (
			chunk, _    = parse.ParseChunk(string(sh.input), "")
			cursorIndex = sh.getCursorIndex()
			suggestions = sug.FindSuggestions(sh.state, chunk, cursorIndex)

			replacement       string
			replacedSpan      parse.NodeSpan
			suggestionStrings []string
			newCharCount      int
		)

		switch len(suggestions) {
		case 0:
			//do nothing
			return
		case 1:
			//do a replacement and do not print suggestions
			replacement = suggestions[0].Value
			replacedSpan = suggestions[0].Span
		default:
			var suggestionValues []string //used to find longest common prefix
			var span = suggestions[0].Span
			addPrefix := true

			for _, sug := range suggestions {
				suggestionValues = append(suggestionValues, sug.Value)
				suggestionStrings = append(suggestionStrings, sug.ShownString)
				if sug.Span != span {
					addPrefix = false
				}
			}

			sort.Strings(suggestionStrings)

			if addPrefix {
				prefix := utils.FindLongestCommonPrefix(suggestionValues)
				if prefix != "" {
					replacement = prefix
					replacedSpan = span
				}
			}
		}

		//replace the incomplete element with replacement
		if replacement != "" {
			beforeElem := sh.input[:replacedSpan.Start]
			afterElem := utils.CopySlice(sh.input[utils.Min(len(sh.input), int(replacedSpan.End)):])

			prevLen := len(sh.input)
			sh.input = append(beforeElem, []rune(replacement)...)
			sh.input = append(sh.input, afterElem...)
			newCharCount = len(sh.input) - prevLen
			saveCursorPosition(sh.out)
		}

		sh.printPromptAndInput(false, suggestionStrings)

		if replacement != "" {
			restoreCursorPosition(sh.out)
			moveCursorForward(sh.out, newCharCount)
		}

	case Enter:

		if sh.foregroundTask != nil {
			return
		}

		sh.history.resetIndex()
		sh.out.Write(core.ANSI_RESET_SEQUENCE)

		//if input is empty we do nothing and print the prompt on a new line
		if strings.Trim(string(sh.input), " ") == "" {
			fmt.Fprint(sh.out, "\n\r")
			sh.promptLen = printPrompt(sh.out, sh.state, sh.config)
			break
		}

		//we add the input to the history
		sh.history.Commands = append(sh.history.Commands, string(sh.input))
		if sh.history.Commands[0] == "" {
			sh.history.Commands = sh.history.Commands[1:]
		} else {
			sh.history.scroll(+1)
		}

		inputString := string(sh.input)
		splitted := strings.Split(inputString, " ")

		switch splitted[0] {
		case "clear":
			sh.resetInput()
			clearScreen(sh.out)
			sh.promptLen = printPrompt(sh.out, sh.state, sh.config)
		case "quit":
			fmt.Fprint(sh.out, "\n\r")
			return true
		default:
			//handle normal commands
			sh.resetInput()
			fmt.Fprint(sh.out, "\n\r")
			clearLine(sh.out)
			moveCursorNextLine(sh.out, 1)

			mod, err := sh.parseModule(inputString)
			var symbolicData *symbolic.SymbolicData
			var staticCheckData *core.StaticCheckData

			if err == nil {
				staticCheckData, symbolicData, err = sh.checkModule(mod)
			}

			if err != nil {
				//print parsing or checking error and print a new prompt
				errString := utils.AddCarriageReturnAfterNewlines(err.Error())
				fmt.Fprint(sh.out, errString, "\n\r")
				moveCursorNextLine(sh.out, 1)
				sh.promptLen = printPrompt(sh.out, sh.state, sh.config)
			} else {
				//TODO: delete useless data in order to reduce memory usage
				sh.state.Global.SymbolicData.AddData(symbolicData)
				sh.state.Global.Module = mod
				sh.state.Global.StaticCheckData = staticCheckData

				localScope := sh.state.LocalScopeStack[0]
				sh.state = core.NewTreeWalkStateWithGlobal(sh.state.Global)
				sh.state.LocalScopeStack = append(sh.state.LocalScopeStack, localScope)

				//start evaluation in a goroutine
				sh.foregroundTask = newTask()

				go func(foregroundTask *fgTask) {
					defer func() {
						foregroundTask.done <- true
					}()

					res, evalErr := core.TreeWalkEval(mod.MainChunk.Node, sh.state)
					foregroundTask.result = res
					foregroundTask.evalErr = evalErr
				}(sh.foregroundTask)
				moveCursorNextLine(sh.out, 1)
			}
		}
	}

	return
}

func (sh *shell) parseModule(inputString string) (*core.Module, error) {

	chunk, err := parse.ParseChunkSource(parse.InMemorySource{
		NameString: "shell-input",
		CodeString: inputString,
	})

	if chunk.Node != nil {
		chunk.Node.IsShellChunk = true
	}

	return &core.Module{
		MainChunk: chunk,
	}, err
}

func (sh *shell) checkModule(mod *core.Module) (*core.StaticCheckData, *symbolic.SymbolicData, error) {
	staticCheckData, checkErr := core.StaticCheck(core.StaticCheckInput{
		Node:              mod.MainChunk.Node,
		Chunk:             mod.MainChunk,
		Module:            mod,
		GlobalConsts:      sh.state.Global.Globals,
		ShellLocalVars:    sh.state.CurrentLocalScope(),
		Patterns:          sh.state.Global.Ctx.GetNamedPatterns(),
		PatternNamespaces: sh.state.Global.Ctx.GetPatternNamespaces(),
	})

	if checkErr != nil {
		return nil, nil, checkErr
	}

	symbolicContext, err := sh.state.Global.Ctx.ToSymbolicValue()

	if err != nil {
		return nil, nil, err
	}

	globalConsts := map[string]any{}
	shellLocalVars := map[string]any{}

	sh.state.Global.Globals.Foreach(func(k string, v core.Value) {
		globalConsts[k] = v
	})

	for k, v := range sh.state.CurrentLocalScope() {
		shellLocalVars[k] = v
	}

	symbData, err := symbolic.SymbolicEvalCheck(symbolic.SymbolicEvalCheckInput{
		Node: mod.MainChunk.Node,
		Module: &symbolic.Module{
			MainChunk: mod.MainChunk,
		},
		GlobalConsts: globalConsts,

		IsShellChunk:   true,
		ShellLocalVars: shellLocalVars,
		Context:        symbolicContext,
	})

	if err != nil {
		return nil, nil, err
	}

	return staticCheckData, symbData, nil
}

func (sh *shell) printFgTaskError(ctx *core.Context) {
	err := sh.foregroundTask.evalErr

	var assertionErr *core.AssertionError
	var errString string
	if errors.As(err, &assertionErr) {
		errString = assertionErr.PrettySPrint(sh.config.PrettyPrintConfig().WithContext(ctx))
	} else {
		errString = utils.StripANSISequences(err.Error())
	}
	errString = utils.AddCarriageReturnAfterNewlines(errString)

	fmt.Fprint(sh.out, errString, "\n\r")
}

func (sh *shell) handleFgTask() {
	//print result or error
	if sh.foregroundTask.evalErr != nil {
		// restore context
		ctxClone := sh.state.Global.Ctx.New()
		ctxClone.SetClosestState(sh.state.Global)
		sh.state.Global.Ctx = ctxClone

		sh.printFgTaskError(ctxClone)
	} else {
		sh.printFgTaskResult()
	}

	moveCursorNextLine(sh.out, 1)
	sh.promptLen = printPrompt(sh.out, sh.state, sh.config)

	sh.foregroundTask = nil
}

func (sh *shell) printFgTaskResult() {

	result := sh.foregroundTask.result

	const VALUE_FMT = "%#v\n\r"
	var s string

	prettyPrintConfig := sh.config.PrettyPrintConfig().WithContext(sh.state.Global.Ctx) // ctx could be cancelled

	switch r := result.(type) {
	default:
		r.PrettyPrint(sh.out, prettyPrintConfig, 0, 0)
		sh.out.Write([]byte{'\n'})
	case nil, core.NilT:
		return
	case core.Str:
		s = utils.StripANSISequences(string(r)) + "\n"
		fmt.Fprint(sh.out, utils.AddCarriageReturnAfterNewlines(s))
	case *core.List:
		if r.Len() == 0 {
			s = "[]\n"
			fmt.Fprint(sh.out, utils.AddCarriageReturnAfterNewlines(s))
			return
		} else {
			r.PrettyPrint(sh.out, prettyPrintConfig, 0, 0)
			sh.out.Write([]byte{'\n'})
		}

	}
}

// implementation of core.Value for shell

func (sh *shell) IsSharable(originState *core.GlobalState) (bool, string) {
	return true, ""
}

func (sh *shell) Share(originState *core.GlobalState) {
	if !sh.shared.CompareAndSwap(false, true) {
		return
	}
}

func (sh *shell) IsShared() bool {
	return sh.shared.Load()
}

func (sh *shell) ForceLock() {
	//
}

func (sh *shell) ForceUnlock() {
	//
}

func (sh *shell) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherSh, ok := other.(*shell)
	return ok && otherSh == sh
}

func (*shell) IsMutable() bool {
	return true
}

func (sh *shell) Reader() *core.Reader {
	sh.ioLock.Lock()
	defer sh.ioLock.Unlock()

	_, ok := sh.out.(*core.RingBuffer)
	if !ok {
		panic(errors.New("cannot get reader for shell: not a ring buffer"))
	}

	if sh.coreReader == nil {
		sh.coreReader = core.WrapReader(io.MultiReader(sh.out, sh.out), &sh.ioLock)
	}
	return sh.coreReader
}

func (sh *shell) Writer() *core.Writer {
	sh.ioLock.Lock()
	defer sh.ioLock.Unlock()

	_, ok := sh.in.(*core.RingBuffer)
	if !ok {
		panic(errors.New("cannot get writer for shell: not a ring buffer"))
	}

	if sh.coreWriter == nil {
		sh.coreWriter = core.WrapWriter(sh.in, false, &sh.ioLock)
	}
	return sh.coreWriter
}

func (sh *shell) Stream(ctx *core.Context, config *core.ReadableStreamConfiguration) core.ReadableStream {
	sh.ioLock.Lock()
	defer sh.ioLock.Unlock()

	//TODO: prevent future calls to .Reader()

	outBuf, ok := sh.out.(*core.RingBuffer)
	if !ok {
		panic(errors.New("cannot get readable stream for shell: output is not a ring buffer"))
	}

	return outBuf.Stream(ctx, nil)

	// errBuf, ok := sh.errOut.(*core.RingBuffer)
	// if !ok {
	// 	panic(errors.New("cannot get readable stream for shell: error output is not a ring buffer"))
	// }

	// if config != nil && config.Filter != nil {
	// 	panic(errors.New("cannot configure shell's output stream"))
	// }

	// stream, err := core.NewConfluenceStream(ctx, []core.ReadableStream{outBuf.Stream(ctx, nil), errBuf.Stream(ctx, nil)})
	// if err != nil {
	// 	panic(err)
	// }
	// return stream
}

func (sh *shell) WritableStream(ctx *core.Context, config *core.WritableStreamConfiguration) core.WritableStream {
	sh.ioLock.Lock()
	defer sh.ioLock.Unlock()

	//TODO: prevent future calls to .Writer()

	buf, ok := sh.in.(*core.RingBuffer)
	if !ok {
		panic(errors.New("cannot get writable stream for shell: no ring buffer"))
	}

	return buf.WritableStream(ctx, config)
}

func (sh *shell) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "start":
		return core.WrapGoClosure(func(ctx *core.Context) {
			go func() {
				sh.runLoop()
			}()
		}), true
	case "stop":
		return core.WrapGoClosure(func(ctx *core.Context) {

			sh.stopping.Store(true)
		}), true
	}
	return nil, false
}

func (sh *shell) Prop(ctx *core.Context, name string) core.Value {
	method, ok := sh.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, sh))
	}
	return method
}

func (*shell) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*shell) PropertyNames(ctx *core.Context) []string {
	return SHELL_PROPNAMES
}

func (sh *shell) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", sh)
}

func (sh *shell) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic_shell.Shell{}, nil
}

//

func _cd(ctx *core.Context, newdir core.Path) error {
	if !newdir.IsDirPath() {
		return errors.New("cd: the core.Path must be a directory core.Path")
	}

	if err := os.Chdir(string(newdir)); err != nil {
		return errors.New("cd: " + err.Error())
	}
	return nil
}
func _pwd(ctx *core.Context) core.Path {
	dir, _ := os.Getwd()
	return core.Path(core.AppendTrailingSlashIfNotPresent(dir))
}

func _whoami(ctx *core.Context) core.Str {
	user, _ := user.Current()
	return core.Str(user.Username)
}

func _hostname(ctx *core.Context) core.Str {
	name, _ := os.Hostname()
	return core.Str(name)
}
