package internal

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/inoxlang/inox/internal/afs"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	cmap "github.com/orcaman/concurrent-map/v2"
)

var (
	ErrCannotReleaseUnregisteredResource   = errors.New("cannot release unregistered resource")
	ErrFailedToAcquireResurce              = errors.New("failed to acquire resource")
	ErrResourceHasHardcodedUrlMetaProperty = errors.New("resource has hardcoded _url_ metaproperty")
	ErrInvalidResourceContent              = errors.New("invalid resource's content")
	ErrContentTypeParserNotFound           = errors.New("parser not found for content type")

	//resourceMap is a global identity map for resources
	resourceMap _resourceMap

	PATH_PROPNAMES         = []string{"segments", "extension", "name", "dir", "ends_with_slash", "rel_equiv", "change_extension", "join"}
	HOST_PROPNAMES         = []string{"scheme", "explicit_port", "without_port"}
	HOST_PATTERN_PROPNAMES = []string{"scheme"}
	URL_PROPNAMES          = []string{"scheme", "host", "path", "raw_query"}
	EMAIL_ADDR_PROPNAMES   = []string{"username", "domain"}
)

func init() {
	ResetResourceMap()
}

type _resourceMap struct {
	lock sync.Mutex
	map_ cmap.ConcurrentMap[string, *resourceInfo]
}

func AcquireResource(r ResourceName) {
	info, ok := resourceMap.map_.Get(r.ResourceName())

	if !ok { //we create an entry for the resource
		if resourceMap.lock.TryLock() {
			info = &resourceInfo{}
			resourceMap.map_.Set(r.ResourceName(), info)
			resourceMap.lock.Unlock()
		} else {
			AcquireResource(r)
		}
	}

	info.lock.Lock()
}

func TryAcquireResource(r ResourceName) bool {
	info, ok := resourceMap.map_.Get(r.ResourceName())

	if !ok { //we create an entry for the resource
		if resourceMap.lock.TryLock() {
			info = &resourceInfo{}
			resourceMap.map_.Set(r.ResourceName(), info)
			resourceMap.lock.Unlock()
		} else {
			return TryAcquireResource(r)
		}
	}

	if !info.lock.TryLock() {
		return false
	}
	return true
}

func TryReleaseResource(r ResourceName) {
	info, ok := resourceMap.map_.Get(r.ResourceName())

	if !ok {
		return
	}

	info.lock.Unlock()
}

func ReleaseResource(r ResourceName) {
	name := r.ResourceName()
	info, ok := resourceMap.map_.Get(name)

	if !ok {
		panic(fmt.Errorf("%w: %s", ErrCannotReleaseUnregisteredResource, name))
	}

	info.lock.Unlock()
}

func ResetResourceMap() {
	resourceMap = _resourceMap{
		map_: cmap.New[*resourceInfo](),
	}
}

type resourceInfo struct {
	lock sync.Mutex
}

type Path string

// NewPath creates a Path in a secure way.
func NewPath(slices []Value, isStaticPathSliceList []bool) (Value, error) {

	pth := ""

	for i, pathSlice := range slices {
		isStaticPathSlice := isStaticPathSliceList[i]

		switch slice := pathSlice.(type) {
		case Str:
			str := string(slice)

			if !isStaticPathSlice && !checkPathInterpolationResult(str) {
				return nil, errors.New("path expression: error: " + S_PATH_INTERP_RESULT_LIMITATION)
			}

			pth += str
		case Path:
			str := string(slice)
			if str[0] == '/' {
				str = "./" + str
			}

			if !isStaticPathSlice && !checkPathInterpolationResult(str) {
				return nil, errors.New("path expression: error: " + S_PATH_INTERP_RESULT_LIMITATION)
			}

			pth = path.Join(pth, str)
		default:
			return nil, fmt.Errorf("path expression: path slices should have a string value, not %T", slice)
		}
	}

	if strings.Contains(pth, "..") {
		return nil, errors.New("path expression: error: " + S_PATH_EXPR_PATH_LIMITATION)
	}

	if !parse.HasPathLikeStart(pth) {
		pth = "./" + pth
	}

	if len(pth) >= 2 {
		if pth[0] == '/' && pth[1] == '/' {
			pth = pth[1:]
		}
	}

	return Path(pth), nil
}

func checkPathInterpolationResult(s string) bool {
	for i, b := range utils.StringAsBytes(s) {
		switch b {
		case '.':
			if i < len(s)-1 && s[i+1] == '.' {
				return false
			}
		case '\\', '?', '*':
			return false
		}
	}
	return true
}

func (pth Path) IsDirPath() bool {
	return pth[len(pth)-1] == '/'
}

func (pth Path) IsAbsolute() bool {
	return pth[0] == '/'
}

func (pth Path) IsRelative() bool {
	return pth[0] == '.'
}

func (pth Path) ToAbs(fls afs.Filesystem) Path {
	if pth.IsAbsolute() {
		return pth
	}
	s, err := fls.Absolute(string(pth))
	if err != nil {
		panic(fmt.Errorf("path resolution: %s", err))
	}
	if pth.IsDirPath() && s[len(s)-1] != '/' {
		s += "/"
	}
	return Path(s)
}

func (pth Path) UnderlyingString() string {
	return string(pth)
}

func (pth Path) ResourceName() string {
	return string(pth)
}

func (pth Path) Extension() string {
	return filepath.Ext(string(pth))
}

func (pth Path) Basename() Str {
	return Str(filepath.Base(string(pth)))
}

func (pth Path) DirPath() Path {
	if pth == "/" {
		return "/"
	}

	s := string(pth)
	if pth.IsDirPath() {
		if s[len(s)-1] != '/' {
			panic(ErrInvalidDirPath)
		}
		s = s[:len(s)-1]
	} else {
		if s[len(s)-1] == '/' {
			panic(ErrInvalidNonDirPath)
		}
	}

	result := Path(s[:strings.LastIndexByte(s, '/')+1])
	return result
}

func (pth Path) RelativeEquiv() Path {
	if pth.IsRelative() {
		return pth
	}
	return "." + pth
}

func (pth Path) PropertyNames(ctx *Context) []string {
	return PATH_PROPNAMES
}

func (pth Path) Prop(ctx *Context, name string) Value {
	fls := ctx.GetFileSystem()

	switch name {
	case "segments":
		split := strings.Split(string(pth), "/")
		var segments ValueList

		for _, segment := range split {
			if segment != "" {
				segments.elements = append(segments.elements, Str(segment))
			}
		}
		return WrapUnderylingList(&segments)
	case "extension":
		return Str(pth.Extension())
	case "name":
		return pth.Basename()
	case "dir":
		return pth.DirPath()
	case "ends_with_slash":
		return Bool(pth.IsDirPath())
	case "rel_equiv":
		return pth.RelativeEquiv()
	case "change_extension":
		return WrapGoClosure(func(ctx *Context, newExt Str) Path {
			ext := pth.Extension()
			if ext == "" {
				return pth + Path(newExt)
			}
			withoutExt := string(pth[:len(pth)-len(ext)])

			if newExt == "" {
				return Path(withoutExt)
			}

			if newExt[0] != '.' {
				panic(errors.New("extension should start with '.' or be empty"))
			}

			return Path(withoutExt + string(newExt))
		})
	case "join":
		return WrapGoClosure(func(ctx *Context, relativePath Path) Path {
			if !relativePath.IsRelative() {
				panic(errors.New("path argument is not relative"))
			}
			dirpath := Path(fls.Join(string(pth), string(relativePath)))
			if relativePath.IsDirPath() && dirpath[len(dirpath)-1] != '/' {
				dirpath += "/"
			}
			if pth.IsRelative() {
				prefix, _, _ := strings.Cut(string(pth), "/")
				dirpath = Path(prefix) + "/" + dirpath
			}
			return dirpath
		})
	default:
		return nil
	}
}

func (Path) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func AppendTrailingSlashIfNotPresent[S ~string](s S) S {
	if s[len(s)-1] != '/' {
		return s + "/"
	}
	return s
}

type PathPattern string

// NewPathPattern creates a PathPattern in a secure way.
func NewPathPattern(slices []Value, isStaticPathSliceList []bool) (Value, error) {
	pth := ""

	for i, pathSlice := range slices {
		isStaticPathSlice := isStaticPathSliceList[i]

		switch s := pathSlice.(type) {
		case Str:
			str := string(s)
			if !isStaticPathSlice && (strings.Contains(str, "..") || strings.Contains(str, "*") || strings.Contains(str, "?") || strings.Contains(str, "[") ||
				strings.ContainsRune(str, '/') || strings.ContainsRune(str, '\\')) {
				return nil, errors.New("path pattern expression: error: result of an interpolation should not contain the substring '..', '*', '?', '[', '/' or '\\' ")
			}
			pth += str
		case Path:
			str := string(s)
			if str[0] == '/' {
				str = "./" + str
			}
			pth = path.Join(pth, str)
		default:
			return nil, fmt.Errorf("path pattern expression: path slices should have a Str or Path value, not a(n) %T", pathSlice)
		}
	}

	if strings.Contains(strings.TrimSuffix(pth, "/..."), "..") {
		return nil, errors.New("path pattern expression: error: result should not contain the substring '..' ")
	}

	if !parse.HasPathLikeStart(pth) {
		pth = "./" + pth
	}

	if len(pth) >= 2 {
		if pth[0] == '/' && pth[1] == '/' {
			pth = pth[1:]
		}
	}

	return PathPattern(pth), nil
}

func (patt PathPattern) IsAbsolute() bool {
	return patt[0] == '/'
}

func (patt PathPattern) IsGlobbingPattern() bool {
	return !patt.IsPrefixPattern()
}

func (patt PathPattern) IsDirGlobbingPattern() bool {
	return patt.IsGlobbingPattern() && patt[len(patt)-1] == '/'
}

func (patt PathPattern) IsPrefixPattern() bool {
	return strings.HasSuffix(string(patt), "/...")
}

func (patt PathPattern) Prefix() string {
	if patt.IsPrefixPattern() {
		return string(patt[0 : len(patt)-len("...")])
	}
	return string(patt)
}

func (patt PathPattern) ToAbs(fls afs.Filesystem) PathPattern {
	if patt.IsAbsolute() {
		return patt
	}
	s, err := fls.Absolute(string(patt))
	if err != nil {
		panic(fmt.Errorf("path pattern resolution: %s", err))
	}
	return PathPattern(s)
}

func (patt PathPattern) Test(ctx *Context, v Value) bool {
	switch other := v.(type) {
	case Path:
		if patt.IsPrefixPattern() {
			return strings.HasPrefix(string(other), patt.Prefix())
		}
		ok, err := path.Match(string(patt), string(other))
		return err == nil && ok
	case PathPattern:
		if patt.IsPrefixPattern() {
			return strings.HasPrefix(string(other), patt.Prefix())
		}
		return patt == other
	default:
		return false
	}
}

func (PathPattern) Call(values []Value) (Pattern, error) {
	return nil, ErrPatternNotCallable
}

func (patt PathPattern) StringPattern() (StringPattern, bool) {
	return &PathStringPattern{optionalPathPattern: patt}, true
}

func (patt PathPattern) UnderlyingString() string {
	return string(patt)
}

func (patt PathPattern) PropertyNames(ctx *Context) []string {
	return nil
}

func (patt PathPattern) Prop(ctx *Context, name string) Value {
	return nil
}

func (PathPattern) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

type URL string

// createPath creates an URL in a secure way.
func NewURL(host Value, pathSlices []Value, isStaticPathSliceList []bool, queryParamNames []Value, queryValues []Value) (Value, error) {

	//path evaluation

	var pth string
	for i, pathSlice := range pathSlices {
		isStaticPathSlice := isStaticPathSliceList[i]

		var str string

		switch s := pathSlice.(type) {
		case Str:
			str = string(s)
			pth += str
		case Path:
			str = string(s)
			if str[0] == '/' {
				str = "./" + str
			}
			pth = path.Join(pth, str)
		default:
			return nil, errors.New("URL expression: " + S_PATH_SLICE_VALUE_LIMITATION)
		}

		if !isStaticPathSlice && !checkPathInterpolationResult(str) {
			return nil, errors.New("URL expression: " + S_PATH_INTERP_RESULT_LIMITATION)
		}
	}

	//we check the final path

	if strings.Contains(pth, "..") {
		return nil, errors.New("URL expression: error: " + S_URL_EXPR_PATH_LIMITATION)
	}

	if pth != "" {
		if pth[0] == ':' {
			return nil, errors.New("URL expression: error: " + S_URL_EXPR_PATH_START_LIMITATION)
		}

		if pth[0] != '/' {
			pth = "/" + pth
		}
	}

	//query evaluation

	queryBuff := bytes.NewBufferString("")
	if len(queryValues) != 0 {
		queryBuff.WriteRune('?')
	}

	for i, paramValue := range queryValues {
		if i != 0 {
			queryBuff.WriteRune('&')
		}

		paramName := string(queryParamNames[i].(Str))
		queryBuff.WriteString(paramName)
		queryBuff.WriteRune('=')

		valueString := string(paramValue.(Str))
		if strings.ContainsAny(valueString, "&#") {
			return nil, errors.New("URL expression: error: " + S_QUERY_PARAM_VALUE_LIMITATION)
		}
		queryBuff.WriteString(valueString)
	}

	u := host.(Host).UnderlyingString() + string(pth) + queryBuff.String()
	if _, err := url.Parse(u); err != nil {
		return nil, errors.New("URL expression: " + err.Error())
	}

	return URL(u), nil
}

func (u URL) Scheme() Scheme {
	url, _ := url.Parse(string(u))
	return Scheme(url.Scheme)
}

func (u URL) Host() Host {
	url, _ := url.Parse(string(u))
	return Host(url.Scheme + "://" + url.Host)
}

func (u URL) Path() Path {
	url, _ := url.Parse(string(u))
	return Path(url.Path)
}

func (u URL) RawQuery() Str {
	url, _ := url.Parse(string(u))
	return Str(url.RawQuery)
}

func (u URL) UnderlyingString() string {
	return string(u)
}

func (u URL) ResourceName() string {
	return string(u)
}

func (u URL) WithScheme(scheme Scheme) URL {
	_, afterScheme, _ := strings.Cut(string(u), "://")
	return URL(scheme + "://" + Scheme(afterScheme))
}

func (u URL) WithoutQuery() URL {
	newURL, _, _ := strings.Cut(string(u), "?")
	return URL(newURL)
}

// AppendRelativePath joins a relative path with the URL's path if it has a directory path.
// If the input path is not relative or if the URL's path is not a directory path the function panics.
func (u URL) AppendRelativePath(relPath Path) URL {
	if !u.Path().IsDirPath() {
		panic(errors.New("relative paths can only be appended to a URL which path ends with /"))
	}
	if !relPath.IsRelative() {
		panic(errors.New("relative path expected"))
	}
	parsed, _ := url.Parse(string(u))
	return URL(strings.Replace(string(u), parsed.RawPath, parsed.RawPath+string(relPath[2:]), 1))
}

func (u URL) PropertyNames(ctx *Context) []string {
	return URL_PROPNAMES
}

func (u URL) Prop(ctx *Context, name string) Value {
	switch name {
	case "scheme":
		return Str(u.Scheme())
	case "host":
		return u.Host()
	case "path":
		return u.Path()
	case "raw_query":
		return u.RawQuery()
	default:
		return nil
	}
}

func (URL) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

// A Scheme represents an URL scheme, example: 'https'.
type Scheme string

func (s Scheme) UnderlyingString() string {
	return string(s)
}

// A Host is composed of the following parts: [<scheme>] '://' <hostname> [':' <port>].
type Host string

func NewHost(hostnamePort Value, scheme string) (Value, error) {
	host := scheme + "://" + string(hostnamePort.(Str))

	if parse.CheckHost(host) != nil {
		return nil, errors.New("host expression: invalid host")
	}

	return Host(host), nil
}

func (host Host) Scheme() Scheme {
	return Scheme(strings.Split(string(host), "://")[0])
}

// HasHttpScheme returns true if the scheme is "http" or "https"
func (host Host) HasHttpScheme() bool {
	scheme := host.Scheme()
	return scheme == "http" || scheme == "https"
}

func (host Host) HasScheme() bool {
	return host.Scheme() != ""
}

func (host Host) HostWithoutPort() Host {
	u, err := url.Parse(string(host))
	if err != nil {
		panic(err)
	}
	_, port, ok := strings.Cut(u.Host, ":")
	if !ok {
		return host
	}
	return Host(strings.Replace(string(host), ":"+port, "", 1))
}

func (host Host) WithoutScheme() string {
	return strings.Split(string(host), "://")[1]
}

func (host Host) ExplicitPort() int {
	index := strings.LastIndexByte(string(host), ':')
	if index > 0 && host[index+1] != '/' {
		port := string(host[index+1:])
		return utils.Must(strconv.Atoi(port))
	}
	return -1
}

func (host Host) UnderlyingString() string {
	return string(host)
}

func (host Host) ResourceName() string {
	return string(host)
}

func (host Host) PropertyNames(ctx *Context) []string {
	return HOST_PROPNAMES
}

func (host Host) Prop(ctx *Context, name string) Value {
	switch name {
	case "scheme":
		return Str(host.Scheme())
	case "explicit_port":
		return Int(host.ExplicitPort())
	case "without_port":
		return host.HostWithoutPort()
	default:
		return nil
	}
}

func (Host) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

type HostPattern string

func (patt HostPattern) UnderlyingString() string {
	return string(patt)
}

func (patt HostPattern) PropertyNames(ctx *Context) []string {
	return HOST_PATTERN_PROPNAMES
}

func (patt HostPattern) Prop(ctx *Context, name string) Value {
	switch name {
	case "scheme":
		return Str(patt.Scheme())
	default:
		return nil
	}
}

func (HostPattern) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

type EmailAddress string

func (addr EmailAddress) UnderlyingString() string {
	return string(addr)
}

func (addr EmailAddress) PropertyNames(ctx *Context) []string {
	return EMAIL_ADDR_PROPNAMES
}

func (addr EmailAddress) Prop(ctx *Context, name string) Value {
	switch name {
	case "username":
		return Str(strings.Split(string(addr), "@")[0])
	case "domain":
		domain := strings.Split(string(addr), "@")[1]
		return Host("://" + domain)
	default:
		return nil
	}
}

func (EmailAddress) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

type URLPattern string

func (patt URLPattern) UnderlyingString() string {
	return string(patt)
}

func (patt URLPattern) IsPrefixPattern() bool {
	return strings.HasSuffix(string(patt), "/...")
}

func (patt URLPattern) Prefix() string {
	return string(patt[0 : len(patt)-len("...")])
}

func (patt URLPattern) PropertyNames(ctx *Context) []string {
	return nil
}

func (patt URLPattern) Prop(ctx *Context, name string) Value {
	return nil
}

func (URLPattern) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (URLPattern) Call(values []Value) (Pattern, error) {
	return nil, ErrPatternNotCallable
}

func (URLPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (patt HostPattern) Scheme() Scheme {
	return Scheme(strings.Split(string(patt), "://")[0])
}

func (patt HostPattern) HasScheme() bool {
	return patt.Scheme() != ""
}

func (patt HostPattern) WithoutScheme() string {
	return strings.Split(string(patt), "://")[1]
}

func (patt HostPattern) Test(ctx *Context, v Value) bool {
	//TODO: cache built regex

	if !patt.HasScheme() {
		patt = NO_SCHEME_SCHEME_NAME + patt
	}
	var urlString string

	switch other := v.(type) {
	case HostPattern:
		return patt == other
	case Host:
		urlString = string(other)
	case URL:
		urlString = string(other)
	case URLPattern:
		urlString = string(other)
	}

	if urlString[0] == ':' { //no scheme
		urlString = NO_SCHEME_SCHEME_NAME + urlString
	}

	otherURL, err := url.Parse(urlString)
	if err != nil {
		return false
	}

	//we escape the dots so that they are properly matched
	regex := strings.ReplaceAll(string(patt), ".", "\\.")

	if patt.Scheme() == "https" {
		regex = strings.ReplaceAll(regex, ":443", "")
	} else if patt.Scheme() == "http" {
		regex = strings.ReplaceAll(regex, ":80", "")
	}

	regex = strings.ReplaceAll(regex, "/", "\\/")
	regex = strings.ReplaceAll(regex, "**", "[-a-zA-Z0-9.]{0,}")
	regex = "^" + strings.ReplaceAll(regex, "*", "[-a-zA-Z0-9]{0,}") + "$"

	host := otherURL.Scheme + "://" + otherURL.Host
	if otherURL.Scheme == "https" {
		host = strings.ReplaceAll(host, ":443", "")
	} else if otherURL.Scheme == "http" {
		host = strings.ReplaceAll(host, ":80", "")
	}

	ok, err := regexp.Match(regex, []byte(host))
	return err == nil && ok
}

func (HostPattern) Call(values []Value) (Pattern, error) {
	return nil, ErrPatternNotCallable
}

func (HostPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

func (patt HostPattern) includesPattern(otherPattern HostPattern) bool {
	if strings.Count(string(patt), "**") > 0 {
		patt := "^" + strings.ReplaceAll(string(patt), "**", "[0-9a-zA-Z*.-]+") + "$"
		regex := regexp.MustCompile(patt)
		return regex.MatchString(string(otherPattern))
	} else if strings.Count(string(otherPattern), "**") > 0 {
		return false
	}
	return patt == otherPattern
}

func (patt URLPattern) Test(ctx *Context, v Value) bool {
	switch other := v.(type) {
	case HostPattern, Host:
		return false
	case URL:
		queryIndex := strings.Index(string(other), "?")
		if queryIndex > 0 {
			other = other[:queryIndex]
		}

		return strings.HasPrefix(string(other), patt.Prefix())
	default:
		return false
	}
}

func ParseOrValidateResourceContent(ctx *Context, resourceContent []byte, ctype Mimetype, doParse, validateRaw bool) (res Value, contentType Mimetype, err error) {
	ct := ctype.WithoutParams()
	switch ct {
	case PLAIN_TEXT_CTYPE:
		res = Str(resourceContent)
	case "", APP_OCTET_STREAM_CTYPE:
		res = NewByteSlice(resourceContent, false, "")
	default:
		parser, ok := GetParser(ct)

		if doParse {
			if !ok {
				res = nil
				contentType = ""
				err = fmt.Errorf("%w (%s)", ErrContentTypeParserNotFound, ct)
				return
			}

			res, err = parser.Parse(ctx, utils.BytesAsString(resourceContent))
		} else if validateRaw {
			if !ok {
				res = nil
				contentType = ""
				err = fmt.Errorf("%w (%s)", ErrContentTypeParserNotFound, ct)
				return
			}

			if !parser.Validate(ctx, utils.BytesAsString(resourceContent)) {
				res = nil
				contentType = ""
				err = ErrInvalidResourceContent
				return
			}
			res = NewByteSlice(resourceContent, false, ct)
		} else {
			res = NewByteSlice(resourceContent, false, ct)
		}
	}
	return
}
