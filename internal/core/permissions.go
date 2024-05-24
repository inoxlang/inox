package core

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/inoxlang/inox/internal/core/permbase"
)

var (
	ErrImpossibleToVerifyPermissionForUrlHolderMutation = errors.New("impossible to verify permission for mutation of URL holder")
)

type PermissionKind = permbase.PermissionKind
type Permission = permbase.Permission

type NotAllowedError struct {
	Permission Permission
	Message    string
}

func NewNotAllowedError(perm Permission) *NotAllowedError {
	return &NotAllowedError{
		Permission: perm,
		Message:    fmt.Sprintf("not allowed, missing permission: %s", perm.String()),
	}
}

func (err NotAllowedError) Error() string {
	return err.Message
}

func (err NotAllowedError) Is(target error) bool {
	notAllowedErr, ok := target.(*NotAllowedError)
	if !ok {
		return false
	}

	return err.Permission.Includes(notAllowedErr.Permission) && notAllowedErr.Permission.Includes(err.Permission) &&
		err.Message == notAllowedErr.Message
}

type GlobalVarPermission struct {
	Kind_ PermissionKind
	Name  string //"*" means any
}

func (perm GlobalVarPermission) Kind() PermissionKind {
	return perm.Kind_
}

func (perm GlobalVarPermission) InternalPermTypename() permbase.InternalPermissionTypename {
	return permbase.GLOBAL_VAR_PERM_TYPENAME
}

func (perm GlobalVarPermission) Includes(otherPerm Permission) bool {
	otherGlobVarPerm, ok := otherPerm.(GlobalVarPermission)
	if !ok || !perm.Kind().Includes(otherGlobVarPerm.Kind()) {
		return false
	}

	return perm.Name == "*" || perm.Name == otherGlobVarPerm.Name
}

func (perm GlobalVarPermission) String() string {
	return fmt.Sprintf("[%s global(s) '%s']", perm.Kind_, perm.Name)
}

//

type EnvVarPermission struct {
	Kind_ PermissionKind
	Name  string //"*" means any
}

func (perm EnvVarPermission) Kind() PermissionKind {
	return perm.Kind_
}

func (perm EnvVarPermission) InternalPermTypename() permbase.InternalPermissionTypename {
	return permbase.ENV_PERM_TYPENAME
}

func (perm EnvVarPermission) Includes(otherPerm Permission) bool {
	otherEnvVarPerm, ok := otherPerm.(EnvVarPermission)
	if !ok || !perm.Kind().Includes(otherEnvVarPerm.Kind()) {
		return false
	}

	return perm.Name == "*" || perm.Name == otherEnvVarPerm.Name
}

func (perm EnvVarPermission) String() string {
	return fmt.Sprintf("[%s env '%s']", perm.Kind_, perm.Name)
}

//

type LThreadPermission struct {
	Kind_ PermissionKind
}

func (perm LThreadPermission) Kind() PermissionKind {
	return perm.Kind_
}

func (perm LThreadPermission) InternalPermTypename() permbase.InternalPermissionTypename {
	return permbase.LTHREAD_PERM_TYPENAME
}

func (perm LThreadPermission) Includes(otherPerm Permission) bool {
	otherLThreadPerm, ok := otherPerm.(LThreadPermission)

	return ok && perm.Kind_.Includes(otherLThreadPerm.Kind_)
}

func (perm LThreadPermission) String() string {
	return fmt.Sprintf("[%s threads]", perm.Kind_)
}

type FilesystemPermission struct {
	Kind_  PermissionKind
	Entity GoString //Path, PathPattern ...
}

func CreateFsReadPerm(entity GoString) FilesystemPermission {
	return FilesystemPermission{Kind_: permbase.Read, Entity: entity}
}

func (perm FilesystemPermission) Kind() PermissionKind {
	return perm.Kind_
}

func (perm FilesystemPermission) InternalPermTypename() permbase.InternalPermissionTypename {
	return permbase.FS_PERM_TYPENAME
}

func (perm FilesystemPermission) Includes(otherPerm Permission) bool {
	otherFsPerm, ok := otherPerm.(FilesystemPermission)
	if !ok || !perm.Kind_.Includes(otherFsPerm.Kind_) {
		return false
	}

	switch e := perm.Entity.(type) {
	case Path:
		otherPath, ok := otherFsPerm.Entity.(Path)
		return ok && e == otherPath
	case PathPattern:
		return e.Includes(nil, otherFsPerm.Entity)
	}

	return false
}

func (perm FilesystemPermission) String() string {
	return fmt.Sprintf("[%s path(s) %s]", perm.Kind_, perm.Entity)
}

type CommandPermission struct {
	CommandName         GoString //string or Path or PathPattern
	SubcommandNameChain []string //can be empty
}

func (perm CommandPermission) InternalPermTypename() permbase.InternalPermissionTypename {
	return permbase.LTHREAD_PERM_TYPENAME
}

func (perm CommandPermission) Kind() PermissionKind {
	return permbase.Use
}

func (perm CommandPermission) Includes(otherPerm Permission) bool {

	otherCmdPerm, ok := otherPerm.(CommandPermission)
	if !ok || !perm.Kind().Includes(otherCmdPerm.Kind()) {
		return false
	}

	switch cmdName := perm.CommandName.(type) {
	case String:
		otherCommandName, ok := otherCmdPerm.CommandName.(String)
		if !ok || otherCommandName != cmdName {
			return false
		}
	case Path:
		otherCommandName, ok := otherCmdPerm.CommandName.(Path)
		if !ok || otherCommandName != cmdName {
			return false
		}
	case PathPattern:
		switch otherCmdPerm.CommandName.(type) {
		case Path, PathPattern:
			if !cmdName.Includes(nil, otherCmdPerm.CommandName) {
				return false
			}
		default:
			return false
		}
	default:
		return false
	}

	if len(otherCmdPerm.SubcommandNameChain) != len(perm.SubcommandNameChain) {
		return false
	}

	for i, name := range perm.SubcommandNameChain {
		if otherCmdPerm.SubcommandNameChain[i] != name {
			return false
		}
	}

	return true
}

func (perm CommandPermission) String() string {
	b := bytes.NewBufferString("[exec command:")
	b.WriteString(fmt.Sprint(perm.CommandName))

	if len(perm.SubcommandNameChain) == 0 {
		b.WriteString(" <no subcommand>")
	}

	for _, name := range perm.SubcommandNameChain {
		b.WriteString(" ")
		b.WriteString(name)
	}
	b.WriteString("]")

	return b.String()
}

type HttpPermission struct {
	Kind_     PermissionKind
	Entity    GoString //URL, URLPattern, HTTPHost, HTTPHostPattern ....
	AnyEntity bool
}

func CreateHttpReadPerm(entity GoString) HttpPermission {
	return HttpPermission{Kind_: permbase.Read, Entity: entity}
}

func (perm HttpPermission) Kind() PermissionKind {
	return perm.Kind_
}

func (perm HttpPermission) InternalPermTypename() permbase.InternalPermissionTypename {
	return permbase.HTTP_PERM_TYPENAME
}

func (perm HttpPermission) Includes(otherPerm Permission) bool {
	otherHttpPerm, ok := otherPerm.(HttpPermission)
	if !ok || !perm.Kind_.Includes(otherHttpPerm.Kind_) {
		return false
	}

	if perm.AnyEntity {
		return true
	}

	if otherHttpPerm.AnyEntity {
		return false
	}

	switch e := perm.Entity.(type) {
	case URL:
		otherURL, ok := otherHttpPerm.Entity.(URL)
		parsedURL, _ := url.Parse(string(e))

		if parsedURL.RawQuery == "" {
			parsedURL.ForceQuery = false

			otherParsedURL, _ := url.Parse(string(otherURL))
			otherParsedURL.RawQuery = ""
			otherParsedURL.ForceQuery = false

			return parsedURL.String() == otherParsedURL.String()
		}

		return ok && e == otherURL
	case URLPattern:
		otherURLPattern, ok := otherHttpPerm.Entity.(URLPattern)
		if ok && e.IsPrefixPattern() && otherURLPattern.IsPrefixPattern() &&
			strings.HasPrefix(strings.ReplaceAll(string(e), "/...", "/"), strings.ReplaceAll(string(otherURLPattern), "/...", "/")) {
			return true
		}

		return e.Includes(nil, otherHttpPerm.Entity)
	case Host:
		host := e.WithoutScheme()
		switch other := otherHttpPerm.Entity.(type) {
		case URL:
			otherURL, err := url.Parse(string(other))
			if err != nil {
				return false
			}
			return otherURL.Scheme == string(e.Scheme()) && otherURL.Host == host
		case URLPattern:
			otherURL, err := url.Parse(string(other))
			if err != nil {
				return false
			}
			return otherURL.Scheme == string(e.Scheme()) && otherURL.Host == host
		case Host:
			return e == other
		}
	case HostPattern:
		return e.Includes(nil, otherHttpPerm.Entity)
	}

	return false
}

func (perm HttpPermission) String() string {
	if perm.AnyEntity {
		return fmt.Sprintf("[%s https://**:*]", perm.Kind_)
	}
	return fmt.Sprintf("[%s %s]", perm.Kind_, perm.Entity)
}

type DatabasePermission struct {
	Kind_  PermissionKind
	Entity GoString
}

func (perm DatabasePermission) Kind() PermissionKind {
	return perm.Kind_
}

func (perm DatabasePermission) InternalPermTypename() permbase.InternalPermissionTypename {
	return permbase.FS_PERM_TYPENAME
}

func (perm DatabasePermission) Includes(otherPerm Permission) bool {
	otherDbPerm, ok := otherPerm.(DatabasePermission)
	if !ok || !perm.Kind_.Includes(otherDbPerm.Kind_) {
		return false
	}

	switch e := perm.Entity.(type) {
	case URL:
		otherURL, ok := otherDbPerm.Entity.(URL)
		parsedURL, _ := url.Parse(string(e))

		if parsedURL.RawQuery != "" || otherURL.RawQuery() != "" {
			panic(ErrUnreachable)
		}

		return ok && e == otherURL
	case URLPattern:
		return e.Includes(nil, otherDbPerm.Entity)
	case Host:
		host := e.WithoutScheme()
		switch other := otherDbPerm.Entity.(type) {
		case URL:
			otherURL, err := url.Parse(string(other))
			if err != nil {
				return false
			}
			return otherURL.Scheme == string(e.Scheme()) && otherURL.Host == host
		case URLPattern:
			otherURL, err := url.Parse(string(other))
			if err != nil {
				return false
			}
			return otherURL.Scheme == string(e.Scheme()) && otherURL.Host == host
		case Host:
			return e == other
		}
	case HostPattern:
		return e.Includes(nil, otherDbPerm.Entity)
	}

	return false
}

func (perm DatabasePermission) String() string {
	return fmt.Sprintf("[%s %s]", perm.Kind_, perm.Entity)
}

type WebsocketPermission struct {
	Kind_    PermissionKind
	Endpoint ResourceName //ignored for some permission kinds
}

func (perm WebsocketPermission) Kind() PermissionKind {
	return perm.Kind_
}

func (perm WebsocketPermission) InternalPermTypename() permbase.InternalPermissionTypename {
	return permbase.WEBSOCKET_PERM_TYPENAME
}

func (perm WebsocketPermission) String() string {
	if perm.Endpoint == nil {
		return fmt.Sprintf("[websocket %s]", perm.Kind_)
	}
	return fmt.Sprintf("[websocket %s %s]", perm.Kind_, perm.Endpoint)
}

func (perm WebsocketPermission) Includes(otherPerm Permission) bool {
	otherWsPerm, ok := otherPerm.(WebsocketPermission)
	if !ok || !perm.Kind_.Includes(otherWsPerm.Kind_) {
		return false
	}

	if perm.Kind_ == permbase.Provide {
		return true
	}

	if perm.Endpoint == otherWsPerm.Endpoint {
		return true
	}

	switch endpoint := perm.Endpoint.(type) {
	case Host:
		switch otherEndpoint := otherWsPerm.Endpoint.(type) {
		case URL:
			return otherEndpoint.Host() == endpoint
		}
	}

	return false
}

type DNSPermission struct {
	Kind_  PermissionKind
	Domain GoString //Host | HostPattern
}

func (perm DNSPermission) Kind() PermissionKind {
	return perm.Kind_
}

func (perm DNSPermission) InternalPermTypename() permbase.InternalPermissionTypename {
	return permbase.DNS_PERM_TYPENAME
}

func (perm DNSPermission) String() string {
	return fmt.Sprintf("[dns %s %s]", perm.Kind_, perm.Domain)
}

func (perm DNSPermission) Includes(otherPerm Permission) bool {
	otherDnsPerm, ok := otherPerm.(DNSPermission)
	if !ok || !perm.Kind_.Includes(otherDnsPerm.Kind_) {
		return false
	}

	switch domain := perm.Domain.(type) {
	case HostPattern:
		switch otherDomain := otherDnsPerm.Domain.(type) {
		case Host:
			return domain.Test(nil, otherDomain)
		case HostPattern:
			return domain.includesPattern(otherDomain)
		}
	case Host:
		switch otherDomain := otherDnsPerm.Domain.(type) {
		case Host:
			return domain == otherDomain
		case HostPattern:
			return false
		}
	}

	return false

}

//----------------------------------------------------------------------

type RawTcpPermission struct {
	Kind_  PermissionKind
	Domain GoString //Host | HostPattern
}

func (perm RawTcpPermission) Kind() PermissionKind {
	return perm.Kind_
}

func (perm RawTcpPermission) InternalPermTypename() permbase.InternalPermissionTypename {
	return permbase.TCP_PERM_TYPENAME
}

func (perm RawTcpPermission) String() string {
	return fmt.Sprintf("[tcp %s %s]", perm.Kind_, perm.Domain)
}

func (perm RawTcpPermission) Includes(otherPerm Permission) bool {
	otherTcpPerm, ok := otherPerm.(RawTcpPermission)
	if !ok || !perm.Kind().Includes(otherTcpPerm.Kind_) {
		return false
	}

	switch domain := perm.Domain.(type) {
	case HostPattern:
		switch otherDomain := otherTcpPerm.Domain.(type) {
		case Host:
			return domain.Includes(nil, otherDomain)
		case HostPattern:
			return domain.includesPattern(otherDomain)
		}
	case Host:
		switch otherDomain := otherTcpPerm.Domain.(type) {
		case Host:
			return domain == otherDomain
		case HostPattern:
			return false
		}
	}

	return false

}

type ValueVisibilityPermission struct {
	Pattern Pattern
}

func (perm ValueVisibilityPermission) InternalPermTypename() permbase.InternalPermissionTypename {
	return permbase.VALUE_VISIBILITY_PERM_TYPENAME
}

func (perm ValueVisibilityPermission) Kind() PermissionKind {
	return permbase.See
}

func (perm ValueVisibilityPermission) String() string {
	return fmt.Sprintf("[see value matching %s]", perm.Pattern)
}

func (perm ValueVisibilityPermission) Includes(otherPerm Permission) bool {
	otherVisibilityPerm, ok := otherPerm.(ValueVisibilityPermission)
	if !ok || !perm.Kind().Includes(otherPerm.Kind()) {
		return false
	}

	//TODO: support all patterns

	if exact, ok := otherVisibilityPerm.Pattern.(*ExactValuePattern); ok {
		return perm.Pattern.Test(nil, exact.value)
	}

	return perm.Pattern.Equal(nil, otherVisibilityPerm.Pattern, map[uintptr]uintptr{}, 0)
}

type SystemGraphAccessPermission struct {
	Kind_ PermissionKind
}

func (perm SystemGraphAccessPermission) InternalPermTypename() permbase.InternalPermissionTypename {
	return permbase.SYSGRAPH_PERM_TYPENAME
}

func (perm SystemGraphAccessPermission) Kind() PermissionKind {
	return perm.Kind_
}

func (perm SystemGraphAccessPermission) String() string {
	return fmt.Sprintf("[%s system graph]", perm.Kind_.String())
}

func (perm SystemGraphAccessPermission) Includes(otherPerm Permission) bool {
	otherSysGraphPerm, ok := otherPerm.(SystemGraphAccessPermission)
	return ok && perm.Kind_.Includes(otherSysGraphPerm.Kind_)
}

func RemovePerms(grantedPerms, removedPerms []Permission) (remainingPerms []Permission) {
top:
	for _, perm := range grantedPerms {
		for _, removedPerm := range removedPerms {
			if removedPerm.Includes(perm) {
				continue top
			}
		}

		remainingPerms = append(remainingPerms, perm)
	}

	return
}
