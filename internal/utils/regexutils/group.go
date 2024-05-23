package regexutils

import (
	"fmt"
	"regexp/syntax"
)

func TurnCapturingGroupsIntoNonCapturing(regex *syntax.Regexp) *syntax.Regexp {
	newRegex := new(syntax.Regexp)

	if regex.Op != syntax.OpCapture {
		newRegex.Op = regex.Op
		newRegex.Flags = regex.Flags
	}

	switch regex.Op {
	case syntax.OpConcat:
		newRegex.Sub = make([]*syntax.Regexp, len(regex.Sub))
		for i, sub := range regex.Sub {
			newRegex.Sub[i] = TurnCapturingGroupsIntoNonCapturing(sub)
		}

	case syntax.OpLiteral:
		newRegex.Rune = regex.Rune

	case syntax.OpCharClass:
		newRegex.Rune = regex.Rune

	case syntax.OpQuest, syntax.OpPlus, syntax.OpStar:
		newRegex.Sub = []*syntax.Regexp{TurnCapturingGroupsIntoNonCapturing(regex.Sub[0])}

	case syntax.OpRepeat:
		newRegex.Min = regex.Min
		newRegex.Max = regex.Max
		newRegex.Sub = []*syntax.Regexp{TurnCapturingGroupsIntoNonCapturing(regex.Sub[0])}

	case syntax.OpCapture:
		return TurnCapturingGroupsIntoNonCapturing(regex.Sub[0])

	case syntax.OpAlternate:
		newRegex.Sub = make([]*syntax.Regexp, len(regex.Sub))
		for i, sub := range regex.Sub {
			newRegex.Sub[i] = TurnCapturingGroupsIntoNonCapturing(sub)
		}

	case syntax.OpAnyChar, syntax.OpAnyCharNotNL, syntax.OpEmptyMatch, syntax.OpNoWordBoundary, syntax.OpWordBoundary:
	case syntax.OpEndText, syntax.OpBeginText:

	default:
		panic(fmt.Errorf("unknown syntax operator %s", regex.Op.String()))
	}

	return newRegex
}
