package core

// WORK IN PROGRESS !

var (
	SELF_SENSITIVE_DATA_NAMES = map[string]struct {
		patterns []Pattern
	}{
		"password": {
			patterns: []Pattern{STR_PATTERN, BYTESLICE_PATTERN},
		},
		"passwordHash": {
			patterns: []Pattern{STR_PATTERN, BYTESLICE_PATTERN},
		},
		"email": {
			patterns: []Pattern{STR_PATTERN},
		},
		"emailAddress": {
			patterns: []Pattern{STR_PATTERN},
		},
		"address": {
			patterns: []Pattern{STR_PATTERN},
		},
		"age":    {},
		"gender": {},

		"X-Api-Key": {},
	}
)

func init() {
	//TODO: add variations
}

func IsSensitiveProperty(ctx *Context, name string, value Value) bool {
	s, ok := SELF_SENSITIVE_DATA_NAMES[name]
	if !ok {
		return false
	}
	if len(s.patterns) == 0 {
		return true
	}
	for _, patt := range s.patterns {
		if patt.Test(ctx, value) {
			return true
		}
	}
	return false
}

func IsAtomSensitive(v Value) bool {
	switch v.(type) {
	case EmailAddress:
		return true
	case CheckedString:
		return false
	default:
		return false
	}
}
