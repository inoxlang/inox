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

	var helpList []TopicHelp
	if err := yaml.Unmarshal(utils.StringAsBytes(BUILTIN_HELP_YAML), &helpList); err != nil {
		log.Panicf("error while parsing builtin.yaml: %s", err)
	}

	for _, item := range helpList {
		helpByTopic[item.Topic] = item
	}
}

func getValueId(v core.Value) (uintptr, bool) {
	var ptr uintptr
	switch val := v.(type) {
	case *core.GoFunction:
		return getGoFuncId(val.GoFunc()), true
	case *core.InoxFunction:
		ptr = reflect.ValueOf(val).Pointer()
	default:
		return 0, false
	}
	return ptr, true
}

func getGoFuncId(fn any) uintptr {
	return reflect.ValueOf(fn).Pointer()
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

func (h TopicHelp) Print(w io.Writer, config HelpMessageConfig) {
	switch config.Format {
	case ColorizedTerminalFormat:
		w.Write(utils.StringAsBytes(h.Text + "\n\r"))

		if len(h.Examples) > 0 {
			w.Write(utils.StringAsBytes("examples:\n\r"))

			for _, example := range h.Examples {

				chunk, err := parse.ParseChunk(example.Code, "")
				if err != nil {
					continue
				}

				w.Write(utils.StringAsBytes("\n\r- "))
				core.PrintColorizedChunk(w, chunk, []rune(example.Code), false, core.GetFullColorSequence(termenv.ANSIWhite, false))

				if example.Explanation != "" {
					w.Write(utils.StringAsBytes(" # "))
					w.Write(utils.StringAsBytes(example.Explanation))
				}

				if example.Output != "" {
					w.Write(utils.StringAsBytes("\n\r  -> "))

					chunk, err := parse.ParseChunk(example.Output, "")
					if err != nil {
						continue
					}

					core.PrintColorizedChunk(w, chunk, []rune(example.Output), false, core.GetFullColorSequence(termenv.ANSIWhite, false))
				}
				w.Write(utils.StringAsBytes("\n\r"))

			}
		}

		if len(h.RelatedTopics) > 0 {
			w.Write(utils.StringAsBytes("\n\rrelated: " + strings.Join(h.RelatedTopics, ", ")))
			w.Write(utils.StringAsBytes("\n\r"))
		}
	case MarkdownFormat:
		w.Write(utils.StringAsBytes(h.Text + "\n"))

		if len(h.Examples) > 0 {
			w.Write(utils.StringAsBytes("\n**examples**:\n```inox\n"))

			for _, example := range h.Examples {
				w.Write(utils.StringAsBytes(example.Code))

				if example.Explanation != "" {
					w.Write(utils.StringAsBytes(" # "))
					w.Write(utils.StringAsBytes(example.Explanation))
				}

				if example.Output != "" {
					w.Write(utils.StringAsBytes("\n# output:\n"))
					w.Write(utils.StringAsBytes(example.Output))
				}
				w.Write(utils.StringAsBytes("\n\n"))
			}
			w.Write(utils.StringAsBytes("\n```"))
		}

		if len(h.RelatedTopics) > 0 {
			w.Write(utils.StringAsBytes("\nrelated: " + strings.Join(h.RelatedTopics, ", ")))
			w.Write(utils.StringAsBytes("\n"))
		}
	default:
		panic(core.ErrUnreachable)
	}

}

func (h TopicHelp) String(config HelpMessageConfig) string {
	buf := bytes.NewBuffer(nil)
	h.Print(buf, config)
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
	//TODO: support uncolorized
	config := HelpMessageConfig{Format: ColorizedTerminalFormat}

	if ident, ok := arg.(core.Identifier); ok {
		help, ok := helpByTopic[string(ident)]
		if ok {
			str = help.String(config)
		}
	} else {
		id, ok := getValueId(arg)

		if ok {
			help, ok := helpMap[id]
			if ok {
				str = help.String(config)
			}
		}
	}

	out.Write([]byte(str))
	utils.MoveCursorNextLine(out, 1)
}

type HelpMessageFormat int

const (
	ColorizedTerminalFormat HelpMessageFormat = iota + 1
	MarkdownFormat
)

type HelpMessageConfig struct {
	Format HelpMessageFormat
}

func HelpForGoFunc(fn any, config HelpMessageConfig) (string, bool) {
	id := getGoFuncId(fn)
	help, ok := helpMap[id]
	if ok {
		return help.String(config), true
	}
	return "", false
}

func HelpForSymbolicGoFunc(fn *symbolic.GoFunction, config HelpMessageConfig) (string, bool) {
	concreteFn, ok := core.GetConcreteGoFuncFromSymbolic(fn)
	if !ok {
		return "", false
	}

	return HelpForGoFunc(concreteFn.Interface(), config)
}
