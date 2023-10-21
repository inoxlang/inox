package symbolic

import (
	"bytes"
	"fmt"
	"sort"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/exp/maps"
)

// A Namespace represents a symbolic Namespace.
type Namespace struct {
	UnassignablePropsMixin
	entries map[string]Value //if nil, matches any Namespace
}

func NewAnyNamespace() *Namespace {
	return &Namespace{}
}

func NewEmptyNamespace() *Namespace {
	return &Namespace{entries: map[string]Value{}}
}

func NewNamespace(entries map[string]Value) *Namespace {
	return &Namespace{entries: entries}
}

func (ns *Namespace) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherNs, ok := v.(*Namespace)
	if !ok {
		return false
	}

	if ns.entries == nil {
		return true
	}

	for k, e := range ns.entries {
		other, ok := otherNs.entries[k]

		if !ok || !e.Test(other, state) {
			return false
		}
	}

	return true
}

func (ns *Namespace) Prop(name string) Value {
	v, ok := ns.entries[name]
	if !ok {
		panic(fmt.Errorf("Namespace does not have a .%s property", name))
	}
	return v
}

func (ns *Namespace) PropertyNames() []string {
	return maps.Keys(ns.entries)
}

func (ns *Namespace) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if ns.entries != nil {
		if w.Depth > config.MaxDepth && len(ns.entries) > 0 {
			w.WriteString("(..namespace..)")
			return
		}

		indentCount := w.ParentIndentCount + 1
		indent := bytes.Repeat(config.Indent, indentCount)

		w.WriteName("namespace{")

		keys := maps.Keys(ns.entries)
		sort.Strings(keys)

		for i, k := range keys {

			if !config.Compact {
				w.WriteLFCR()
				w.WriteBytes(indent)
			}

			if config.Colorize {
				w.WriteBytes(config.Colors.IdentifierLiteral)
			}

			w.WriteBytes(utils.Must(utils.MarshalJsonNoHTMLEspace(k)))

			if config.Colorize {
				w.WriteBytes(ANSI_RESET_SEQUENCE)
			}

			//colon
			w.WriteBytes(COLON_SPACE)

			//value
			v := ns.entries[k]
			v.PrettyPrint(w.IncrDepthWithIndent(indentCount), config)

			//comma & indent
			isLastEntry := i == len(keys)-1

			if !isLastEntry {
				w.WriteBytes(COMMA_SPACE)
			}
		}

		if !config.Compact && len(keys) > 0 {
			w.WriteLFCR()
		}

		w.WriteManyBytes(bytes.Repeat(config.Indent, w.Depth), []byte{'}'})
		return
	}
	w.WriteName("namespace")
}

func (ns *Namespace) WidestOfType() Value {
	return ANY_REC
}
