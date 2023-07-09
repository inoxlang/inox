package symbolic

import (
	"bufio"
	"bytes"
	"fmt"
	"sort"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

// A Namespace represents a symbolic Namespace.
type Namespace struct {
	UnassignablePropsMixin
	entries map[string]SymbolicValue //if nil, matches any Namespace
}

func NewAnyNamespace() *Namespace {
	return &Namespace{}
}

func NewEmptyNamespace() *Namespace {
	return &Namespace{entries: map[string]SymbolicValue{}}
}

func NewNamespace(entries map[string]SymbolicValue) *Namespace {
	return &Namespace{entries: entries}
}

func (rec *Namespace) Test(v SymbolicValue) bool {
	otherNs, ok := v.(*Namespace)
	if !ok {
		return false
	}

	if rec.entries == nil {
		return true
	}

	for k, e := range rec.entries {
		other, ok := otherNs.entries[k]

		if !ok || !e.Test(other) {
			return false
		}
	}

	return true
}

func (rec *Namespace) Prop(name string) SymbolicValue {
	v, ok := rec.entries[name]
	if !ok {
		panic(fmt.Errorf("Namespace does not have a .%s property", name))
	}
	return v
}

func (rec *Namespace) PropertyNames() []string {
	return utils.GetMapKeys(rec.entries)
}

func (rec *Namespace) Widen() (SymbolicValue, bool) {
	if rec.entries == nil {
		return nil, false
	}

	widenedEntries := map[string]SymbolicValue{}
	allAlreadyWidened := true

	for k, v := range rec.entries {
		widened, ok := v.Widen()
		if ok {
			allAlreadyWidened = false
			v = widened
		}
		widenedEntries[k] = v
	}

	if allAlreadyWidened {
		return &Namespace{}, true
	}

	return &Namespace{entries: widenedEntries}, true
}

func (rec *Namespace) IsWidenable() bool {
	return rec.entries != nil
}

func (rec *Namespace) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if rec.entries != nil {
		if depth > config.MaxDepth && len(rec.entries) > 0 {
			utils.Must(w.Write(utils.StringAsBytes("(..namespace..)")))
			return
		}

		indentCount := parentIndentCount + 1
		indent := bytes.Repeat(config.Indent, indentCount)

		utils.Must(w.Write(utils.StringAsBytes("namespace{")))

		keys := utils.GetMapKeys(rec.entries)
		sort.Strings(keys)

		for i, k := range keys {

			if !config.Compact {
				utils.Must(w.Write(LF_CR))
				utils.Must(w.Write(indent))
			}

			if config.Colorize {
				utils.Must(w.Write(config.Colors.IdentifierLiteral))
			}

			utils.Must(w.Write(utils.Must(utils.MarshalJsonNoHTMLEspace(k))))

			if config.Colorize {
				utils.Must(w.Write(ANSI_RESET_SEQUENCE))
			}

			//colon
			utils.Must(w.Write(COLON_SPACE))

			//value
			v := rec.entries[k]
			v.PrettyPrint(w, config, depth+1, indentCount)

			//comma & indent
			isLastEntry := i == len(keys)-1

			if !isLastEntry {
				utils.Must(w.Write(COMMA_SPACE))
			}
		}

		if !config.Compact && len(keys) > 0 {
			utils.Must(w.Write(LF_CR))
		}

		utils.MustWriteMany(w, bytes.Repeat(config.Indent, depth), []byte{'}'})
		return
	}
	utils.Must(w.Write(utils.StringAsBytes("%namespace")))
}

func (r *Namespace) WidestOfType() SymbolicValue {
	return ANY_REC
}
