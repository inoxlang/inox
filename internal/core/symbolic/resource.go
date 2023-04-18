package internal

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	WALK_ELEM = NewObject(map[string]SymbolicValue{
		"name":        &String{},
		"path":        &Path{},
		"isDir":       &Bool{},
		"isRegular":   &Bool{},
		"isWalkStart": &Bool{},
	}, nil)

	ANY_PATH       = &Path{}
	PATH_PROPNAMES = []string{"segments", "extension", "name", "dir", "ends_with_slash", "rel_equiv", "change_extension", "join"}
)

// A Path represents a symbolic Path.
type Path struct {
	UnassignablePropsMixin
	_ int
}

func (i *Path) Test(v SymbolicValue) bool {
	_, ok := v.(*Path)
	return ok
}

func (a *Path) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *Path) IsWidenable() bool {
	return false
}

func (a *Path) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%path")))
	return
}

func (p *Path) ResourceName() *String {
	return &String{}
}

func (p *Path) PropertyNames() []string {
	return PATH_PROPNAMES
}

func (*Path) Prop(name string) SymbolicValue {
	switch name {
	case "segments":
		return &List{generalElement: &String{}}
	case "extension":
		return &String{}
	case "name":
		return &String{}
	case "dir":
		return &Path{}
	case "ends_with_slash":
		return &Bool{}
	case "rel_equiv":
		return &Path{}
	case "change_extension":
		return &GoFunction{
			fn: func(ctx *Context, newExtension *String) *Path {
				return &Path{}
			},
		}
	case "join":
		return &GoFunction{
			fn: func(ctx *Context, relativePath *Path) *Path {
				return &Path{}
			},
		}
	default:
		return nil
	}
}

func (s *Path) underylingString() *String {
	return &String{}
}

func (s *Path) WalkerElement() SymbolicValue {
	return WALK_ELEM
}

func (s *Path) WalkerNodeMeta() SymbolicValue {
	return ANY
}

func (s *Path) WidestOfType() SymbolicValue {
	return &Path{}
}

// A URL represents a symbolic URL.
type URL struct {
	UnassignablePropsMixin
	_ int
}

func (u *URL) Test(v SymbolicValue) bool {
	_, ok := v.(*URL)
	return ok
}

func (u *URL) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (u *URL) IsWidenable() bool {
	return false
}

func (u *URL) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%url")))
	return
}

func (u *URL) underylingString() *String {
	return &String{}
}

func (u *URL) ResourceName() *String {
	return &String{}
}

func (u *URL) PropertyNames() []string {
	return []string{"scheme", "host", "path", "raw_query"}
}

func (u *URL) Prop(name string) SymbolicValue {
	switch name {
	case "scheme":
		return &String{}
	case "host":
		return &Host{}
	case "path":
		return &Path{}
	case "raw_query":
		return &String{}
	default:
		return nil
	}
}

func (u *URL) WidestOfType() SymbolicValue {
	return &URL{}
}

// A Scheme represents a symbolic Scheme.
type Scheme struct {
	_ int
}

func (s *Scheme) Test(v SymbolicValue) bool {
	_, ok := v.(*Scheme)
	return ok
}

func (s *Scheme) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (s *Scheme) IsWidenable() bool {
	return false
}

func (s *Scheme) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%scheme")))
	return
}

func (s *Scheme) underylingString() *String {
	return &String{}
}

func (s *Scheme) WidestOfType() SymbolicValue {
	return &Scheme{}
}

//

// A Host represents a symbolic Host.
type Host struct {
	UnassignablePropsMixin
	_ int
}

func (s *Host) Test(v SymbolicValue) bool {
	_, ok := v.(*Host)
	return ok
}

func (s *Host) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (s *Host) IsWidenable() bool {
	return false
}

func (s *Host) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%host")))
	return
}

func (h *Host) ResourceName() *String {
	return &String{}
}

func (s *Host) PropertyNames() []string {
	return []string{"scheme", "explicit_port", "without_port"}
}

func (*Host) Prop(name string) SymbolicValue {
	switch name {
	case "scheme":
		return &String{}
	case "explicit_port":
		return &Int{}
	case "without_port":
		return &Host{}
	default:
		return nil
	}
}

func (s *Host) underylingString() *String {
	return &String{}
}

func (s *Host) WidestOfType() SymbolicValue {
	return &Host{}
}
