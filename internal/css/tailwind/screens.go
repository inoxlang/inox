package tailwind

import (
	"slices"
	"strings"
)

const (
	SM_BREAKPOINT   = "sm"
	MD_BREAKPOINT   = "md"
	LG_BREAKPOINT   = "lg"
	XL_BREAKPOINT   = "xl"
	_2XL_BREAKPOINT = "2xl"
)

type Breakpoint struct {
	Name        string
	Description string
	MinWidthPx  int
}

var (
	DEFAULT_BREAKPOINTS = []Breakpoint{
		{
			Name:        SM_BREAKPOINT,
			Description: "small (min-width: 640px)",
			MinWidthPx:  640,
		},
		{
			Name:        MD_BREAKPOINT,
			Description: "medium (min-width: 768px)",
			MinWidthPx:  768,
		},
		{
			Name:        LG_BREAKPOINT,
			Description: "large (min-width: 1024px)",
			MinWidthPx:  1024,
		},
		{
			Name:        XL_BREAKPOINT,
			Description: "extra large (min-width: 1280px)",
			MinWidthPx:  1280,
		},
		{
			Name:        _2XL_BREAKPOINT,
			Description: "extra large 2 (min-width: 1536px)",
			MinWidthPx:  1536,
		},
	}
)

func IsDefaultBreakpointName(name string) bool {
	return slices.ContainsFunc(DEFAULT_BREAKPOINTS, func(b Breakpoint) bool {
		return b.Name == name
	})
}

func GetDefaultBreakpointByName(name string) (Breakpoint, bool) {
	for _, breakpoint := range DEFAULT_BREAKPOINTS {
		if breakpoint.Name == name {
			return breakpoint, true
		}
	}
	return Breakpoint{}, false
}

func GetBreakpointNamesByPrefix(prefix string) (names []string) {
	for _, breakpoint := range DEFAULT_BREAKPOINTS {
		if strings.HasPrefix(breakpoint.Name, prefix) {
			names = append(names, breakpoint.Name)
		}
	}
	return
}
