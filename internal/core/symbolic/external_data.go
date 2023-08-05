package symbolic

import "strconv"

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
	} // default data for tests
)

type ExternalData struct {
	ToSymbolicValue       func(v any, wide bool) (SymbolicValue, error)
	SymbolicToPattern     func(v SymbolicValue) (any, bool)
	GetQuantity           func(values []float64, units []string) (any, error)
	GetRate               func(values []float64, units []string, divUnit string) (any, error)
	ConvertKeyReprToValue func(string) any
	IsReadable            func(v any) bool
	IsWritable            func(v any) bool
	IsIndexKey            func(k string) bool

	DEFAULT_PATTERN_NAMESPACES map[string]*PatternNamespace

	IMPLICIT_KEY_LEN_KEY                    string
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
