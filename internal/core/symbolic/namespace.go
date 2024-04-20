package symbolic

import (
	"fmt"
	"sort"

	pprint "github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/exp/maps"
)

var (
	ANY_NAMESPACE                 = &Namespace{}
	ANY_MUTABLE_ENTRIES_NAMESPACE = &Namespace{
		checkMutability: true,
		mutableEntries:  true,
	}
	ANY_IMMUTABLE_NAMESPACE = &Namespace{
		checkMutability: true,
		mutableEntries:  false,
	}
)

// A Namespace represents a symbolic Namespace.
type Namespace struct {
	UnassignablePropsMixin
	entries map[string]Value //if nil, matches any Namespace

	mutableEntries  bool
	checkMutability bool
}

func NewEmptyNamespace() *Namespace {
	return &Namespace{entries: map[string]Value{}}
}

func NewEmptyMutableEntriesNamespace() *Namespace {
	return &Namespace{
		entries:         map[string]Value{},
		mutableEntries:  true,
		checkMutability: true,
	}
}

func NewNamespace(entries map[string]Value) *Namespace {
	return &Namespace{
		entries:         entries,
		checkMutability: true,
	}
}

func NewMutableEntriesNamespace(entries map[string]Value) *Namespace {
	return &Namespace{
		entries:         entries,
		mutableEntries:  true,
		checkMutability: true,
	}
}

func (ns *Namespace) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherNs, ok := v.(*Namespace)
	if !ok {
		return false
	}

	if ns.checkMutability && ns.mutableEntries != otherNs.mutableEntries {
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
	keys := maps.Keys(ns.entries)
	sort.Strings(keys)
	return keys
}

func (ns *Namespace) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w = w.IncrDepth()

	if ns.entries != nil {
		if w.Depth > config.MaxDepth && len(ns.entries) > 0 {
			w.WriteString("(..namespace..)")
			return
		}

		w.WriteName("namespace{")

		keys := maps.Keys(ns.entries)
		sort.Strings(keys)

		for i, k := range keys {

			if !config.Compact {
				w.WriteEndOfLine()
				w.WriteInnerIndent()
			}

			if config.Colorize {
				w.WriteBytes(config.Colors.IdentifierLiteral)
			}

			w.WriteBytes(utils.Must(utils.MarshalJsonNoHTMLEspace(k)))

			if config.Colorize {
				w.WriteAnsiReset()
			}

			//colon
			w.WriteString(": ")

			//value
			v := ns.entries[k]
			v.PrettyPrint(w.IncrIndent(), config)

			//comma & indent
			isLastEntry := i == len(keys)-1

			if !isLastEntry {
				w.WriteString(", ")
			}
		}

		if !config.Compact && len(keys) > 0 {
			w.WriteEndOfLine()
		}

		w.WriteOuterIndent()
		w.WriteByte('}')
		return
	}
	w.WriteName("namespace")
}

func (ns *Namespace) WidestOfType() Value {
	return ANY_NAMESPACE
}
