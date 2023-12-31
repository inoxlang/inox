package core

import (
	"strings"

	"github.com/inoxlang/inox/internal/utils"
)

type MigrationOpHandlers struct {
	Deletions       map[PathPattern]*MigrationOpHandler //handler can be nil
	Inclusions      map[PathPattern]*MigrationOpHandler
	Replacements    map[PathPattern]*MigrationOpHandler
	Initializations map[PathPattern]*MigrationOpHandler
}

func (handlers MigrationOpHandlers) FilterTopLevel() MigrationOpHandlers {
	filtered := MigrationOpHandlers{}

	for pattern, handler := range handlers.Deletions {
		if strings.Count(string(pattern), "/") > 1 {
			continue
		}
		if filtered.Deletions == nil {
			filtered.Deletions = map[PathPattern]*MigrationOpHandler{}
		}
		filtered.Deletions[pattern] = handler
	}

	for pattern, handler := range handlers.Inclusions {
		if strings.Count(string(pattern), "/") > 1 {
			continue
		}
		if filtered.Inclusions == nil {
			filtered.Inclusions = map[PathPattern]*MigrationOpHandler{}
		}
		filtered.Inclusions[pattern] = handler
	}

	for pattern, handler := range handlers.Replacements {
		if strings.Count(string(pattern), "/") > 1 {
			continue
		}
		if filtered.Replacements == nil {
			filtered.Replacements = map[PathPattern]*MigrationOpHandler{}
		}
		filtered.Replacements[pattern] = handler
	}

	for pattern, handler := range handlers.Initializations {
		if strings.Count(string(pattern), "/") > 1 {
			continue
		}
		if filtered.Initializations == nil {
			filtered.Initializations = map[PathPattern]*MigrationOpHandler{}
		}
		filtered.Initializations[pattern] = handler
	}

	return filtered
}

func (handlers MigrationOpHandlers) FilterByPrefix(path Path) MigrationOpHandlers {
	filtered := MigrationOpHandlers{}

	prefix := string(path)
	prefixSlash := string(prefix)
	prefixNoSlash := string(prefix)

	if prefixSlash[len(prefixSlash)-1] != '/' {
		prefixSlash += "/"
	} else if prefixNoSlash != "/" {
		prefixNoSlash = prefixNoSlash[:len(prefixNoSlash)-1]
	}

	// if prefix is /users:
	// /users will match
	// /users/x will match
	// /users-x will not match
	matchedBy := func(pattern PathPattern) bool {
		if pattern.IsPrefixPattern() {
			panic(ErrUnreachable)
		}

		patternString := string(pattern)

		prefixPattern := patternString
		//remove trailing slash
		if prefixPattern != "/" && prefixPattern[len(prefixPattern)-1] == '/' {
			prefixPattern = prefixPattern[:len(prefixPattern)-1]
		}

		slashCount := strings.Count(prefixNoSlash, "/")
		patternSlashCount := strings.Count(prefixPattern, "/")

		if patternSlashCount < slashCount {
			return false
		}

		for i := 0; i < patternSlashCount-slashCount; i++ {
			index := strings.LastIndex(prefixPattern, "/")
			prefixPattern = prefixPattern[:index]
		}

		if prefixNoSlash == prefixPattern || strings.HasPrefix(prefixPattern, prefixSlash) {
			return true
		}
		return PathPattern(prefixPattern).Test(nil, path)
	}

	for pattern, handler := range handlers.Deletions {

		if matchedBy(pattern) {
			if filtered.Deletions == nil {
				filtered.Deletions = map[PathPattern]*MigrationOpHandler{}
			}
			filtered.Deletions[pattern] = handler
		}
	}

	for pattern, handler := range handlers.Inclusions {
		if matchedBy(pattern) {
			if filtered.Inclusions == nil {
				filtered.Inclusions = map[PathPattern]*MigrationOpHandler{}
			}
			filtered.Inclusions[pattern] = handler
		}
	}

	for pattern, handler := range handlers.Replacements {
		if matchedBy(pattern) {
			if filtered.Replacements == nil {
				filtered.Replacements = map[PathPattern]*MigrationOpHandler{}
			}
			filtered.Replacements[pattern] = handler
		}
	}

	for pattern, handler := range handlers.Initializations {
		if matchedBy(pattern) {
			if filtered.Initializations == nil {
				filtered.Initializations = map[PathPattern]*MigrationOpHandler{}
			}
			filtered.Initializations[pattern] = handler
		}
	}

	return filtered
}

type MigrationOpHandler struct {
	//ignored if InitialValue is set
	Function     *InoxFunction
	InitialValue Serializable
}

func (h MigrationOpHandler) GetResult(ctx *Context, state *GlobalState) Value {
	if h.Function != nil {
		return utils.Must(h.Function.Call(state, nil, []Value{}, nil))
	} else {
		return utils.Must(RepresentationBasedClone(ctx, h.InitialValue))
	}
}
