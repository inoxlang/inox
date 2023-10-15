package symbolic

import (
	"bufio"
	"errors"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	WALK_ELEM = NewInexactObject(map[string]Serializable{
		"name":          &String{},
		"path":          &Path{},
		"is-dir":        ANY_BOOL,
		"is-regular":    ANY_BOOL,
		"is-walk-start": ANY_BOOL,
	}, nil, nil)

	ANY_PATH             = &Path{}
	ANY_DIR_PATH         = &Path{pattern: ANY_DIR_PATH_PATTERN}
	ANY_NON_DIR_PATH     = &Path{pattern: ANY_NON_DIR_PATH_PATTERN}
	ANY_ABS_PATH         = &Path{pattern: ANY_ABS_PATH_PATTERN}
	ANY_REL_PATH         = &Path{pattern: ANY_REL_PATH_PATTERN}
	ANY_ABS_DIR_PATH     = &Path{pattern: ANY_ABS_DIR_PATH_PATTERN}
	ANY_ABS_NON_DIR_PATH = &Path{pattern: ANY_ABS_NON_DIR_PATH_PATTERN}
	ANY_REL_DIR_PATH     = &Path{pattern: ANY_REL_DIR_PATH_PATTERN}
	ANY_REL_NON_DIR_PATH = &Path{pattern: ANY_REL_NON_DIR_PATH_PATTERN}
	ANY_URL              = &URL{}
	ANY_SCHEME           = &Scheme{}
	ANY_HOST             = &Host{}
	ANY_PORT             = &Port{}

	PATH_PROPNAMES = []string{"segments", "extension", "name", "dir", "ends_with_slash", "rel_equiv", "change_extension", "join"}
)

// A Path represents a symbolic Path.
type Path struct {
	hasValue bool
	value    string

	pattern *PathPattern

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

func NewPath(v string) *Path {
	if v == "" {
		panic(errors.New("string should not be empty"))
	}

	return &Path{
		hasValue: true,
		value:    v,
	}
}

func NewPathMatchingPattern(p *PathPattern) *Path {
	return &Path{
		pattern: p,
	}
}

func (p *Path) Test(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherPath, ok := v.(*Path)
	if !ok {
		return false
	}

	if p.pattern != nil {
		return p.pattern.TestValue(v, state)
	}

	if !p.hasValue {
		return true
	}
	return otherPath.hasValue && p.value == otherPath.value
}

func (p *Path) IsConcretizable() bool {
	return p.hasValue
}

func (p *Path) Concretize(ctx ConcreteContext) any {
	if !p.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return extData.ConcreteValueFactories.CreatePath(p.value)
}

func (p *Path) Static() Pattern {
	return ANY_PATH_PATTERN
}

func (p *Path) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if p.hasValue {
		utils.Must(w.Write(utils.StringAsBytes(p.value)))
		return
	}

	if p.pattern != nil {

		s := ""
		switch {
		case p.pattern.absoluteness == UnspecifiedPathAbsoluteness && p.pattern.dirConstraint == UnspecifiedDirOrFilePath:
			s = "%path"
		case p.pattern.absoluteness == UnspecifiedPathAbsoluteness && p.pattern.dirConstraint == DirPath:
			s = "%dir-path"
		case p.pattern.absoluteness == UnspecifiedPathAbsoluteness && p.pattern.dirConstraint == NonDirPath:
			s = "%non-dir-path"
		case p.pattern.absoluteness == AbsolutePath && p.pattern.dirConstraint == UnspecifiedDirOrFilePath:
			s = "%absolute-path"
		case p.pattern.absoluteness == AbsolutePath && p.pattern.dirConstraint == DirPath:
			s = "%absolute-dir-path"
		case p.pattern.absoluteness == AbsolutePath && p.pattern.dirConstraint == NonDirPath:
			s = "%absolute-non-dir-path"
		case p.pattern.absoluteness == RelativePath && p.pattern.dirConstraint == UnspecifiedDirOrFilePath:
			s = "%relative-path"
		case p.pattern.absoluteness == RelativePath && p.pattern.dirConstraint == DirPath:
			s = "%relative-dir-path"
		case p.pattern.absoluteness == RelativePath && p.pattern.dirConstraint == NonDirPath:
			s = "%relative-non-dir-path"
		}

		if s != "" {
			utils.Must(w.Write(utils.StringAsBytes(s)))
			return
		}

		utils.Must(w.Write(utils.StringAsBytes("%patch(matching ")))

		if p.pattern.node != nil {
			utils.Must(w.Write(utils.StringAsBytes(p.pattern.stringifiedNode)))
		} else {
			p.pattern.PrettyPrint(w, config, depth, 0)
		}

		utils.PanicIfErr(w.WriteByte(')'))
	} else {
		utils.Must(w.Write(utils.StringAsBytes("%path")))
	}
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
		return NewListOf(ANY_STR)
	case "extension":
		return ANY_STR
	case "name":
		return ANY_STR
	case "dir":
		if p.pattern != nil {
			switch p.pattern.absoluteness {
			case AbsolutePath:
				return ANY_ABS_DIR_PATH
			case RelativePath:
				return ANY_REL_DIR_PATH
			}
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

func (p *Path) underlyingString() *String {
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
	hasValue bool
	value    string

	pattern *URLPattern

	UnassignablePropsMixin
	SerializableMixin
}

func NewUrl(v string) *URL {
	if v == "" {
		panic(errors.New("string should not be empty"))
	}
	return &URL{
		hasValue: true,
		value:    v,
	}
}

func NewUrlMatchingPattern(p *URLPattern) *URL {
	return &URL{
		pattern: p,
	}
}

func (u *URL) Test(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherURL, ok := v.(*URL)
	if !ok {
		return false
	}

	if u.pattern != nil {
		return u.pattern.TestValue(v, state)
	}

	if !u.hasValue {
		return true
	}

	return otherURL.hasValue && u.value == otherURL.value
}

func (u *URL) IsConcretizable() bool {
	return u.hasValue
}

func (u *URL) Concretize(ctx ConcreteContext) any {
	if !u.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return extData.ConcreteValueFactories.CreateURL(u.value)
}

func (u *URL) Static() Pattern {
	return ANY_URL_PATTERN
}

func (u *URL) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if u.hasValue {
		utils.Must(w.Write(utils.StringAsBytes(u.value)))
		return
	}

	utils.Must(w.Write(utils.StringAsBytes("%url")))

	if u.pattern != nil {
		utils.Must(w.Write(utils.StringAsBytes("(matching ")))

		if u.pattern.node != nil {
			utils.Must(w.Write(utils.StringAsBytes(u.pattern.stringifiedNode)))
		} else {
			u.pattern.PrettyPrint(w, config, depth, 0)
		}

		utils.PanicIfErr(w.WriteByte(')'))
	}
}

func (u *URL) underlyingString() *String {
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
	hasValue bool
	value    string
	SerializableMixin
}

func NewScheme(v string) *Scheme {
	if v == "" {
		panic(errors.New("string should not be empty"))
	}
	return &Scheme{
		hasValue: true,
		value:    v,
	}
}

func (s *Scheme) Test(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherScheme, ok := v.(*Scheme)
	if !ok {
		return false
	}
	if !s.hasValue {
		return true
	}
	return otherScheme.hasValue && s.value == otherScheme.value
}

func (s *Scheme) IsConcretizable() bool {
	return s.hasValue
}

func (s *Scheme) Concretize(ctx ConcreteContext) any {
	if !s.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return extData.ConcreteValueFactories.CreateScheme(s.value)
}

func (s *Scheme) Static() Pattern {
	return &TypePattern{val: ANY_SCHEME}
}

func (s *Scheme) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if s.hasValue {
		utils.Must(w.Write(utils.StringAsBytes(s.value)))
	} else {
		utils.Must(w.Write(utils.StringAsBytes("%scheme")))
	}
}

func (s *Scheme) underlyingString() *String {
	return ANY_STR
}

func (s *Scheme) WidestOfType() SymbolicValue {
	return ANY_SCHEME
}

// A Host represents a symbolic Host.
type Host struct {
	hasValue bool
	value    string

	pattern *HostPattern

	UnassignablePropsMixin
	SerializableMixin
}

func NewHost(v string) *Host {
	if v == "" {
		panic(errors.New("string should not be empty"))
	}
	return &Host{
		hasValue: true,
		value:    v,
	}
}

func NewHostMatchingPattern(p *HostPattern) *Host {
	return &Host{
		pattern: p,
	}
}

func (h *Host) Test(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherHost, ok := v.(*Host)
	if !ok {
		return false
	}

	if h.pattern != nil {
		return h.pattern.TestValue(v, state)
	}

	if !h.hasValue {
		return true
	}

	return otherHost.hasValue && h.value == otherHost.value
}

func (h *Host) IsConcretizable() bool {
	return h.hasValue
}

func (h *Host) Concretize(ctx ConcreteContext) any {
	if !h.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return extData.ConcreteValueFactories.CreateHost(h.value)
}

func (h *Host) Static() Pattern {
	return ANY_HOST_PATTERN
}

func (h *Host) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if h.hasValue {
		utils.Must(w.Write(utils.StringAsBytes(h.value)))
		return
	}

	utils.Must(w.Write(utils.StringAsBytes("%host")))

	if h.pattern != nil {
		utils.Must(w.Write(utils.StringAsBytes("(matching ")))

		if h.pattern.node != nil {
			utils.Must(w.Write(utils.StringAsBytes(h.pattern.stringifiedNode)))
		} else {
			h.pattern.PrettyPrint(w, config, depth, 0)
		}

		utils.PanicIfErr(w.WriteByte(')'))
	}
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
		return ANY_HOST
	default:
		return nil
	}
}

func (h *Host) underlyingString() *String {
	return ANY_STR
}

func (h *Host) WidestOfType() SymbolicValue {
	return ANY_HOST
}
