package tailwind

import (
	"context"
	_ "embed"
	"errors"
	"slices"
	"strings"
	"sync"

	"github.com/inoxlang/inox/internal/css"
)

var (
	//go:embed subset.css
	TAIL_CSS string

	BASE_RULESETS []Ruleset //sorted by selector

	ErrSubsetAlreadyInitialized = errors.New("subset is already initialized")
	ErrSubsetNotInitialized     = errors.New("subset is not initialized")

	//Cache used for prefixes with potentially a lot of rulesets.
	baseRulesetCache     = map[string][]Ruleset{}
	baseRulesetCacheLock sync.Mutex
)

func InitSubset() error {
	if BASE_RULESETS != nil {
		return ErrSubsetAlreadyInitialized
	}

	stylesheet, err := css.ParseString(context.Background(), TAIL_CSS)
	if err != nil {
		return err
	}

	for _, n := range stylesheet.Children {
		if n.Type == css.Ruleset {
			name := n.SelectorString()
			userFriendlyName := strings.ReplaceAll(name, "\\.", ".")
			userFriendlyName = strings.ReplaceAll(userFriendlyName, "\\/", "/")

			BASE_RULESETS = append(BASE_RULESETS, Ruleset{
				BaseName:             name,
				UserFriendlyBaseName: userFriendlyName,
				NameWithModifiers:    strings.TrimPrefix(name, "."),
				Ruleset:              n,
			})
		}
	}

	slices.SortFunc(BASE_RULESETS, func(a, b Ruleset) int {
		return strings.Compare(a.BaseName, b.BaseName)
	})

	//Remove possible duplicates.

	for i := 1; i < len(BASE_RULESETS); i++ {
		if BASE_RULESETS[i].BaseName == BASE_RULESETS[i-1].BaseName {
			copy(BASE_RULESETS[i-1:], BASE_RULESETS[i:])
			BASE_RULESETS = BASE_RULESETS[:len(BASE_RULESETS)-1]
		}
	}

	return nil
}

// GetBaseRuleset retrieves a base ruleset (no modifier) by its name.
// Note that '.5', ':' and '/<digit>' (e.g. /2) sequences in $prefix are respectively escaped into '\.5', '\:' and '\/<digit>' (e.g. \/2).
func GetBaseRuleset(selector string) (Ruleset, bool) {
	selector = escapeSelector(selector)
	index, found := slices.BinarySearchFunc(BASE_RULESETS, selector, func(r Ruleset, s string) int {
		return strings.Compare(r.BaseName, s)
	})

	if found {
		return BASE_RULESETS[index], true
	}
	return Ruleset{}, false
}

// GetRulesetsFromSubset searches for all base rulesets whose selector starts with $prefix, modifiers are not supported.
// Note that '.5', ':' and '/<digit>' (e.g. /2) sequences in $prefix are respectively escaped into '\.5', '\:' and '\/<digit>' (e.g. \/2).
func GetRulesetsFromSubset(prefix string) []Ruleset {

	if len(prefix) == 0 || prefix == "." {
		return nil
	}

	if strings.Contains(prefix, ":") {
		//The prefix should not contain modifiers.
		return nil
	}

	prefix = escapeSelector(prefix)

	var (
		rulesets []Ruleset
		cacheHit bool
	)

	useCache := len(prefix) <= 3

	if useCache {
		baseRulesetCacheLock.Lock()
		rulesets, cacheHit = baseRulesetCache[prefix]
		baseRulesetCacheLock.Unlock()
	}

	if !cacheHit || !useCache {
		index, _ := slices.BinarySearchFunc(BASE_RULESETS, prefix, func(r Ruleset, s string) int {
			return strings.Compare(r.BaseName, s)
		})

		//Example: if prefix is `.h` $index is the position of the first .hXXXXX rule.

		for i := index; i < len(BASE_RULESETS) && strings.HasPrefix(BASE_RULESETS[i].BaseName, prefix); i++ {
			rulesets = append(rulesets, BASE_RULESETS[i])
		}
	}

	if useCache && !cacheHit && len(rulesets) != 0 { //Update cache.
		baseRulesetCacheLock.Lock()
		baseRulesetCache[prefix] = rulesets
		baseRulesetCacheLock.Unlock()
	}

	return rulesets
}

func escapeSelector(selector string) string {
	//escape .5, ':' and /<digit>

	var escaped []byte
	escaped = append(escaped, selector[0])

	for i := 1; i < len(selector); i++ {
		b := selector[i]

		switch {
		case b == '5' && selector[i-1] == '.' && ( /*check if already escaped*/ i == 1 || selector[i-2] != '\\'):
			escaped[i-1] = '\\'
			escaped = append(escaped, '.', b)
		case '0' <= b && b <= '9' && selector[i-1] == '/' && ( /*check if already escaped*/ i == 1 || selector[i-2] != '\\'):
			escaped[i-1] = '\\'
			escaped = append(escaped, '/', b)
		case b == ':':
			escaped = append(escaped, '\\', b)
		default:
			escaped = append(escaped, b)
		}
	}

	return string(escaped)
}
