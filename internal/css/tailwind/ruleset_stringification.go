package tailwind

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/inoxlang/inox/internal/css"
	"github.com/inoxlang/inox/internal/utils"
)

func WriteRulesets(w io.Writer, rulesets []Ruleset) error {
	rulesets = slices.Clone(rulesets)
	//remove duplicate
	slices.SortFunc(rulesets, func(a, b Ruleset) int {
		return strings.Compare(a.NameWithModifiers, b.NameWithModifiers)
	})

	deduplicatedRulesets := []Ruleset{rulesets[0]}

	for i := 1; i < len(rulesets); i++ {
		if rulesets[i].NameWithModifiers == rulesets[i-1].NameWithModifiers {
			continue
		}
		deduplicatedRulesets = append(deduplicatedRulesets, rulesets[i])
	}

	//output

	type breakpointAtRule struct {
		modifier string
		node     css.Node
	}

	var (
		regularRulesets []Ruleset
		breakpoints     []breakpointAtRule
	)

	for _, ruleset := range deduplicatedRulesets {
		modifier := ruleset.Modifier0

		if IsDefaultBreakpointName(modifier) {
			//Add the ruleset to the breakpoint's at-rule.

			breakpointInfo := utils.MustGet(GetDefaultBreakpointByName(modifier))
			breakpointIndex := slices.IndexFunc(breakpoints, func(r breakpointAtRule) bool { return r.modifier == modifier })

			var breakpoint *breakpointAtRule

			if breakpointIndex < 0 {
				breakpoints = append(breakpoints, breakpointAtRule{
					modifier: modifier,
					node:     makeMinWidthAtRule(breakpointInfo.MinWidthPx),
				})
				breakpoint = &breakpoints[len(breakpoints)-1]
			} else {
				breakpoint = &breakpoints[breakpointIndex]
			}

			breakpoint.node.Children = append(breakpoint.node.Children, ruleset.Ruleset)

			continue
		}

		if modifier != "" {
			panic(fmt.Errorf("modifier %s is not supported yet", modifier))
		}

		regularRulesets = append(regularRulesets, ruleset)
	}

	//Write rulesets and at-rules.
	_, err := w.Write(linefeeds)
	if err != nil {
		return err
	}

	for _, breakpoint := range breakpoints {
		err := breakpoint.node.WriteTo(w)
		if err != nil {
			return err
		}
		_, err = w.Write(linefeeds)
		if err != nil {
			return err
		}
	}

	for _, ruleset := range regularRulesets {
		err := ruleset.Ruleset.WriteTo(w)
		if err != nil {
			return err
		}
		_, err = w.Write(linefeeds)
		if err != nil {
			return err
		}
	}

	return nil
}
