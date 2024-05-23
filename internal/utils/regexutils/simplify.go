package regexutils

import (
	"errors"
	"regexp/syntax"
	"slices"
)

// DestructivelySimplify simplifies the regex ($regex is not modified), it first calls TurnCapturingGroupsIntoNonCapturing,
// then syntax.Regexp.DestructivelySimplify, and finally it tries its best to simplify the elements of shape AA* into A+.
func DestructivelySimplify(regex *syntax.Regexp) *syntax.Regexp {
	regex = TurnCapturingGroupsIntoNonCapturing(regex)
	regex = regex.Simplify()
	return turnAAStarIntoAplus(regex)
}

// turnAAStarIntoAplus tries its best to simplify elements of shape AA* into A+.
// Capturing groups are not supported.
func turnAAStarIntoAplus(regex *syntax.Regexp) *syntax.Regexp {
	newRegex := new(syntax.Regexp)

	if regex.Op != syntax.OpConcat {
		*newRegex = *regex
	}

	if regex.Op == syntax.OpCapture && regex.Cap >= 1 {
		panic(errors.New("capturing groups are not supported"))
	}

	switch regex.Op {
	case syntax.OpConcat:
		newRegex.Op = syntax.OpConcat
		newRegex.Flags = regex.Flags

		if len(newRegex.Sub) == 1 {
			return newRegex
		}

		newRegex.Sub = make([]*syntax.Regexp, 0, len(regex.Sub))
		newIndex := 0

		for i, sub := range regex.Sub {
			newElem := turnAAStarIntoAplus(sub)

			//A*
			if i != 0 && newElem.Op == syntax.OpStar {
				prevNewElem := newRegex.Sub[newIndex-1]
				repeated := newElem.Sub[0]

				if prevNewElem.Equal(repeated) {
					//Turn the previous element (A) into (A)+ and do not append $newElem.
					newRegex.Sub[newIndex-1] = &syntax.Regexp{
						Op:    syntax.OpPlus,
						Flags: prevNewElem.Flags,
						Sub:   []*syntax.Regexp{prevNewElem},
					}
					continue
				}

				//elem(AB)elem(B*) -> elem(A)elem(B+)
				if prevNewElem.Op == syntax.OpLiteral &&
					repeated.Op == syntax.OpLiteral &&
					len(prevNewElem.Rune) > len(repeated.Rune) {
					prevNewElemRunes := prevNewElem.Rune
					repeatedRunes := repeated.Rune

					end := prevNewElemRunes[len(prevNewElemRunes)-len(repeatedRunes):]

					if slices.Equal(end, repeatedRunes) {
						//Turn the previous element (AB) into (A) and add B+ instead of B* ($newElem).
						prevNewElem.Rune = prevNewElemRunes[:len(prevNewElemRunes)-len(repeatedRunes)]
						newElem.Op = syntax.OpPlus
					}
				}

			}
			newRegex.Sub = append(newRegex.Sub, newElem)
			newIndex++
		}

		if len(newRegex.Sub) == 1 {
			return newRegex.Sub[0]
		}
	}

	return newRegex
}
