package internal

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"reflect"
	"strings"

	_ "embed"

	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/muesli/termenv"
	"gopkg.in/yaml.v3"

	parse "github.com/inoxlang/inox/internal/parse"
)

const (
	helpUsage = "usage: help <topic>\n\rspecial shell commands: quit, clear\n"
)

var (
	helpMap     = map[uintptr]TopicHelp{}
	helpByTopic = map[string]TopicHelp{}

	//go:embed builtin.yaml
	BUILTIN_HELP_YAML string
)

func init() {
	core.RegisterSymbolicGoFunctions([]any{
		Help, func(ctx *symbolic.Context, args ...symbolic.SymbolicValue) {},
	})

	if err := yaml.Unmarshal(utils.StringAsBytes(BUILTIN_HELP_YAML), helpByTopic); err != nil {
		log.Panicf("error while parsing builtin.yaml: %s", err)
	}
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

func RegisterHelpValue(v any, topic string) {
	help, ok := helpByTopic[topic]
	if !ok {
		panic(fmt.Errorf("help topic '%s' does not exist", topic))
	}

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
		helpByTopic[help.Topic] = help
	}
}

func RegisterHelpValues(values map[string]any) {
	for k, v := range values {
		RegisterHelpValue(v, k)
	}
}

type TopicHelp struct {
	Value         any
	Topic         string    `json:"topic"`
	RelatedTopics []string  `json:"related-topics"`
	Summary       string    `json:"summary"`
	Text          string    `json:"text"`
	Examples      []Example `yaml:"examples"`
}

type Example struct {
	Code        string `json:"code"`
	Explanation string `json:"explanation"`
	Output      string `json:"output"`
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

		help, ok := helpByTopic[string(ident)]
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
