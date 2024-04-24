package codecompletion

import (
	"bytes"
	"slices"
	"strconv"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/jsoniter"
	parse "github.com/inoxlang/inox/internal/parse"
)

type expectedValueCompletionComputationConfig struct {
	expectedOrGuessedValue         symbolic.Value
	search                         completionSearch
	tryBestGuessIfNotConcretizable bool

	//Name of the property of which the completion of the value will be computed.
	//This field can be set to help guessing a good completion.
	propertyName string

	//Key of the entry of which the completion of the value will be computed.
	//This field can be set to help guessing a good completion.
	dictKey symbolic.Serializable

	parentPropertyName string
	parentDictKey      symbolic.Serializable

	_depth int //_getExpectedValueCompletion call depth
}

func getExpectedValueCompletion(params expectedValueCompletionComputationConfig) (completion string, isGuess bool, ok bool) {
	indentationUnit := params.search.chunk.EstimatedIndentationUnit()

	buf := make([]byte, 0, 10)

	isGuess, ok = _getExpectedValueCompletion(&buf, indentationUnit, params)
	if ok {
		completion = string(buf)
	}

	return
}

func _getExpectedValueCompletion(buf *[]byte, indentationUnit string, params expectedValueCompletionComputationConfig) (isGuess, completionOk bool) {
	expectedorGuessedValue := params.expectedOrGuessedValue
	search := params.search

	switch v := expectedorGuessedValue.(type) {
	case *symbolic.Object:
		completionOk = true

		appendByte(buf, '{')
		propCount := 0

		v.ForEachEntry(func(propName string, propValue symbolic.Value) error {
			if v.IsExistingPropertyOptional(propName) {
				return nil
			}
			propCount++

			appendByte(buf, '\n')
			for range params._depth + 1 {
				appendString(buf, indentationUnit)
			}

			appendPropName(buf, propName)
			appendString(buf, ": ")
			_isGuess, _ := _getExpectedValueCompletion(buf, indentationUnit, expectedValueCompletionComputationConfig{
				expectedOrGuessedValue:         propValue,
				search:                         search,
				propertyName:                   propName,
				parentPropertyName:             params.propertyName,
				parentDictKey:                  params.parentDictKey,
				tryBestGuessIfNotConcretizable: params.tryBestGuessIfNotConcretizable,
				_depth:                         params._depth + 1,
			})
			if _isGuess {
				isGuess = true
			}
			return nil
		})

		if propCount > 0 {
			appendByte(buf, '\n')
		}

		for range params._depth + 1 {
			appendString(buf, indentationUnit)
		}
		appendByte(buf, '}')

		return
	case *symbolic.Record:
		completionOk = true

		appendString(buf, "#{")
		propCount := 0

		v.ForEachEntry(func(propName string, propValue symbolic.Value) error {
			if v.IsExistingPropertyOptional(propName) {
				return nil
			}
			propCount++

			appendByte(buf, '\n')
			for range params._depth + 1 {
				appendString(buf, indentationUnit)
			}

			appendPropName(buf, propName)
			appendString(buf, ": ")

			_isGuess, _ := _getExpectedValueCompletion(buf, indentationUnit, expectedValueCompletionComputationConfig{
				expectedOrGuessedValue:         propValue,
				search:                         search,
				propertyName:                   propName,
				parentPropertyName:             params.propertyName,
				parentDictKey:                  params.parentDictKey,
				tryBestGuessIfNotConcretizable: params.tryBestGuessIfNotConcretizable,
				_depth:                         params._depth + 1,
			})
			if _isGuess {
				isGuess = true
			}
			return nil
		})

		if propCount > 0 {
			appendByte(buf, '\n')
		}

		for range params._depth {
			appendString(buf, indentationUnit)
		}
		appendByte(buf, '}')

		return
	case *symbolic.Dictionary:
		if !v.AllKeysConcretizable() {
			return
		}

		completionOk = true

		appendString(buf, ":{")

		v.ForEachEntry(func(key symbolic.Serializable, keyString string, value symbolic.Value) error {
			appendByte(buf, '\n')
			for range params._depth + 1 {
				appendString(buf, indentationUnit)
			}
			appendString(buf, keyString)

			appendString(buf, ": ")
			_isGuess, _ := _getExpectedValueCompletion(buf, indentationUnit, expectedValueCompletionComputationConfig{
				expectedOrGuessedValue:         value,
				search:                         search,
				dictKey:                        key,
				parentPropertyName:             params.propertyName,
				parentDictKey:                  params.parentDictKey,
				tryBestGuessIfNotConcretizable: params.tryBestGuessIfNotConcretizable,
				_depth:                         params._depth + 1,
			})
			if _isGuess {
				isGuess = true
			}
			return nil
		})

		appendByte(buf, '\n')
		for range params._depth {
			appendString(buf, indentationUnit)
		}
		appendByte(buf, '}')
	case *symbolic.List:
		if !v.HasKnownLen() {
			return
		}

		completionOk = true

		if v.KnownLen() == 0 {
			appendString(buf, "[]")
			return
		}

		appendString(buf, "[")

		for i := 0; i < v.KnownLen(); i++ {
			elem := v.ElementAt(i)

			appendByte(buf, '\n')
			for range params._depth + 1 {
				appendString(buf, indentationUnit)
			}

			_getExpectedValueCompletion(buf, indentationUnit, expectedValueCompletionComputationConfig{
				expectedOrGuessedValue:         elem,
				search:                         search,
				parentPropertyName:             params.propertyName,
				parentDictKey:                  params.parentDictKey,
				tryBestGuessIfNotConcretizable: params.tryBestGuessIfNotConcretizable,
				_depth:                         params._depth + 1,
			})
		}

		appendByte(buf, '\n')
		for range params._depth {
			appendString(buf, indentationUnit)
		}
		appendByte(buf, ']')
	case symbolic.StringLike:
		symbString := v.GetOrBuildString()
		if symbString.HasValue() {
			jsoniter.AppendString(buf, symbString.Value())
			completionOk = true
			return
		}
	case *symbolic.Bool:
		if v.IsConcretizable() {
			if v.MustGetValue() {
				appendString(buf, "true")
			} else {
				appendString(buf, "false")
			}
			completionOk = true
			return
		}
	case *symbolic.Int:
		if v.HasValue() {
			*buf = strconv.AppendInt(*buf, v.Value(), 10)
			completionOk = true
			return
		}
	case *symbolic.Float:
		if v.IsConcretizable() {
			prevLen := len(*buf)
			*buf = strconv.AppendFloat(*buf, v.MustGetValue(), 'f', -1, 64)
			if !bytes.ContainsAny((*buf)[prevLen:], ".e") {
				appendString(buf, ".0")
			}
			completionOk = true
			return
		}
	case *symbolic.Path:
		if v.IsConcretizable() {
			completionOk = true
			appendString(buf, symbolic.Stringify(v))
		}
	case *symbolic.PathPattern:
		if v.IsConcretizable() {
			completionOk = true
			appendString(buf, symbolic.Stringify(v))
		}
	case symbolic.IMultivalue:
		if !params.tryBestGuessIfNotConcretizable {
			return
		}

		guessingCtx, ok := getGuessingContext(search)
		if !ok || guessingCtx.goFunctionCallee == nil {
			return
		}

		switch guessingCtx.normalizeGoFunctionName {
		case getNormalizedGoFuncName((*symbolic.DatabaseIL).UpdateSchema):

			//If the expected value is a migration handler function or an initial value.

			if _, ok := params.dictKey.(*symbolic.PathPattern); ok && inoxconsts.IsDbMigrationPropertyName(params.parentPropertyName) {
				guessedValue, ok := guessDatabaseMigrationHandlerValue(params.propertyName, expectedorGuessedValue)
				if !ok {
					return
				}
				isGuess = true
				_, completionOk = _getExpectedValueCompletion(buf, indentationUnit, expectedValueCompletionComputationConfig{
					expectedOrGuessedValue:         guessedValue,
					search:                         search,
					tryBestGuessIfNotConcretizable: true,
					propertyName:                   params.propertyName,
					_depth:                         params._depth,
				})
				return
			}
		}
	}
	return
}

func guessDatabaseMigrationHandlerValue(propertyName string, expectedValue symbolic.Value) (symbolic.Value, bool) {
	if multiValue, ok := expectedValue.(symbolic.IMultivalue); ok {
		values := multiValue.OriginalMultivalue().Values()
		for _, val := range values {
			if symbolic.IsConcretizable(val) {
				return val, true
			}
		}
	}

	switch propertyName {
	case inoxconsts.DB_MIGRATION__INCLUSIONS_PROP_NAME:
	case inoxconsts.DB_MIGRATION__INITIALIZATIONS_PROP_NAME:
	case inoxconsts.DB_MIGRATION__REPLACEMENTS_PROP_NAME:
	case inoxconsts.DB_MIGRATION__DELETIONS_PROP_NAME:
	}

	return nil, false
}

type guessingContext struct {
	goFunctionCallee        *symbolic.GoFunction
	normalizeGoFunctionName string
}

func getGuessingContext(search completionSearch) (ctx guessingContext, isRelevant bool) {

	symbolicData := search.state.Global.SymbolicData

	if search.deepestCall == nil {
		return
	}

	deepestCall := search.deepestCall
	deepestCallIndex := slices.Index(search.ancestorChain, parse.Node(deepestCall))

	if deepestCallIndex < 0 {
		return
	}

	callee, _ := symbolicData.GetMostSpecificNodeValue(deepestCall.Callee)

	var goFunc *symbolic.GoFunction

	switch callee := callee.(type) {
	case *symbolic.GoFunction:
		goFunc = callee
	case *symbolic.Function:
		if fn, ok := callee.OriginGoFunction(); ok {
			goFunc = fn
		}
	}

	if goFunc == nil {
		return
	}

	ctx = guessingContext{
		goFunctionCallee:        goFunc,
		normalizeGoFunctionName: getNormalizedGoFuncName(goFunc.GoFunc()),
	}
	isRelevant = true
	return
}
