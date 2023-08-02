package symbolic

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	WALK_ELEM = NewObject(map[string]Serializable{
		"name":          &String{},
		"path":          &Path{},
		"is-dir":        ANY_BOOL,
		"is-regular":    ANY_BOOL,
		"is-walk-start": ANY_BOOL,
	}, nil, nil)

	ANY_PATH     = &Path{}
	ANY_DIR_PATH = &Path{
		dirConstraint: DirPath,
	}
	ANY_NON_DIR_PATH = &Path{
		dirConstraint: NonDirPath,
	}
	ANY_ABS_PATH = &Path{
		absoluteness: AbsolutePath,
	}
	ANY_REL_PATH = &Path{
		absoluteness: RelativePath,
	}
	ANY_ABS_DIR_PATH = &Path{
		absoluteness:  AbsolutePath,
		dirConstraint: DirPath,
	}
	ANY_ABS_NON_DIR_PATH = &Path{
		absoluteness:  AbsolutePath,
		dirConstraint: NonDirPath,
	}
	ANY_REL_DIR_PATH = &Path{
		absoluteness:  RelativePath,
		dirConstraint: DirPath,
	}
	ANY_REL_NON_DIR_PATH = &Path{
		absoluteness:  RelativePath,
		dirConstraint: NonDirPath,
	}
	ANY_URL    = &URL{}
	ANY_SCHEME = &Scheme{}
	ANY_HOST   = &Host{}
	ANY_PORT   = &Port{}

	PATH_PROPNAMES = []string{"segments", "extension", "name", "dir", "ends_with_slash", "rel_equiv", "change_extension", "join"}
)

// A Path represents a symbolic Path.
type Path struct {
	absoluteness  PathAbsoluteness
	dirConstraint DirPathConstraint

	UnassignablePropsMixin
	SerializableMixin
}

type PathAbsoluteness int
type DirPathConstraint int

const (
	UnspecifiedPathAbsoluteness PathAbsoluteness = iota
	AbsolutePath
	RelativePath
)

const (
	UnspecifiedDirOrFilePath DirPathConstraint = iota
	DirPath
	NonDirPath
)

func (p *Path) Test(v SymbolicValue) bool {
	otherPath, ok := v.(*Path)
	if !ok {
		return false
	}

	if p.absoluteness != UnspecifiedPathAbsoluteness && otherPath.absoluteness != p.absoluteness {
		return false
	}

	if p.dirConstraint != UnspecifiedDirOrFilePath && otherPath.dirConstraint != p.dirConstraint {
		return false
	}

	return ok
}

func (p *Path) Static() Pattern {
	return ANY_PATH_PATTERN
}

func (p *Path) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	s := "%path"

	switch p.absoluteness {
	case AbsolutePath:
		s += "(#abs"
	case RelativePath:
		s += "(#rel"
	}

	if p.absoluteness != UnspecifiedPathAbsoluteness && p.dirConstraint != UnspecifiedDirOrFilePath {
		s += ","
	} else if p.dirConstraint != UnspecifiedDirOrFilePath {
		s += "("
	}

	switch p.dirConstraint {
	case DirPath:
		s += "#dir"
	case NonDirPath:
		s += "#non-dir"
	}

	if p.absoluteness != UnspecifiedPathAbsoluteness || p.dirConstraint != UnspecifiedDirOrFilePath {
		s += ")"
	}

	utils.Must(w.Write(utils.StringAsBytes(s)))
}

func (p *Path) ResourceName() *String {
	return ANY_STR
}

func (p *Path) PropertyNames() []string {
	return PATH_PROPNAMES
}

func (p *Path) Prop(name string) SymbolicValue {
	switch name {
	case "segments":
		return &List{generalElement: &String{}}
	case "extension":
		return ANY_STR
	case "name":
		return ANY_STR
	case "dir":
		switch p.absoluteness {
		case AbsolutePath:
			return ANY_ABS_DIR_PATH
		case RelativePath:
			return ANY_REL_DIR_PATH
		}
		return ANY_DIR_PATH
	case "ends_with_slash":
		return ANY_BOOL
	case "rel_equiv":
		return ANY_PATH
	case "change_extension":
		return &GoFunction{
			fn: func(ctx *Context, newExtension *String) *Path {
				return p
			},
		}
	case "join":
		return &GoFunction{
			fn: func(ctx *Context, relativePath *Path) *Path {
				return p
			},
		}
	default:
		return nil
	}
}

func (p *Path) underylingString() *String {
	return ANY_STR
}

func (p *Path) WalkerElement() SymbolicValue {
	return WALK_ELEM
}

func (p *Path) WalkerNodeMeta() SymbolicValue {
	return ANY
}

func (p *Path) WidestOfType() SymbolicValue {
	return ANY_PATH
}

// A URL represents a symbolic URL.
type URL struct {
	UnassignablePropsMixin
	SerializableMixin
}

func (u *URL) Test(v SymbolicValue) bool {
	_, ok := v.(*URL)
	return ok
}

func (u *URL) Static() Pattern {
	return ANY_URL_PATTERN
}

func (u *URL) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%url")))
}

func (u *URL) underylingString() *String {
	return ANY_STR
}

func (u *URL) ResourceName() *String {
	return ANY_STR
}

func (u *URL) PropertyNames() []string {
	return []string{"scheme", "host", "path", "raw_query"}
}

func (u *URL) Prop(name string) SymbolicValue {
	switch name {
	case "scheme":
		return ANY_STR
	case "host":
		return &Host{}
	case "path":
		return &Path{}
	case "raw_query":
		return ANY_STR
	default:
		return nil
	}
}

func (u *URL) WidestOfType() SymbolicValue {
	return ANY_URL
}

// A Scheme represents a symbolic Scheme.
type Scheme struct {
	_ int
}

func (s *Scheme) Test(v SymbolicValue) bool {
	_, ok := v.(*Scheme)
	return ok
}

func (s *Scheme) Static() Pattern {
	return &TypePattern{val: ANY_SCHEME}
}

func (s *Scheme) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%scheme")))
}

func (s *Scheme) underylingString() *String {
	return ANY_STR
}

func (s *Scheme) WidestOfType() SymbolicValue {
	return ANY_SCHEME
}

//

// A Host represents a symbolic Host.
type Host struct {
	UnassignablePropsMixin
	SerializableMixin
}

func (h *Host) Test(v SymbolicValue) bool {
	_, ok := v.(*Host)
	return ok
}

func (h *Host) Static() Pattern {
	return ANY_HOST_PATTERN
}

func (h *Host) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%host")))
}

func (h *Host) ResourceName() *String {
	return ANY_STR
}

func (s *Host) PropertyNames() []string {
	return []string{"scheme", "explicit_port", "without_port"}
}

func (*Host) Prop(name string) SymbolicValue {
	switch name {
	case "scheme":
		return ANY_STR
	case "explicit_port":
		return &Int{}
	case "without_port":
		return &Host{}
	default:
		return nil
	}
}

func (h *Host) underylingString() *String {
	return ANY_STR
}

func (h *Host) WidestOfType() SymbolicValue {
	return ANY_HOST
}
