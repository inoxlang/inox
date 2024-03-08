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
		ruleset, ok := tailwind.GetRuleset("." + name)
		if ok {
			result.UsedTailwindRules[ruleset.Name] = ruleset
		}
	}
}
