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
	Name              string //e.g. .h-0, .h-0\.5, .h-1\/2
	UserFriendltyName string //e.g. .h-0, .h-0.5, .h-1/2
	Node              css.Node
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
			name := n.SelectorString()
			userFriendlyName := strings.ReplaceAll(name, "\\.", ".")
			userFriendlyName = strings.ReplaceAll(userFriendlyName, "\\/", "/")

			TAILWIND_SUBSET_RULESETS = append(TAILWIND_SUBSET_RULESETS, Ruleset{
				Name:              name,
				UserFriendltyName: userFriendlyName,
				Node:              n,
			})
		}
	}

	slices.SortFunc(TAILWIND_SUBSET_RULESETS, func(a, b Ruleset) int {
		return strings.Compare(a.Name, b.Name)
	})

	return nil
}

// GetRulesetsFromSubset searches for all rulesets whose selector starts with $prefix.
// Note that '.5' and '/<digit>' (e.g. /2) sequences in $prefix are respectively escaped into '\.5' and '\/<digit>' (e.g. \/2).
func GetRulesetsFromSubset(prefix string) []Ruleset {
	//escape .5 and /<digit>

	var escaped []byte
	escaped = append(escaped, prefix[0])

	for i := 1; i < len(prefix); i++ {
		b := prefix[i]

		switch {
		case b == '5' && prefix[i-1] == '.' && ( /*check if already escaped*/ i == 1 || prefix[i-2] != '\\'):
			escaped[i-1] = '\\'
			escaped = append(escaped, '.', b)
		case '0' <= b && b <= '9' && prefix[i-1] == '/' && ( /*check if already escaped*/ i == 1 || prefix[i-2] != '\\'):
			escaped[i-1] = '\\'
			escaped = append(escaped, '/', b)
		default:
			escaped = append(escaped, b)
		}
	}

	prefix = string(escaped)

	index, _ := slices.BinarySearchFunc(TAILWIND_SUBSET_RULESETS, prefix, func(r Ruleset, s string) int {
		return strings.Compare(r.Name, s)
	})

	//Example: if prefix is `.h` $index is the position of the first .hXXXXX rule.

	var rulesets []Ruleset

	for i := index; i < len(TAILWIND_SUBSET_RULESETS) && strings.HasPrefix(TAILWIND_SUBSET_RULESETS[i].Name, prefix); i++ {
		rulesets = append(rulesets, TAILWIND_SUBSET_RULESETS[i])
	}

	return rulesets
}
