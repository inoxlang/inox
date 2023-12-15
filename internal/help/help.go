package help

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"maps"
	"reflect"
	"strings"

	_ "embed"

	"github.com/goccy/go-yaml"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/muesli/termenv"

	parse "github.com/inoxlang/inox/internal/parse"
)

const (
	helpUsage = "usage: help <topic>\n\rspecial shell commands: quit, clear\n"
)

var (
	helpMap     = map[uintptr]TopicHelp{}
	helpByTopic = map[string]TopicHelp{}
	topicGroups map[string]TopicGroup
	//go:embed builtins.yaml
	BUILTIN_HELP_YAML string

	//go:embed language.yaml
	LANGUAGE_HELP_YAML string
)

type TopicGroup struct {
	IsNamespace bool        `yaml:"namespace"`
	Elements    []TopicHelp `yaml:"elements"`
}

func init() {
	core.RegisterSymbolicGoFunctions([]any{
		Help, func(ctx *symbolic.Context, args ...symbolic.Value) {},
	})

	if err := yaml.Unmarshal(utils.StringAsBytes(BUILTIN_HELP_YAML), &topicGroups); err != nil {
		log.Panicf("error while parsing builtins.yaml: %s", err)
	}

	var languageTopicGroups map[string]TopicGroup
	if err := yaml.Unmarshal(utils.StringAsBytes(LANGUAGE_HELP_YAML), &languageTopicGroups); err != nil {
		log.Panicf("error while parsing language.yaml: %s", err)
	}

	maps.Copy(topicGroups, languageTopicGroups)

	var addTopic func(item TopicHelp, groupName string, group TopicGroup)

	addTopic = func(item TopicHelp, groupName string, group TopicGroup) {
		isNamespace := strings.EqualFold(item.Topic, groupName) && group.IsNamespace

		if isNamespace {
			// add all elements of the group as subtopics (except the current topic)
			item.SubTopicNames = append(item.SubTopicNames, utils.FilterMapSlice(group.Elements, func(e TopicHelp) (string, bool) {
				if strings.EqualFold(e.Topic, groupName) {
					return "", false
				}
				return e.Topic, true
			})...)
		}

		item.Text = strings.TrimSpace(item.Text)
		if !strings.HasSuffix(item.Text, ".") {
			item.Text += "."
		}

		for _, subTopic := range item.SubTopics {
			item.SubTopicNames = append(item.SubTopicNames, subTopic.Topic)
			addTopic(subTopic, "?", TopicGroup{})
		}

		helpByTopic[item.Topic] = item

		if item.Alias != "" {
			helpByTopic[item.Alias] = item
		}
	}

	for groupName, group := range topicGroups {
		for _, item := range group.Elements {
			addTopic(item, groupName, group)
		}
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
	Value any
	//scalar
	Topic       string `yaml:"topic"`
	Alias       string `yaml:"alias"`
	Summary     string `yaml:"summary"`
	Text        string `yaml:"text"`
	IsNamespace bool   `yaml:"namespace"`

	//lists
	RelatedTopics []string    `yaml:"related-topics"`
	Examples      []Example   `yaml:"examples"`
	SubTopicNames []string    `yaml:"subtopic-names"`
	SubTopics     []TopicHelp `yaml:"subtopics"`
}

type Example struct {
	Code        string `yaml:"code"`
	Explanation string `yaml:"explanation"`
	Output      string `yaml:"output"`
}

func (h TopicHelp) Print(w io.Writer, config HelpMessageConfig) {
	switch config.Format {
	case ColorizedTerminalFormat:
		w.Write(utils.StringAsBytes(h.Text))

		if len(h.Examples) > 0 {
			w.Write(utils.StringAsBytes("\n\rexamples:\n\r"))

			for _, example := range h.Examples {

				//add carriage returns because of the shell + left padding
				singleLine := !strings.Contains(example.Code, "\n")
				code := strings.ReplaceAll(example.Code, "\n", "\n\r  ")

				chunk, err := parse.ParseChunk(code, "")
				if err != nil {
					continue
				}

				w.Write(utils.StringAsBytes("\n\r- "))
				core.PrintColorizedChunk(w, chunk, []rune(code), false, core.GetFullColorSequence(termenv.ANSIWhite, false))

				if example.Explanation != "" {
					if singleLine {
						w.Write(utils.StringAsBytes(" # "))
					} else {
						w.Write(utils.StringAsBytes("# "))
					}
					w.Write(utils.StringAsBytes(example.Explanation))
				}

				if example.Output != "" {
					w.Write(utils.StringAsBytes("\n\r  -> "))

					chunk, err := parse.ParseChunk(example.Output, "")
					if err != nil {
						continue
					}

					colorized := core.GetColorizedChunk(chunk, []rune(example.Output), false, core.GetFullColorSequence(termenv.ANSIWhite, false))
					colorized = strings.ReplaceAll(colorized, "\n", "\n   ")
					w.Write(utils.StringAsBytes(colorized))
				}
				w.Write(utils.StringAsBytes("\n\r"))

			}
		} else {
			w.Write(utils.StringAsBytes("\n\r"))
		}

		if len(h.SubTopicNames) > 0 {
			w.Write(utils.StringAsBytes("\n\rsubtopics:\n\r\t- " + strings.Join(h.SubTopicNames, "\n\r\t- ")))
			w.Write(utils.StringAsBytes("\n\r"))
		}

		if len(h.RelatedTopics) > 0 {
			w.Write(utils.StringAsBytes("\n\rrelated: " + strings.Join(h.RelatedTopics, ", ")))
			w.Write(utils.StringAsBytes("\n\r"))
		}
		w.Write(utils.StringAsBytes("\n"))
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
	} else if strLike, ok := arg.(core.StringLike); ok {
		s := strLike.GetOrBuildString()
		help, ok := helpByTopic[s]
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

func HelpFor(s string, config HelpMessageConfig) (string, bool) {
	help, ok := helpByTopic[s]
	if ok {
		return help.String(config), true
	}

	return "", false
}
