package internal

var (
	extData = ExternalData{
		CONSTRAINTS_KEY: "_constraints_",
		VISIBILITY_KEY:  "_visibility_",
		SymbolicToPattern: func(v SymbolicValue) (any, bool) {
			//not a real pattern but it's okay
			return struct{}{}, true
		},
	} // default data for tests
)

type ExternalData struct {
	ToSymbolicValue       func(v any, wide bool) (SymbolicValue, error)
	SymbolicToPattern     func(v SymbolicValue) (any, bool)
	GetQuantity           func(values []float64, units []string) (any, error)
	ConvertKeyReprToValue func(string) any
	IsReadable            func(v any) bool
	IsWritable            func(v any) bool

	DEFAULT_PATTERN_NAMESPACES map[string]*PatternNamespace

	IMPLICIT_KEY_LEN_KEY string
	CONSTRAINTS_KEY      string
	VISIBILITY_KEY       string
}

func SetExternalData(data ExternalData) {
	extData = data
}
