package tailwind

import (
	_ "embed"
	"errors"
	"slices"
	"strings"

	"github.com/inoxlang/inox/internal/css"
)

var (
	//go:embed subset.css
	TAIL_CSS string

	TAILWIND_SUBSET_RULESETS []Ruleset //sorted by selector

	ErrSubsetAlreadyInitialized = errors.New("subset is already initialized")
	ErrSubsetNotInitialized     = errors.New("subset is not initialized")
)

type Ruleset struct {
	Name string
	Node css.Node
}

func InitSubset() error {
	if TAILWIND_SUBSET_RULESETS != nil {
		return ErrSubsetAlreadyInitialized
	}

	stylesheet, err := css.ParseString(TAIL_CSS)
	if err != nil {
		return err
	}

	for _, n := range stylesheet.Children {
		if n.Type == css.Ruleset {
			TAILWIND_SUBSET_RULESETS = append(TAILWIND_SUBSET_RULESETS, Ruleset{
				Name: n.SelectorString(),
				Node: n,
			})
		}
	}

	slices.SortFunc(TAILWIND_SUBSET_RULESETS, func(a, b Ruleset) int {
		return strings.Compare(a.Name, b.Name)
	})

	return nil
}

func GetRulesetsFromSubset(prefix string) []Ruleset {

	index, found := slices.BinarySearchFunc(TAILWIND_SUBSET_RULESETS, prefix, func(r Ruleset, s string) int {
		return strings.Compare(r.Name, s)
	})

	if found {
		return []Ruleset{TAILWIND_SUBSET_RULESETS[index]}
	}

	//Example: if prefix is `.h` $index is the position of the first .hXXXXX rule.

	var rulesets []Ruleset

	for i := index; i < len(TAILWIND_SUBSET_RULESETS) && strings.HasPrefix(TAILWIND_SUBSET_RULESETS[i].Name, prefix); i++ {
		rulesets = append(rulesets, TAILWIND_SUBSET_RULESETS[i])
	}

	return rulesets
}
