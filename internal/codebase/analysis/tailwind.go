package analysis

import (
	"strings"

	"github.com/inoxlang/inox/internal/css/tailwind"
	"github.com/inoxlang/inox/internal/parse"
)

func addUsedTailwindRulesets(classAttributeValue parse.Node, result *Result) {
	attrValue := ""

	switch v := classAttributeValue.(type) {
	case *parse.DoubleQuotedStringLiteral:
		attrValue = v.Value
	case *parse.MultilineStringLiteral:
		attrValue = v.Value
		//TODO: support string templates
	default:
		return
	}

	classNames := strings.Split(attrValue, " ")
	for _, name := range classNames {
		name = strings.TrimSpace(name)

		var ruleset tailwind.Ruleset
		modifier, basename, hasModifier := strings.Cut(name, ":")

		if !hasModifier {
			basename = name
		}

		baseRuleset, ok := tailwind.GetBaseRuleset("." + basename)
		if !ok {
			continue
		}

		if hasModifier {
			ruleset = baseRuleset.WithOnlyModifier(modifier)
		} else {
			ruleset = baseRuleset
		}
		result.UsedTailwindRules[ruleset.NameWithModifiers] = ruleset
	}
}
