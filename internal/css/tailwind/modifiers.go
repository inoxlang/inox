package tailwind

import "strings"

var (
	DEFAULT_MODIFIERS []ModifierInfo
)

func init() {
	for _, breakpoint := range DEFAULT_BREAKPOINTS {
		DEFAULT_MODIFIERS = append(DEFAULT_MODIFIERS, ModifierInfo{
			Name:        breakpoint.Name,
			Description: breakpoint.Description,
			Kind:        BreakpointModifier,
		})
	}
}

type ModifierInfo struct {
	Name        string
	Description string
	Kind        ModifierKind
}

type ModifierKind int

const (
	BreakpointModifier ModifierKind = iota
)

func GetModifierInfoByPrefix(prefix string) (modifiers []ModifierInfo) {
	for _, modifier := range DEFAULT_MODIFIERS {
		if strings.HasPrefix(modifier.Name, prefix) {
			modifiers = append(modifiers, modifier)
		}
	}
	return
}
