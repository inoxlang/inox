package symbolic

import "github.com/inoxlang/inox/internal/utils"

var (
	INEXACT_OBJECT_WITH_A_ONE          = NewInexactObject(map[string]Serializable{"a": NewInt(1)}, nil, nil)
	READONLY_INEXACT_OBJECT_WITH_A_ONE = INEXACT_OBJECT_WITH_A_ONE.ReadonlyObject()

	ABS_DIR_PATH_EXAMPLE1 = NewPath("/dir/")
	ABS_DIR_PATH_EXAMPLE2 = NewPath("/dir/subdir/")

	REL_DIR_PATH_EXAMPLE1 = NewPath("./dir/")
	REL_DIR_PATH_EXAMPLE2 = NewPath("./dir/subdir/")

	ABS_FILE_PATH_EXAMPLE1 = NewPath("/file.json")
	ABS_FILE_PATH_EXAMPLE2 = NewPath("/dir/file.json")

	REL_FILE_PATH_EXAMPLE1 = NewPath("./file.json")
	REL_FILE_PATH_EXAMPLE2 = NewPath("./dir/file.json")

	PATH_EXAMPLES = []*Path{
		ABS_DIR_PATH_EXAMPLE1,
		ABS_DIR_PATH_EXAMPLE2,
		REL_DIR_PATH_EXAMPLE1,
		REL_DIR_PATH_EXAMPLE2,

		ABS_FILE_PATH_EXAMPLE1,
		ABS_FILE_PATH_EXAMPLE2,

		REL_FILE_PATH_EXAMPLE1,
		REL_FILE_PATH_EXAMPLE2,
	}

	_ = []MatchingValueExampleProvider{
		(*Object)(nil),
		(*Path)(nil),
	}
)

type MatchingValueExampleProvider interface {
	SymbolicValue

	Examples(cctx ExampleComputationContext) []MatchingValueExample
}

type ExampleComputationContext struct {
	NonMatchingValue SymbolicValue
}

type MatchingValueExample struct {
	Value             SymbolicValue
	AdditionalMessage string
}

func GetExamples(v SymbolicValue, cctx ExampleComputationContext) []MatchingValueExample {
	if IsConcretizable(v) {
		return []MatchingValueExample{{Value: v}}
	}

	if exampleProvider, ok := v.(MatchingValueExampleProvider); ok {
		return exampleProvider.Examples(cctx)
	}
	return nil
}

func (o *Object) Examples(cctx ExampleComputationContext) []MatchingValueExample {
	if o.entries == nil {
		if o.readonly {
			return []MatchingValueExample{{Value: EMPTY_OBJECT}, {Value: READONLY_INEXACT_OBJECT_WITH_A_ONE}}
		}
		return []MatchingValueExample{{Value: EMPTY_OBJECT}, {Value: INEXACT_OBJECT_WITH_A_ONE}}
	}

	if len(o.dependencies) != 0 {
		return nil
	}

	return nil
}

func (p *Path) Examples(cctx ExampleComputationContext) []MatchingValueExample {
	if p.hasValue {
		return nil
	}

	return utils.FilterMapSlice(PATH_EXAMPLES, func(path *Path) (MatchingValueExample, bool) {
		if p.Test(path, RecTestCallState{}) {
			return MatchingValueExample{Value: path}, true
		}

		return MatchingValueExample{}, false
	})

}
