package symbolic

import (
	"errors"
	"net/url"

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

	HTTP_SCHEME  = NewScheme("http")
	HTTPS_SCHEME = NewScheme("https")
	WS_SCHEME    = NewScheme("ws")
	WSS_SCHEME   = NewScheme("wss")

	ANY_HTTP_HOST  = NewHostMatchingPattern(ANY_HTTP_HOST_PATTERN)
	ANY_HTTPS_HOST = NewHostMatchingPattern(ANY_HTTPS_HOST_PATTERN)
	ANY_WS_HOST    = NewHostMatchingPattern(ANY_WS_HOST_PATTERN)
	ANY_WSS_HOST   = NewHostMatchingPattern(ANY_WSS_HOST_PATTERN)

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

func (p *Path) Test(v Value, state RecTestCallState) bool {
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

func (p *Path) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if p.hasValue {
		w.WriteString(p.value)
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
			w.WriteString(s)
			return
		}

		w.WriteName("patch(matching ")

		if p.pattern.node != nil {
			w.WriteString(p.pattern.stringifiedNode)
		} else {
			p.pattern.PrettyPrint(w.ZeroIndent(), config)
		}

		w.WriteByte(')')
	} else {
		w.WriteName("path")
	}
}

func (p *Path) ResourceName() *String {
	return ANY_STR
}

func (p *Path) PropertyNames() []string {
	return PATH_PROPNAMES
}

func (p *Path) Prop(name string) Value {
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

func (p *Path) WalkerElement() Value {
	return WALK_ELEM
}

func (p *Path) WalkerNodeMeta() Value {
	return ANY
}

func (p *Path) WidestOfType() Value {
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

func (u *URL) Test(v Value, state RecTestCallState) bool {
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

func (u *URL) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if u.hasValue {
		w.WriteString(u.value)
		return
	}

	w.WriteName("url")

	if u.pattern != nil {
		w.WriteString("(matching ")

		if u.pattern.node != nil {
			w.WriteString(u.pattern.stringifiedNode)
		} else {
			u.pattern.PrettyPrint(w.ZeroIndent(), config)
		}

		w.WriteByte(')')
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

func (u *URL) Prop(name string) Value {
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

func (u *URL) WidestOfType() Value {
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

func GetOrNewScheme(v string) *Scheme {
	if v == "" {
		panic(errors.New("string should not be empty"))
	}
	switch v {
	case "http":
		return HTTP_SCHEME
	case "https":
		return HTTPS_SCHEME
	case "ws":
		return WS_SCHEME
	case "wss":
		return WSS_SCHEME
	}
	return NewScheme(v)
}

func (s *Scheme) Test(v Value, state RecTestCallState) bool {
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

func (s *Scheme) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if s.hasValue {
		w.WriteString(s.value)
	} else {
		w.WriteName("scheme")
	}
}

func (s *Scheme) underlyingString() *String {
	return ANY_STR
}

func (s *Scheme) WidestOfType() Value {
	return ANY_SCHEME
}

// A Host represents a symbolic Host.
type Host struct {
	hasValue bool
	value    string // ://example.com, https://example.com, ...

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

func (h *Host) Scheme() (*Scheme, bool) {
	if h.hasValue {
		if h.value[0] == ':' { //scheme-less host
			return nil, false
		}
		u := utils.Must(url.Parse(h.value))
		return GetOrNewScheme(u.Scheme), true
	}

	if h.pattern != nil {
		return h.pattern.Scheme()
	}
	return nil, false
}

func (h *Host) Test(v Value, state RecTestCallState) bool {
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

func (h *Host) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if h.hasValue {
		w.WriteString(h.value)
		return
	}

	w.WriteName("host")

	if h.pattern != nil {
		if h.pattern.node != nil {
			w.WriteString("(matching ")
			w.WriteString(h.pattern.stringifiedNode)
			w.WriteByte(')')
		} else if h.pattern.scheme != nil && h.pattern.scheme.hasValue {
			w.WriteString("(")
			w.WriteString(h.pattern.scheme.value)
			w.WriteString(")")
			return
		}
		w.WriteString("(?)")
	}
}

func (h *Host) ResourceName() *String {
	return ANY_STR
}

func (s *Host) PropertyNames() []string {
	return []string{"scheme", "explicit_port", "without_port"}
}

func (*Host) Prop(name string) Value {
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

func (h *Host) WidestOfType() Value {
	return ANY_HOST
}
