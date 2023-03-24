package internal

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"

	core "github.com/inox-project/inox/internal/core"
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
	"github.com/inox-project/inox/internal/utils"
	"github.com/muesli/termenv"

	parse "github.com/inox-project/inox/internal/parse"
)

const (
	helpUsage = "usage: help <topic>\n\rspecial shell commands: quit, clear\n"
)

var (
	helpMap     = map[uintptr]TopicHelp{}
	helpByTitle = map[string]TopicHelp{}
)

func init() {
	core.RegisterSymbolicGoFunctions([]any{
		Help, func(ctx *symbolic.Context, args ...symbolic.SymbolicValue) {},
	})
}

func getValueId(v core.Value) (uintptr, bool) {
	var ptr uintptr
	switch val := v.(type) {
	case *core.GoFunction:
		ptr = reflect.ValueOf(val.GoFunc()).Pointer()
	case *core.InoxFunction:
		ptr = reflect.ValueOf(val).Pointer()
	default:
		return 0, false
	}
	return ptr, true
}

func RegisterHelp(help TopicHelp) {
	v := help.Value

	if v != nil {
		if _, ok := v.(core.Value); !ok {
			v = core.ValOf(v)
		}
		help.Value = v

		id, ok := getValueId(v.(core.Value))
		if !ok {
			panic(fmt.Errorf("failed to register help content of value %#v", v))
		}

		helpMap[id] = help
	}

	if help.Topic != "" {
		helpByTitle[help.Topic] = help
	}
}

func RegisterHelps(help []TopicHelp) {
	for _, item := range help {
		RegisterHelp(item)
	}
}

type TopicHelp struct {
	Value         any
	Topic         string
	RelatedTopics []string
	Summary       string
	Text          string
	Examples      []Example
}

type Example struct {
	Code        string
	Explanation string
	Output      string
}

func (h TopicHelp) Print(w io.Writer) {
	w.Write([]byte(h.Text + "\n\r"))

	if len(h.Examples) > 0 {
		w.Write([]byte("examples:\n\r"))

		for _, example := range h.Examples {

			chunk, err := parse.ParseChunk(example.Code, "")
			if err != nil {
				continue
			}

			w.Write([]byte("\n\r- "))
			core.PrintColorizedChunk(w, chunk, []rune(example.Code), false, core.GetFullColorSequence(termenv.ANSIWhite, false))

			if example.Explanation != "" {
				w.Write([]byte(" # "))
				w.Write([]byte(example.Explanation))
			}

			if example.Output != "" {
				w.Write([]byte("\n\r  -> "))

				chunk, err := parse.ParseChunk(example.Output, "")
				if err != nil {
					continue
				}

				core.PrintColorizedChunk(w, chunk, []rune(example.Output), false, core.GetFullColorSequence(termenv.ANSIWhite, false))
			}
			w.Write([]byte("\n\r"))

		}
	}

	if len(h.RelatedTopics) > 0 {
		w.Write([]byte("\n\rrelated: " + strings.Join(h.RelatedTopics, ", ")))
		w.Write([]byte("\n\r"))
	}

}

func (h TopicHelp) String() string {
	buf := bytes.NewBuffer(nil)
	h.Print(buf)
	return buf.String()
}

func Help(ctx *core.Context, args ...core.Value) {
	out := ctx.GetClosestState().Out
	if len(args) == 0 {
		out.Write([]byte(helpUsage))
		utils.MoveCursorNextLine(out, 1)
		return
	}

	arg := args[0]
	str := "no help found\n"

	if ident, ok := arg.(core.Identifier); ok {

		help, ok := helpByTitle[string(ident)]
		if ok {
			str = help.String()
		}
	} else {
		id, ok := getValueId(arg)

		if ok {
			help, ok := helpMap[id]
			if ok {
				str = help.String()
			}
		}
	}

	out.Write([]byte(str))
	utils.MoveCursorNextLine(out, 1)
}
