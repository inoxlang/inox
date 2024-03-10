package tailwind

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/inoxlang/inox/internal/css"
)

var (
	linefeeds = []byte{'\n', '\n'}
)

type Ruleset struct {
	BaseName             string   //e.g. .h-0, .h-0\.5, .h-1\/2 (no breakpoint or media query ...)
	UserFriendlyBaseName string   //e.g. .h-0, .h-0.5, .h-1/2
	NameWithModifiers    string   //e.g. sm\:h-0
	Ruleset              css.Node //Ruleset (not an at-rule)
	Modifier0            string
	//TODO: support multiple modifiers (at least 2)
}

func (r Ruleset) WithOnlyModifier(modifier string) Ruleset {
	new := r
	new.Modifier0 = modifier
	new.NameWithModifiers = modifier + "\\:" + strings.TrimPrefix(new.BaseName, ".")
	new.Ruleset.UpdateFirstSelectorElement(func(elem css.Node) css.Node {
		if elem.Type == css.ClassName {
			elem.Data = new.NameWithModifiers
			return elem
		}
		panic(fmt.Errorf("selector not supported: %s", new.Ruleset.Children[0].String()))
	})

	return new
}

func (r *Ruleset) String() string {

	if r.Modifier0 == "" {
		return r.Ruleset.String()
	}

	breakpointInfo, ok := GetDefaultBreakpointByName(r.Modifier0)
	if !ok {
		panic(fmt.Errorf("unsupported modifier %s", r.Modifier0))
	}

	atRule := makeMinWidthAtRule(breakpointInfo.MinWidthPx)

	atRule.Children = append(atRule.Children, r.Ruleset)

	return atRule.String()
}

func makeMinWidthAtRule(minWidthPx int) css.Node {
	return css.Node{
		Type: css.AtRule,
		Data: "@media",
		Children: []css.Node{
			{
				Type: css.MediaFeature,
				Data: "min-width",
				Children: []css.Node{
					{
						Type: css.Dimension,
						Data: strconv.Itoa(minWidthPx) + "px",
					},
				},
			},
		},
	}
}
