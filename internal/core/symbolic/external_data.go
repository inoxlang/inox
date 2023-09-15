package symbolic

import (
	"context"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	extData = ExternalData{
		CONSTRAINTS_KEY:                         "_constraints_",
		VISIBILITY_KEY:                          "_visibility_",
		MANIFEST_PARAMS_SECTION_NAME:            "parameters",
		MOD_ARGS_VARNAME:                        "mod-args",
		MANIFEST_POSITIONAL_PARAM_NAME_FIELD:    "name",
		MANIFEST_POSITIONAL_PARAM_PATTERN_FIELD: "pattern",
		SymbolicToPattern: func(v SymbolicValue) (any, bool) {
			//not a real pattern but it's okay
			return struct{}{}, true
		},
		IsIndexKey: func(key string) bool {
			//TODO: number of implicit keys will be soon limited so this function should be refactored to only check for integers
			// with a small number of digits.
			_, err := strconv.ParseUint(key, 10, 32)
			return err == nil
		},
		PathMatch: func(pth, pattern string) bool {
			if strings.HasSuffix(pattern, "/...") {
				return strings.HasPrefix(pth, pattern[:len(pattern)-len("...")])
			}
			ok, err := path.Match(pattern, pth)
			return err == nil && ok
		},
		URLMatch: func(url, pattern string) bool {
			return strings.HasPrefix(url, pattern[:len(pattern)-len("...")])
		},
		HostMatch: func(host, pattern string) bool {
			regex := "^" + strings.ReplaceAll(pattern, "*", "[-a-zA-Z0-9]{0,}") + "$"

			return utils.Must(regexp.Match(regex, []byte(host)))
		},
	} // default data for tests
)

type ExternalData struct {
	ToSymbolicValue                        func(v any, wide bool) (SymbolicValue, error)
	SymbolicToPattern                      func(v SymbolicValue) (any, bool)
	GetQuantity                            func(values []float64, units []string) (any, error)
	GetRate                                func(values []float64, units []string, divUnit string) (any, error)
	ConvertKeyReprToValue                  func(string) any
	IsReadable                             func(v any) bool
	IsWritable                             func(v any) bool
	IsIndexKey                             func(k string) bool
	PathMatch                              func(path, pattern string) bool
	URLMatch                               func(url, pattern string) bool
	HostMatch                              func(host, pattern string) bool
	CheckDatabaseSchema                    func(objectPattern any) error
	GetTopLevelEntitiesMigrationOperations func(concreteCtx context.Context, current, next any) ([]MigrationOp, error)

	ConcreteValueFactories ConcreteValueFactories

	DEFAULT_PATTERN_NAMESPACES map[string]*PatternNamespace

	CONSTRAINTS_KEY                         string
	VISIBILITY_KEY                          string
	MANIFEST_PARAMS_SECTION_NAME            string
	MANIFEST_POSITIONAL_PARAM_NAME_FIELD    string
	MANIFEST_POSITIONAL_PARAM_PATTERN_FIELD string
	MOD_ARGS_VARNAME                        string
}

func SetExternalData(data ExternalData) {
	extData = data
}
