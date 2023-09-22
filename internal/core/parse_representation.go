package core

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/inoxlang/inox/internal/globalnames"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ErrNonSupportedMetaProperty = errors.New("non-supported meta property")
)

func ParseRepr(ctx *Context, b []byte) (Serializable, error) {
	v, errIndex, err := _parseRepr(b, ctx)
	if errIndex < 0 {
		return v, nil
	}

	if err != nil {
		return nil, fmt.Errorf("error at index %d: %w", errIndex, err)
	}

	if errIndex == len(b) {
		return nil, fmt.Errorf("error at end of representation")
	}

	return nil, fmt.Errorf("error at index %d, unexpected character '%s'", errIndex, string(b[errIndex]))
}

type reprParsingState int

const (
	rstateInit reprParsingState = iota
	rstateSingleDash
	rstateDoubleDash
	rstateColon
	rstateIdentLike
	rstateFlagLitName
	rstateOptionalPropKey
	rstateOptionalPropStringKey
	rstateDot
	rstateTwoDots
	rstatePercent
	rstatePercentDot
	rstatePercentTwoDots
	rstatePercentAlpha
	rstateHash
	rstateIdentifier
	rstateFinishedAtom

	//udata
	rstateUdataIdent
	rstateUdata
	rstateUdataAfterRoot
	rstateUdataOpeningBrace
	rstateUdataBodyComma
	rstateUdataClosingBrace
	rstateUdataHiearchyEntryOpeningBrace
	rstateUdataHiearchyEntryBodyComma
	rstateUdataHiearchyEntryAfterVal
	rstateUdataHiearchyEntryClosingBrace

	//function call
	rstateCallOpeningParen
	rstateCallComma
	rstateCallClosingParen

	//pattern call
	rstatePatternCallOpeningParen
	rstatePatternCallComma
	rstatePatternCallClosingParen

	rstatePropertyName

	//email addresses
	rstateEmailAddressUsername
	rstateEmailAddress

	//numbers, quantities
	rstateInt
	rstateIntDot
	rstateFloatDecimalPart
	rstateFloatE
	rstateFloatExponentNumber
	rstatePortNumber
	rstatePortSchemeName
	rstateIntDotDot
	rstateIntInclusiveRange

	//dates
	rstateDate

	//quantities & rates
	rstateQtyUnit
	rstateRateSlash
	rstateRateUnit

	//paths & path patterns
	rstatePathLike
	rstatePathPatternLike

	rstateUnquotedPathLike
	rstateQuotedPathLike
	rstateFinishedQuotedPathLike

	rstateUnquotedPathPatternLike
	rstateQuotedPathPatternLike
	rstateFinishedQuotedPathPatternLike

	//url & hosts
	rstateSchemeColon
	rstateSchemeSingleSlash
	rstateScheme
	rstateHostLike
	rstateURLLike

	//url & host patterns
	rstateURLPatternNoPostSchemeSlash
	rstateURLPatternSinglePostSchemeSlash
	rstateHostPattern
	rstateURLPatternInPath
	rstateURLPatternInQuery
	rstateURLPatternInFragment

	//runes
	rstateRune
	rstateClosingSimpleQuote

	//strings
	rstateString
	rstateClosingDoubleQuotes

	//object
	rstateObjOpeningBrace
	rstateObjectColon
	rstateObjClosingBrace
	rstateObjectComma

	//record
	rstateRecordOpeningBrace
	rstateRecordColon
	rstateRecordClosingBrace
	rstateRecordComma

	//object pattern
	rstateObjPatternOpeningBrace
	rstateObjPatternColon
	rstateObjPatternClosingBrace
	rstateObjectPatternComma

	//dict
	rstateDictOpeningBrace
	rstateDictColon
	rstateDictClosingBrace
	rstateDictComma

	//list
	rstateListOpeningBracket
	rstateListClosingBracket
	rstateListComma

	//tuple
	rstateTupleOpeningBracket
	rstateTupleClosingBracket
	rstateTupleGeneralElementPercent
	rstateTupleComma

	//tuple pattern
	rstateListPatternOpeningBracket
	rstateListPatternClosingBracket
	rstateListPatternComma

	//key list
	rstateKeyListOpeningBrace
	rstateKeyListComma
	rstateKeyListClosingBrace

	//byte slice
	rstate0x
	rstateByteSliceBytes
	rstateByteSliceClosingBracket

	rstatePatternConvOpeningParen
)

type CompoundValueKind int

const (
	NoVal CompoundValueKind = iota
	ObjVal
	LstVal
	TupleVal
	KLstVal
	DictVal
	RecordVal
	ObjectPatternVal
	ListPatternVal
	UdataVal
	UdataHiearchyEntryVal
	PatternCallVal
	CallVal
)

type InReprCall int

const (
	CreateRunesInRepr InReprCall = iota + 1
)

func _parseRepr(b []byte, ctx *Context) (val Serializable, errorIndex int, specifiedError error) {

	if len(b) == 0 {
		return nil, 0, nil
	}

	const stackHeight = 20

	var (
		state              reprParsingState
		prevAtomState      reprParsingState
		stateBeforeComment reprParsingState
		call               InReprCall
		commentEnd         = -1
		atomStartIndex     = -1
		atomEndIndex       = -1
		unitStart          = -1
		rateUnitStart      = -1
		quantityRateStart  = -1

		stackIndex               = -1
		stack                    [stackHeight]CompoundValueKind
		compoundValueStack       [stackHeight]Serializable
		objectKeyStack           [stackHeight]string
		optionalPropStack        [stackHeight]bool
		dictKeyStack             [stackHeight]Serializable
		hieararchyEntryHasBraces [stackHeight]bool
		inPattern                = []bool{false}
		byteSliceDigits          []byte
		quantityValues           []float64
		quantityUnits            []string
		callArguments            [][]Serializable //arguments for pattern calls & regular calls
		lastCompoundValue        Serializable
		i                        = -1
		c                        = byte(0)
	)

	defer func() {
		if v := recover(); v != nil {
			val = nil
			errorIndex = i
		}
	}()

	parseAtom := func() (Serializable, int, error) {
		var end = i

		if atomEndIndex > 0 {
			end = atomEndIndex
		}

		atomBytes := b[atomStartIndex:end]

		var v Serializable
		var index int = -1

		_state := state

		if _state == rstateFinishedAtom {
			_state = prevAtomState
			prevAtomState = -1
		}

		switch _state {
		case rstateIdentifier:
			v = Identifier(atomBytes[1:])
		case rstatePropertyName:
			v = PropertyName(atomBytes[1:])
		case rstatePercentAlpha:
			name := string(atomBytes[1:])
			v = ctx.ResolveNamedPattern(name)
			if v == nil {
				index = len(atomBytes)
				specifiedError = fmt.Errorf("named pattern %s is not defined", name)
				break
			}
		case rstateInt:
			if len(quantityUnits) != 0 && unitStart < 0 { //integer not followed by a unit
				index = len(atomBytes)
				break
			}
			v, index = _parseIntRepr(atomBytes)
		case rstateFloatDecimalPart, rstateFloatExponentNumber: //float not followed by a unit
			if len(quantityUnits) != 0 && unitStart < 0 {
				index = len(atomBytes)
				break
			}
			if len(quantityUnits) != 0 {
				index = len(atomBytes)
				break
			}
			v, index = _parseFloatRepr(atomBytes)
		case rstateIntInclusiveRange:
			lowerBoundBytes, upperBoundBytes, _ := bytes.Cut(atomBytes, []byte{'.', '.'})
			lowerBound, index := _parseIntRepr(lowerBoundBytes)
			if index >= 0 {
				break
			}
			upperBound, index := _parseIntRepr(upperBoundBytes)
			if index >= 0 {
				break
			}
			v = NewIncludedEndIntRange(int64(lowerBound), int64(upperBound))
		case rstatePortNumber, rstatePortSchemeName:
			v, index = _parsePortRepr(atomBytes)
		case rstateIdentLike:
			s := string(atomBytes)
			switch s {
			case "true":
				v = True
			case "false":
				v = False
			case "nil":
				v = Nil
			default:
				index = 0
			}
		case rstateClosingSimpleQuote:
			if atomBytes[1] == '\\' {
				if len(atomBytes) != 4 {
					index = 2
				} else {
					switch atomBytes[2] {
					case 'a':
						v = Rune('\a')
					case 'b':
						v = Rune('\b')
					case 'f':
						v = Rune('\f')
					case 'n':
						v = Rune('\n')
					case 'r':
						v = Rune('\r')
					case 't':
						v = Rune('\t')
					case 'v':
						v = Rune('\v')
					case '\\':
						v = Rune('\\')
					case '\'':
						v = Rune('\'')
					default:
						index = 1
					}
				}
			} else {
				r, size := utf8.DecodeRune(atomBytes[1:])
				if r == utf8.RuneError || size != len(atomBytes)-2 {
					index = 1
				} else {
					v = Rune(r)
				}
			}
		case rstateClosingDoubleQuotes:
			var s string
			err := json.Unmarshal(atomBytes, &s)
			if err != nil {
				index = len(atomBytes) //fix
			} else {
				switch call {
				case CreateRunesInRepr:
					v = NewRuneSlice([]rune(s))
				default:
					v = Str(s)
				}
			}
		case rstateByteSliceClosingBracket:
			if len(byteSliceDigits)%2 == 1 {
				index = len(atomBytes)
			} else if len(byteSliceDigits) == 0 {
				v = &ByteSlice{IsDataMutable: true}
			} else {
				decoded := make([]byte, hex.DecodedLen(len(byteSliceDigits)))
				_, err := hex.Decode(decoded, byteSliceDigits)
				if err != nil {
					index = len(atomBytes)
				} else {
					v = &ByteSlice{IsDataMutable: true, Bytes: decoded}
				}
			}
			byteSliceDigits = nil
		case rstateFlagLitName:
			var name string
			if atomBytes[1] == '-' {
				name = string(atomBytes[2:])
			} else {
				name = string(atomBytes[1:])
			}
			v = Option{Name: name, Value: True}
		case rstatePathLike, rstateUnquotedPathLike:
			v = Path(atomBytes)
		case rstateFinishedQuotedPathLike:
			var clean []byte

			for _, c := range atomBytes {
				if c == '`' {
					continue
				}
				clean = append(clean, c)
			}

			v = Path(clean)
		case rstatePathPatternLike, rstateUnquotedPathPatternLike:
			v = PathPattern(atomBytes[1:])
		case rstateFinishedQuotedPathPatternLike:
			var clean []byte

			for _, c := range atomBytes[1:] {
				if c == '`' {
					continue
				}
				clean = append(clean, c)
			}

			v = PathPattern(clean)
		case rstateScheme:
			s := string(atomBytes[:len(atomBytes)-3])
			if !utils.SliceContains(parse.SCHEMES, s) {
				index = len(atomBytes)
			} else {
				v = Scheme(s)
			}
		case rstateHostLike:
			if !parse.LOOSE_HOST_PATTERN_REGEX.Match(atomBytes) {
				index = len(atomBytes)
			} else {
				s := string(atomBytes)

				if err := parse.CheckHost(s); err != nil {
					index = len(atomBytes)
				} else {
					v = Host(s)
				}
			}
		case rstateHostPattern:
			if !parse.LOOSE_HOST_PATTERN_REGEX.Match(atomBytes[1:]) {
				index = len(atomBytes)
			} else {
				s := string(atomBytes[1:])
				if err := parse.CheckHostPattern(s); err != nil {
					index = len(atomBytes)
				} else {
					v = HostPattern(s)
				}
			}
		case rstateURLLike:
			if !parse.URL_REGEX.Match(atomBytes) {
				index = len(atomBytes)
			} else {
				v = URL(atomBytes)
			}
		case rstateURLPatternInPath, rstateURLPatternInQuery, rstateURLPatternInFragment:
			b := atomBytes[1:]
			if !parse.URL_REGEX.Match(b) || parse.CheckURLPattern(string(b)) != nil {
				index = len(atomBytes)
			} else {
				v = URLPattern(atomBytes[1:])
			}
		case rstateEmailAddress:
			_, domain, _ := bytes.Cut(atomBytes, []byte{'@'})
			lastDotIndex := bytes.LastIndexByte(domain, '.')

			//we check that the domain contains at least one '.' and that it is not at the end of the domain.
			if lastDotIndex < 0 || lastDotIndex == len(domain)-1 {
				index = len(atomBytes)
				break
			}

			//we check that the TLD part only contains alpha characters.
			for _, b := range domain[lastDotIndex+1:] {
				if !isAlpha(b) {
					index = len(atomBytes)
					break
				}
			}

			v = EmailAddress(atomBytes)
		case rstateDate:
			date, err := parse.ParseDateLiteral(atomBytes)
			if err != nil {
				index = len(atomBytes)
				break
			} else if date.Location() != time.UTC { //only UTC location allowed
				index = len(atomBytes)
				break
			}

			v = Date(date)
		case rstateQtyUnit, rstateRateUnit:
			if state == rstateQtyUnit {
				quantityUnits = append(quantityUnits, string(b[unitStart:i]))
			}

			qty, err := evalQuantity(quantityValues, quantityUnits)
			if err != nil {
				index = quantityRateStart
				break
			}
			quantityValues = nil
			quantityUnits = nil
			quantityRateStart = -1
			unitStart = -1

			v = qty

			if state == rstateRateUnit {
				rate, err := evalRate(qty, string(b[rateUnitStart:i]))
				if err != nil {
					index = i - quantityRateStart
					break
				}
				rateUnitStart = -1
				v = rate
			}
		default:
			panic(fmt.Errorf(""))
		}

		if p, ok := v.(PathPattern); ok {
			s := string(p)
			b := []byte(s)
			isPrefixPattern := strings.Contains(s, "/...")

			if isPrefixPattern && (!strings.HasSuffix(s, "/...") || strings.Contains(strings.TrimSuffix(s, "/..."), "/...")) {
				return nil, atomStartIndex + len(atomBytes), nil
			}

			hasGlobbing := false

			for i, c := range b {
				if (c == '[' || c == '*' || c == '?') && countPrevBackslashes(b, i)%2 == 0 {
					hasGlobbing = true
					break
				}
			}

			if isPrefixPattern && hasGlobbing {
				return nil, atomStartIndex + len(atomBytes), nil
			}
		}

		if index >= 0 {
			return nil, atomStartIndex + index, nil
		}

		atomStartIndex = -1
		atomEndIndex = -1
		return v, -1, nil
	}

	pushQuantityNumber := func() (errIndex int, specifiedError error) {

		if len(quantityValues) == 0 {
			quantityRateStart = atomStartIndex
		}

		number, errInd, specifiedError := parseAtom()
		if errInd >= 0 {
			return errInd, specifiedError
		}
		switch n := number.(type) {
		case Float:
			quantityValues = append(quantityValues, float64(n))
		case Int:
			quantityValues = append(quantityValues, float64(n))
		}

		atomStartIndex = quantityRateStart
		return -1, nil
	}

	getVal := func(i int) (Serializable, int, bool, error) {
		switch state {
		case rstateInt,
			rstateFloatDecimalPart, rstateFloatExponentNumber,
			rstateIntInclusiveRange,
			rstatePortNumber, rstatePortSchemeName,
			rstateFlagLitName, rstateClosingDoubleQuotes, rstateClosingSimpleQuote, rstateIdentLike,
			rstateByteSliceClosingBracket,
			//paths
			rstatePathLike, rstateUnquotedPathLike, rstateFinishedQuotedPathLike,
			//path patterns
			rstatePathPatternLike, rstateUnquotedPathPatternLike, rstateFinishedQuotedPathPatternLike,
			//hosts
			rstateScheme,
			rstateHostLike,
			//urls
			rstateURLLike,
			//url patterns
			rstateURLPatternInPath, rstateURLPatternInQuery, rstateURLPatternInFragment,
			//host patterns
			rstateHostPattern,
			//email address
			rstateEmailAddress,
			//dates
			rstateDate,
			//quantities & rates
			rstateQtyUnit,
			rstateRateUnit,
			//identifiers
			rstateIdentifier,
			rstatePropertyName,
			//named patterns
			rstatePercentAlpha,
			//
			rstateFinishedAtom:

			v, ind, specifiedError := parseAtom()
			return v, ind, true, specifiedError
		case rstateObjClosingBrace, rstateRecordClosingBrace, rstateDictClosingBrace, rstateListClosingBracket, rstateTupleClosingBracket,
			rstateObjPatternClosingBrace,
			rstateListPatternClosingBracket,
			rstatePatternCallClosingParen,
			rstateCallClosingParen,
			rstateKeyListClosingBrace,
			rstateUdataClosingBrace, rstateUdataHiearchyEntryClosingBrace:
			defer func() {
				lastCompoundValue = nil
			}()
			return lastCompoundValue, -1, true, nil
		default:
			return nil, -1, false, nil
		}
	}

	for i, c = range b {

		//handle comments, strings & paths separately because they accept a wide range of characters
		switch state {
		case stateBeforeComment:
			if i < commentEnd {
				continue
			}
			commentEnd = -1
			state = stateBeforeComment
			stateBeforeComment = -1
		case rstatePathLike:
			switch c {
			case '`':
				state = rstateQuotedPathLike
				continue
			case '"', '{', '\n':
				return nil, i, nil
			case ' ', '\t', '\r', ']', '}', ')', ',', ':', '|':
			default:
				if isNextForbiddenSpaceCharacter(i, b) {
					return nil, i, nil
				}
				state = rstateUnquotedPathLike
				continue
			}
		case rstatePathPatternLike:
			if atomEndIndex >= 0 {
				return nil, i, nil
			}
			switch c {
			case '`':
				state = rstateQuotedPathPatternLike
				continue
			case '"', '{', '\n':
				return nil, i, nil
			case ' ', '\t', '\r', ']', '}', ')', ',', ':', '|':
			default:
				if isNextForbiddenSpaceCharacter(i, b) {
					return nil, i, nil
				}
				state = rstateUnquotedPathPatternLike
				continue
			}
		case rstateRune:
			switch c {
			case '\n':
				return nil, i, nil
			case '\'':
			default:
				continue
			}
		case rstateString:
			switch c {
			case '\n':
				return nil, i, nil
			case '"':
			default:
				continue
			}
		case rstateByteSliceBytes:
			switch c {
			case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				byteSliceDigits = append(byteSliceDigits, byte(c))
				continue
			case ' ', '\t', '\r':
				continue
			}
		case rstateUnquotedPathLike, rstateUnquotedPathPatternLike:
			if atomEndIndex >= 0 {
				return nil, i, nil
			}
			switch c {
			case '"', '{', '\n':
				return nil, i, nil
			case ' ', '\t', '\r', ']', '}', ')', ',', ':', '|':
			default:
				if isNextForbiddenSpaceCharacter(i, b) {
					return nil, i, nil
				}
				continue
			}
		case rstateQuotedPathLike, rstateQuotedPathPatternLike:
			if c == '{' || c == '\n' {
				return nil, i, nil
			}
			if c != '`' {
				continue
			}
		case rstateUdataIdent:
			if isAlpha(c) || isDigit(c) || c == '-' || c == '_' {
				state = rstateIdentifier
				continue
			} else {
				state = rstateUdata
				stackIndex++
				stack[stackIndex] = UdataVal
				compoundValueStack[stackIndex] = &UData{}
				prevAtomState = -1
				atomStartIndex = -1
				atomEndIndex = -1
			}
		case rstateFinishedAtom:
			switch c {
			case ' ', '\t', '\r', ']', '}', ')', ',', ':', '|':
			case '#':
				if i >= len(b)-1 || !parse.IsCommentFirstSpace(rune(b[i+1])) { //not comment
					return nil, i, nil
				}
			case '\n':
				return nil, i, nil
			default:
				return nil, atomEndIndex, nil
			}
		}

		switch c {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			switch state {
			case rstateInit,
				rstateObjectColon, rstateObjectComma,
				rstateRecordColon, rstateRecordComma,
				rstateObjPatternColon, rstateObjectPatternComma,
				rstateDictOpeningBrace, rstateDictColon,
				rstateListOpeningBracket, rstateListComma,
				rstateTupleOpeningBracket, rstateTupleComma,
				rstateListPatternOpeningBracket, rstateListPatternComma,
				rstatePatternCallOpeningParen, rstatePatternCallComma,
				rstateCallOpeningParen, rstateCallComma,
				rstatePatternConvOpeningParen,
				rstateUdata, rstateUdataOpeningBrace, rstateUdataBodyComma,
				rstateUdataHiearchyEntryOpeningBrace, rstateUdataHiearchyEntryBodyComma, rstateUdataHiearchyEntryClosingBrace:

				if atomEndIndex >= 0 {
					return nil, atomEndIndex, nil
				}

				atomStartIndex = i
				state = rstateInt
				prevAtomState = -1
			case rstateSingleDash:
				if atomEndIndex >= 0 {
					return nil, atomEndIndex, nil
				}
				atomStartIndex = i - 1
				state = rstateInt
				prevAtomState = -1
			case rstateIntDot:
				state = rstateFloatDecimalPart
			case rstateFloatE:
				state = rstateFloatExponentNumber
			case rstateIntDotDot:
				state = rstateIntInclusiveRange
			case rstateColon:
				atomStartIndex = i - 1
				state = rstatePortNumber
				prevAtomState = -1
			case rstateQtyUnit:
				quantityUnits = append(quantityUnits, string(b[unitStart:i]))
				unitStart = -1
				atomStartIndex = i
				state = rstateInt
			case rstatePathLike, rstatePathPatternLike:
				panic(ErrUnreachable)
			case rstateScheme:
				state = rstateHostLike
			case rstateInt, rstateIdentLike,
				rstateByteSliceBytes,
				rstateFloatDecimalPart, rstateFloatExponentNumber,
				rstateIntInclusiveRange,
				rstatePortNumber, rstatePortSchemeName,
				rstateString,
				rstateUnquotedPathPatternLike,
				rstateURLLike,
				rstateHostLike,
				rstateHostPattern,
				rstateURLPatternInPath, rstateURLPatternInQuery, rstateURLPatternInFragment,
				rstateEmailAddress,
				rstateDate,
				rstateIdentifier,
				rstatePropertyName:
			default:
				return nil, i, nil
			}
		case '-':
			switch state {
			case rstateInit,
				rstateObjectColon, rstateObjectComma,
				rstateRecordColon, rstateRecordComma,
				rstateObjPatternColon, rstateObjectPatternComma,
				rstateDictOpeningBrace, rstateDictColon,
				rstateListOpeningBracket, rstateListComma,
				rstateTupleOpeningBracket, rstateTupleComma,
				rstateListPatternOpeningBracket, rstateListPatternComma,
				rstatePatternCallOpeningParen, rstatePatternCallComma,
				rstateCallOpeningParen, rstateCallComma,
				rstatePatternConvOpeningParen:

				if atomEndIndex >= 0 {
					return nil, atomEndIndex, nil
				}

				atomStartIndex = i
				state = rstateSingleDash
				prevAtomState = -1
			case rstateHash:
				state = rstateIdentifier
			case rstateDot:
				state = rstatePropertyName
			case rstateSingleDash:
				state = rstateDoubleDash
			case rstateFloatE:
				state = rstateFloatExponentNumber
			case rstateIdentLike, rstateString,
				rstateHostLike, rstateHostPattern,
				rstateURLPatternInPath,
				rstateEmailAddressUsername, rstateEmailAddress,
				rstateDate,
				rstateIdentifier,
				rstatePropertyName:
			default:
				return nil, i, nil
			}
		case '_':
			switch state {
			case rstateInit,
				rstateObjectColon, rstateObjectComma,
				rstateRecordColon, rstateRecordComma,
				rstateObjPatternColon, rstateObjectPatternComma,
				rstateDictOpeningBrace, rstateDictColon,
				rstateListOpeningBracket, rstateListComma,
				rstateTupleOpeningBracket, rstateTupleComma,
				rstateListPatternOpeningBracket, rstateListPatternComma,
				rstatePatternCallOpeningParen, rstatePatternCallComma,
				rstateCallOpeningParen, rstateCallComma,
				rstatePatternConvOpeningParen,
				rstateUdata, rstateUdataOpeningBrace, rstateUdataBodyComma, rstateUdataHiearchyEntryOpeningBrace, rstateUdataHiearchyEntryBodyComma:

				if atomEndIndex >= 0 {
					return nil, atomEndIndex, nil
				}

				atomStartIndex = i
				state = rstateIdentLike
				prevAtomState = -1
			case rstateHash:
				state = rstateIdentifier
			case rstateDot:
				state = rstatePropertyName
			case rstateIdentLike, rstateString,
				rstateURLPatternInPath, rstateURLPatternInQuery, rstateURLPatternInFragment,
				rstateEmailAddressUsername,
				rstateIdentifier,
				rstatePropertyName:
			default:
				return nil, i, nil
			}
		case '.':
			switch state {
			case rstateInit,
				rstateObjectColon, rstateObjectComma,
				rstateRecordColon, rstateRecordComma,
				rstateObjPatternColon, rstateObjectPatternComma,
				rstateDictOpeningBrace, rstateDictColon,
				rstateListOpeningBracket, rstateListComma,
				rstateTupleOpeningBracket, rstateTupleComma,
				rstateListPatternOpeningBracket, rstateListPatternComma,
				rstatePatternCallOpeningParen, rstatePatternCallComma,
				rstateCallOpeningParen, rstateCallComma,
				rstatePatternConvOpeningParen,
				rstateUdata, rstateUdataOpeningBrace, rstateUdataBodyComma, rstateUdataHiearchyEntryOpeningBrace, rstateUdataHiearchyEntryBodyComma:

				if atomEndIndex >= 0 {
					return nil, atomEndIndex, nil
				}

				atomStartIndex = i
				state = rstateDot
				prevAtomState = -1
			case rstateInt:
				state = rstateIntDot
			case rstateIntDot:
				state = rstateIntDotDot
			case rstateDot:
				state = rstateTwoDots
			case rstatePercent:
				state = rstatePercentDot
			case rstatePercentDot:
				state = rstatePercentTwoDots
			case rstateHostPattern:
				if b[i-1] == '.' {
					return nil, i, nil
				}
			case rstateIdentLike:
				state = rstateEmailAddressUsername
			case rstateHostLike, rstateURLLike, rstateURLPatternInPath,
				rstateEmailAddressUsername, rstateEmailAddress:
			default:
				return nil, i, nil
			}
		case '/':
			switch state {
			case rstateInit,
				rstateObjectColon, rstateObjectComma,
				rstateRecordColon, rstateRecordComma,
				rstateObjPatternColon, rstateObjectPatternComma,
				rstateDictOpeningBrace, rstateDictColon,
				rstateListOpeningBracket, rstateListComma,
				rstateTupleOpeningBracket, rstateTupleComma,
				rstateListPatternOpeningBracket, rstateListPatternComma,
				rstatePatternCallOpeningParen, rstatePatternCallComma,
				rstateCallOpeningParen, rstateCallComma,
				rstatePatternConvOpeningParen,
				rstateUdata, rstateUdataOpeningBrace, rstateUdataBodyComma, rstateUdataHiearchyEntryOpeningBrace, rstateUdataHiearchyEntryBodyComma:

				if atomEndIndex >= 0 {
					return nil, atomEndIndex, nil
				}

				atomStartIndex = i
				state = rstatePathLike
				prevAtomState = -1
			case rstatePortNumber:
				state = rstatePortSchemeName
			case rstateDot, rstateTwoDots:
				state = rstatePathLike
			case rstatePercent, rstatePercentDot, rstatePercentTwoDots:
				state = rstatePathPatternLike
			case rstateSchemeColon:
				state = rstateSchemeSingleSlash
			case rstateSchemeSingleSlash:
				state = rstateScheme
			case rstateHostLike:
				if b[i-1] != ':' && (b[i-2] != ':' || b[i-1] != '/') {
					state = rstateURLLike
				}
			case rstateUnquotedPathLike, rstateUnquotedPathPatternLike:
				panic(ErrUnreachable)
			case rstateURLPatternNoPostSchemeSlash:
				state = rstateURLPatternSinglePostSchemeSlash
			case rstateURLPatternSinglePostSchemeSlash:
				state = rstateHostPattern
			case rstateHostPattern:
				state = rstateURLPatternInPath
			case rstateQtyUnit:
				quantityUnits = append(quantityUnits, string(b[unitStart:i]))
				state = rstateRateSlash
			case rstateURLLike, rstateURLPatternInPath,
				rstateDate:
			default:
				return nil, i, nil
			}
		case '%':
			switch state {
			case rstateInit,
				rstateObjectColon, rstateObjectComma,
				rstateRecordColon, rstateRecordComma,
				rstateObjPatternColon, rstateObjectPatternComma,
				rstateDictOpeningBrace, rstateDictColon,
				rstateListOpeningBracket, rstateListComma,
				rstateTupleOpeningBracket, rstateTupleComma,
				rstateListPatternOpeningBracket, rstateListPatternComma,
				rstatePatternCallOpeningParen, rstatePatternCallComma,
				rstateCallOpeningParen, rstateCallComma,
				rstatePatternConvOpeningParen,
				rstateUdata, rstateUdataOpeningBrace, rstateUdataBodyComma, rstateUdataHiearchyEntryOpeningBrace, rstateUdataHiearchyEntryBodyComma:
				if atomEndIndex >= 0 {
					return nil, atomEndIndex, nil
				}

				atomStartIndex = i
				state = rstatePercent
				prevAtomState = -1
			case rstateListPatternClosingBracket:
				state = rstateTupleGeneralElementPercent
			case rstateIdentLike:
				state = rstateEmailAddressUsername
			case rstateHostLike, rstateURLPatternInPath, rstateURLPatternInQuery, rstateURLPatternInFragment,
				rstateEmailAddressUsername:
			case rstateInt, rstateFloatDecimalPart, rstateFloatExponentNumber:
				unitStart = i
				if ind, err := pushQuantityNumber(); ind >= 0 {
					return nil, ind, err
				}
				state = rstateQtyUnit
			default:
				return nil, i, nil
			}
		case '?':
			switch state {
			case rstateURLLike,
				rstateHostPattern:
			case rstateURLPatternInPath:
				state = rstateURLPatternInQuery
			case rstateIdentLike:
				state = rstateOptionalPropKey
			case rstateClosingDoubleQuotes:
				state = rstateOptionalPropStringKey
			default:
				return nil, i, nil
			}
		case '*':
			switch state {
			case rstateURLLike,
				rstateHostPattern,
				rstateURLPatternInPath, rstateURLPatternInQuery, rstateURLPatternInFragment:
			default:
				return nil, i, nil
			}
		case '{':
			if inPattern[len(inPattern)-1] && state != rstatePercent {
				return nil, i, nil
			}

			switch state {
			case rstateInit,
				rstateObjectColon, rstateObjectComma,
				rstateRecordColon, rstateRecordComma,
				rstateObjPatternColon, rstateObjectPatternComma,
				rstateDictOpeningBrace, rstateDictColon,
				rstateListOpeningBracket, rstateListComma,
				rstateTupleOpeningBracket, rstateTupleComma,
				rstateListPatternOpeningBracket, rstateListPatternComma,
				rstatePatternCallOpeningParen, rstatePatternCallComma,
				rstateCallOpeningParen, rstateCallComma,
				rstatePatternConvOpeningParen,
				rstateUdataOpeningBrace, rstateUdataBodyComma, rstateUdataHiearchyEntryOpeningBrace, rstateUdataHiearchyEntryBodyComma:

				state = rstateObjOpeningBrace
				stackIndex++
				stack[stackIndex] = ObjVal
				compoundValueStack[stackIndex] = &Object{}
			case rstateDot:
				if atomEndIndex >= 0 {
					return nil, atomEndIndex, nil
				}
				atomStartIndex = -1
				state = rstateKeyListOpeningBrace
				stackIndex++
				stack[stackIndex] = KLstVal
				compoundValueStack[stackIndex] = make(KeyList, 0)
			case rstateColon:
				atomStartIndex = -1
				state = rstateDictOpeningBrace
				stackIndex++
				stack[stackIndex] = DictVal
				compoundValueStack[stackIndex] = NewDictionary(nil)
			case rstatePercent:
				atomStartIndex = -1
				state = rstateObjPatternOpeningBrace
				stackIndex++
				stack[stackIndex] = ObjectPatternVal
				compoundValueStack[stackIndex] = &ObjectPattern{inexact: true}
				inPattern = append(inPattern, true)
			case rstateHash:
				atomStartIndex = -1
				state = rstateRecordOpeningBrace
				stackIndex++
				stack[stackIndex] = RecordVal
				compoundValueStack[stackIndex] = &Record{}
			case rstateUdata, rstateUdataAfterRoot:
				state = rstateUdataOpeningBrace

				//set current value as a hiearchy entry
				stackIndex++
				stack[stackIndex] = UdataHiearchyEntryVal
				compoundValueStack[stackIndex] = &UDataHiearchyEntry{}
				hieararchyEntryHasBraces[stackIndex] = false
			default:
				switch stack[stackIndex] {
				case UdataHiearchyEntryVal:
					if state != rstateUdataHiearchyEntryAfterVal {
						val, index, ok, specifiedError := getVal(i)
						if index > 0 || !ok {
							return nil, index, specifiedError
						}
						entry := compoundValueStack[stackIndex].(*UDataHiearchyEntry)
						entry.Value = val
					}

					state = rstateUdataHiearchyEntryOpeningBrace
					hieararchyEntryHasBraces[stackIndex] = true

					//set current value as a hiearchy entry
					stackIndex++
					stack[stackIndex] = UdataHiearchyEntryVal
					compoundValueStack[stackIndex] = &UDataHiearchyEntry{}
					hieararchyEntryHasBraces[stackIndex] = false
				default:
					return nil, i, nil
				}
			}
		case '}':
			if state == rstateString {
				continue
			}

			switch stack[stackIndex] {
			case ObjVal:
				switch state {
				case rstateObjOpeningBrace, rstateObjectComma:
				case rstateObjectColon:
					return nil, i, nil
				default:
					key := objectKeyStack[stackIndex]
					if key == "" {
						return nil, i, nil
					}
					if val, errIndex, ok, specifiedError := getVal(i); ok {
						if errIndex >= 0 {
							return nil, errIndex, specifiedError
						}
						obj := compoundValueStack[stackIndex].(*Object)

						//property
						if key != URL_METADATA_KEY {
							obj.keys = append(obj.keys, key)
							obj.values = append(obj.values, val)
						} else {
							obj.url = val.(URL)
						}

						objectKeyStack[stackIndex] = ""
					} else {
						return nil, i, nil
					}
				}

				obj := compoundValueStack[stackIndex].(*Object)
				obj.sortProps()
				// add handlers before because jobs can mutate the object
				if err := obj.addMessageHandlers(ctx); err != nil {
					return nil, i, nil
				}
				if err := obj.instantiateLifetimeJobs(ctx); err != nil {
					return nil, i, nil
				}
				lastCompoundValue = obj
				stack[stackIndex] = NoVal
				stackIndex--
				state = rstateObjClosingBrace

			case RecordVal:
				switch state {
				case rstateRecordOpeningBrace, rstateRecordComma:
				case rstateRecordColon:
					return nil, i, nil
				default:
					key := objectKeyStack[stackIndex]
					if key == "" {
						return nil, i, nil
					}
					if val, errIndex, ok, specifiedError := getVal(i); ok {
						if errIndex >= 0 {
							return nil, errIndex, specifiedError
						}
						record := compoundValueStack[stackIndex].(*Record)
						record.keys = append(record.keys, key)
						record.values = append(record.values, val)
						objectKeyStack[stackIndex] = ""
					} else {
						return nil, i, nil
					}
				}

				rec := compoundValueStack[stackIndex].(*Record)
				rec.sortProps()
				lastCompoundValue = rec
				stack[stackIndex] = NoVal
				stackIndex--
				state = rstateRecordClosingBrace

			case ObjectPatternVal:
				switch state {
				case rstateObjPatternOpeningBrace, rstateObjectPatternComma:
				case rstateObjPatternColon:
					return nil, i, nil
				default:
					key := objectKeyStack[stackIndex]
					if key == "" {
						return nil, i, nil
					}

					isOptionalProp := optionalPropStack[stackIndex]
					optionalPropStack[stackIndex] = false

					if val, errIndex, ok, specifiedError := getVal(i); ok {
						if errIndex >= 0 {
							return nil, errIndex, specifiedError
						}
						pattern := compoundValueStack[stackIndex].(*ObjectPattern)
						if pattern.entryPatterns == nil {
							pattern.entryPatterns = map[string]Pattern{}
						}
						if isOptionalProp {
							if pattern.optionalEntries == nil {
								pattern.optionalEntries = map[string]struct{}{}
							}
							pattern.optionalEntries[key] = struct{}{}
						}

						pattern.entryPatterns[key] = toPattern(val)
						objectKeyStack[stackIndex] = ""
					} else {
						return nil, i, nil
					}
				}

				inPattern = inPattern[:len(inPattern)-1]

				patt := compoundValueStack[stackIndex].(*ObjectPattern)
				lastCompoundValue = patt
				stack[stackIndex] = NoVal
				stackIndex--
				state = rstateObjPatternClosingBrace
			case DictVal:
				switch state {
				case rstateDictOpeningBrace, rstateDictComma:
				case rstateDictColon:
					return nil, i, nil
				default:
					key := dictKeyStack[stackIndex]
					if key == nil {
						return nil, i, nil
					}
					if val, errIndex, ok, specifiedError := getVal(i); ok {
						if errIndex >= 0 {
							return nil, errIndex, specifiedError
						}
						keyRepr := string(GetRepresentation(key, ctx)) // representation is context-dependent -> possible issues
						dict := compoundValueStack[stackIndex].(*Dictionary)
						dict.keys[keyRepr] = key
						dict.entries[keyRepr] = val
						dictKeyStack[stackIndex] = nil
					} else {
						return nil, i, nil
					}
				}

				lastCompoundValue = compoundValueStack[stackIndex]
				stack[stackIndex] = NoVal
				stackIndex--
				state = rstateDictClosingBrace

			case KLstVal:
				switch state {
				case rstateKeyListOpeningBrace, rstateKeyListComma:
				default:
					var end int = i
					if atomEndIndex > 0 {
						end = atomEndIndex
					}
					compoundValueStack[stackIndex] = append(compoundValueStack[stackIndex].(KeyList), string(b[atomStartIndex:end]))
					atomStartIndex = -1
					atomEndIndex = -1
					state = rstateKeyListComma
				}

				lastCompoundValue = compoundValueStack[stackIndex]
				stack[stackIndex] = NoVal
				stackIndex--
				state = rstateKeyListClosingBrace

			case UdataHiearchyEntryVal: // end of hiearchy entry body or its parent
				entry := compoundValueStack[stackIndex].(*UDataHiearchyEntry)

				if !hieararchyEntryHasBraces[stackIndex] { // end of parent
					if entry.Value == nil {
						if val, errIndex, ok, specifiedError := getVal(i); ok {
							if errIndex >= 0 {
								return nil, errIndex, specifiedError
							}

							entry.Value = val
						}
					}

					// pop child
					compoundValueStack[stackIndex] = nil
					hieararchyEntryHasBraces[stackIndex] = false
					stack[stackIndex] = NoVal
					stackIndex--

					parentIndex := stackIndex

					switch p := compoundValueStack[parentIndex].(type) {
					case *UData:
						if entry.Value != nil {
							p.HiearchyEntries = append(p.HiearchyEntries, *entry)
						}
						state = rstateUdataClosingBrace

						//pop parent
						lastCompoundValue = p
						compoundValueStack[parentIndex] = nil
						hieararchyEntryHasBraces[stackIndex] = false
						stack[stackIndex] = NoVal
						stackIndex--

					case *UDataHiearchyEntry:
						if entry.Value != nil {
							p.Children = append(p.Children, *entry)
						}
						state = rstateUdataHiearchyEntryClosingBrace

						//add parent to grand parent + reset parent

						switch grandParent := compoundValueStack[parentIndex-1].(type) {
						case *UData:
							grandParent.HiearchyEntries = append(grandParent.HiearchyEntries, *p)
						case *UDataHiearchyEntry:
							grandParent.Children = append(grandParent.Children, *p)
							state = rstateUdataHiearchyEntryClosingBrace
						}

						*p = UDataHiearchyEntry{}
						hieararchyEntryHasBraces[parentIndex] = false
					}

				} else { //end of entry's body
					switch p := compoundValueStack[stackIndex-1].(type) {
					case *UData:
						p.HiearchyEntries = append(p.HiearchyEntries, *entry)
					case *UDataHiearchyEntry:
						p.Children = append(p.Children, *entry)
					}

					state = rstateUdataHiearchyEntryClosingBrace
					*entry = UDataHiearchyEntry{} //reset
					hieararchyEntryHasBraces[stackIndex] = false
				}
			default:
				return nil, i, nil
			}

		case ':':
			if prevAtomState == rstateIdentLike || prevAtomState == rstateOptionalPropKey || prevAtomState == rstateOptionalPropStringKey {
				state = prevAtomState
				prevAtomState = -1
			}

			switch state {
			case rstateInit,
				rstateObjectColon,
				rstateRecordColon,
				rstateObjPatternColon,
				rstateDictOpeningBrace, rstateDictColon,
				rstateListOpeningBracket, rstateListComma,
				rstateTupleOpeningBracket, rstateTupleComma,
				rstateListPatternOpeningBracket, rstateListPatternComma,
				rstatePatternCallOpeningParen, rstatePatternCallComma,
				rstateCallOpeningParen, rstateCallComma,
				rstatePatternConvOpeningParen:

				if inPattern[len(inPattern)-1] {
					return nil, i, nil
				}

				state = rstateColon
			case rstateUdataOpeningBrace, rstateUdataBodyComma:
				return nil, i, nil
			case rstateIdentLike:

				if i < len(b)-2 && b[i+1] == '/' && b[i+2] == '/' {
					switch i - atomStartIndex {
					case 2:
						if bytes.Equal(b[atomStartIndex:i], utils.StringAsBytes("ws")) {
							state = rstateSchemeColon
							continue
						}
					case 3:
						if bytes.Equal(b[atomStartIndex:i], utils.StringAsBytes("wss")) {
							state = rstateSchemeColon
							continue
						}
						if bytes.Equal(b[atomStartIndex:i], utils.StringAsBytes("ldb")) {
							state = rstateSchemeColon
							continue
						}
						if bytes.Equal(b[atomStartIndex:i], utils.StringAsBytes("odb")) {
							state = rstateSchemeColon
							continue
						}
					case 4:
						if bytes.Equal(b[atomStartIndex:i], utils.StringAsBytes("http")) {
							state = rstateSchemeColon
							continue
						}
						if bytes.Equal(b[atomStartIndex:i], utils.StringAsBytes("file")) {
							state = rstateSchemeColon
							continue
						}
					case 5:
						if bytes.Equal(b[atomStartIndex:i], utils.StringAsBytes("https")) {
							state = rstateSchemeColon
							continue
						}
					default: //invalid scheme
						return nil, i + 2, nil
					}
				}

				if objectKeyStack[stackIndex] != "" {
					return nil, i, nil
				}
				var end int = i
				if atomEndIndex > 0 {
					end = atomEndIndex
				}

				key := string(b[atomStartIndex:end])
				if parse.IsMetadataKey(key) && (key != URL_METADATA_KEY || stack[stackIndex] != ObjVal) {
					return nil, end, ErrNonSupportedMetaProperty
				}
				objectKeyStack[stackIndex] = key

				atomStartIndex = -1
				atomEndIndex = -1
				state = rstateObjectColon
			case rstateOptionalPropKey:
				var end int = i
				if atomEndIndex > 0 {
					end = atomEndIndex
				}

				if stack[stackIndex] != ObjectPatternVal {
					return nil, i, nil
				}

				key := string(b[atomStartIndex : end-1])
				if parse.IsMetadataKey(key) {
					return nil, end, ErrNonSupportedMetaProperty
				}

				objectKeyStack[stackIndex] = key
				optionalPropStack[stackIndex] = true
				atomStartIndex = -1
				atomEndIndex = -1
				state = rstateObjPatternColon
			case rstateOptionalPropStringKey:

				var end int = i
				if atomEndIndex > 0 {
					end = atomEndIndex
				}

				if stack[stackIndex] != ObjectPatternVal {
					return nil, i, nil
				}

				var s string
				bytes := b[atomStartIndex:end]
				err := json.Unmarshal(bytes, &s)
				if err != nil {
					return nil, i, nil
				}

				key := s
				if parse.IsMetadataKey(key) {
					return nil, end, ErrNonSupportedMetaProperty
				}

				objectKeyStack[stackIndex] = key
				optionalPropStack[stackIndex] = true
				atomStartIndex = -1
				atomEndIndex = -1
				state = rstateObjPatternColon
			case rstatePercent:
				state = rstateURLPatternNoPostSchemeSlash
			case rstatePercentAlpha:
				scheme := b[atomStartIndex+1 : i]
				if !parse.IsSupportedSchemeName(string(scheme)) {
					if len(scheme) > parse.MAX_SCHEME_NAME_LEN {
						return nil, i, nil
					}
					continue
				}
				state = rstateURLPatternNoPostSchemeSlash
			case rstateHostLike, rstateHostPattern, rstateURLPatternNoPostSchemeSlash:
				continue
			default:
				//key

				switch stack[stackIndex] {
				case DictVal:
					key := dictKeyStack[stackIndex]
					if key != nil || stack[stackIndex] != DictVal {
						return nil, i, nil
					}
					if val, errIndex, ok, specifiedError := getVal(i); ok {
						if errIndex >= 0 {
							return nil, errIndex, specifiedError
						}
						switch v := val.(type) {
						case Str, Path, PathPattern, URL, URLPattern, Host, HostPattern, Bool:
							dictKeyStack[stackIndex] = v
							state = rstateDictColon
							continue
						}
					}
					return nil, i, nil
				case ObjVal:
					key := objectKeyStack[stackIndex]
					if key != "" {
						return nil, i, nil
					}
					if val, errIndex, ok, specifiedError := getVal(i); ok {
						if errIndex >= 0 {
							return nil, errIndex, specifiedError
						}
						switch v := val.(type) {
						case Str:
							key := string(v)
							if parse.IsMetadataKey(key) && key != URL_METADATA_KEY {
								return nil, i, ErrNonSupportedMetaProperty //TODO: return end position of key
							}
							objectKeyStack[stackIndex] = key

							state = rstateObjectColon
							continue
						}
					}
					return nil, i, nil
				case RecordVal:
					key := objectKeyStack[stackIndex]
					if key != "" {
						return nil, i, nil
					}
					if val, errIndex, ok, specifiedError := getVal(i); ok {
						if errIndex >= 0 {
							return nil, errIndex, specifiedError
						}
						switch v := val.(type) {
						case Str:
							key := string(v)
							if parse.IsMetadataKey(key) {
								return nil, i, ErrNonSupportedMetaProperty //TODO: return end position of key
							}
							objectKeyStack[stackIndex] = key

							state = rstateRecordColon
							continue
						}
					}
					return nil, i, nil

				case ObjectPatternVal:
					key := objectKeyStack[stackIndex]
					if key != "" {
						return nil, i, nil
					}
					if val, errIndex, ok, specifiedError := getVal(i); ok {
						if errIndex >= 0 {
							return nil, errIndex, specifiedError
						}
						switch v := val.(type) {
						case Str:
							key := string(v)
							if parse.IsMetadataKey(key) {
								return nil, i, ErrNonSupportedMetaProperty //TODO: return end position of key
							}
							objectKeyStack[stackIndex] = key

							state = rstateObjPatternColon
							continue
						}
					}
					return nil, i, nil

				default:
					if prevAtomState >= 0 {
						return nil, atomEndIndex, nil
					}
					return nil, i, nil
				}
			}

		case ',':
			switch state {
			case rstateHostLike, rstateURLPatternNoPostSchemeSlash,
				rstateObjectComma, rstateRecordComma, rstateObjectPatternComma, rstateDictComma, rstateListComma,
				rstateTupleComma, rstateKeyListComma, rstatePatternCallComma,
				rstateUdataBodyComma, rstateUdataHiearchyEntryBodyComma:
				continue
			case rstateUdataHiearchyEntryClosingBrace:
				state = rstateUdataHiearchyEntryBodyComma
				continue
			}

			switch stack[stackIndex] {
			case ObjVal:
				key := objectKeyStack[stackIndex]
				if key == "" {
					state = rstateObjectComma
					continue
				}

				if val, errIndex, ok, specifiedError := getVal(i); ok {
					if errIndex >= 0 {
						return nil, errIndex, specifiedError
					}

					obj := compoundValueStack[stackIndex].(*Object)

					//property
					if key != URL_METADATA_KEY {
						obj.keys = append(obj.keys, key)
						obj.values = append(obj.values, val)
					} else {
						obj.url = val.(URL)
					}

					objectKeyStack[stackIndex] = ""
					state = rstateObjectComma
					continue
				}
			case RecordVal:
				key := objectKeyStack[stackIndex]
				if key == "" {
					state = rstateRecordComma
					continue
				}

				if val, errIndex, ok, specifiedError := getVal(i); ok {
					if errIndex >= 0 {
						return nil, errIndex, specifiedError
					}
					record := compoundValueStack[stackIndex].(*Record)
					record.keys = append(record.keys, key)
					record.values = append(record.values, val)
					objectKeyStack[stackIndex] = ""
					state = rstateRecordComma
					continue
				}
			case ObjectPatternVal:
				key := objectKeyStack[stackIndex]
				if key == "" {
					state = rstateObjectPatternComma
					continue
				}

				isOptionalProp := optionalPropStack[stackIndex]
				optionalPropStack[stackIndex] = false

				if val, errIndex, ok, specifiedError := getVal(i); ok {
					if errIndex >= 0 {
						return nil, errIndex, specifiedError
					}
					patt := compoundValueStack[stackIndex].(*ObjectPattern)
					if patt.entryPatterns == nil {
						patt.entryPatterns = map[string]Pattern{}
					}
					patt.entryPatterns[key] = toPattern(val)

					if isOptionalProp {
						if patt.optionalEntries == nil {
							patt.optionalEntries = map[string]struct{}{}
						}
						patt.optionalEntries[key] = struct{}{}
					}

					objectKeyStack[stackIndex] = ""
					state = rstateObjectPatternComma
					continue
				}
			case DictVal:
				key := dictKeyStack[stackIndex]
				if key == nil {
					state = rstateDictComma
					continue
				}
				if val, errIndex, ok, specifiedError := getVal(i); ok {
					if errIndex >= 0 {
						return nil, errIndex, specifiedError
					}
					keyRepr := string(GetRepresentation(key, ctx)) // representation is context-dependent -> possible issues
					dict := compoundValueStack[stackIndex].(*Dictionary)
					dict.keys[keyRepr] = key
					dict.entries[keyRepr] = val
					dictKeyStack[stackIndex] = nil
					state = rstateDictComma
					continue
				}
			case LstVal:
				if val, errIndex, ok, specifiedError := getVal(i); ok {
					if errIndex >= 0 {
						return nil, errIndex, specifiedError
					}
					list := compoundValueStack[stackIndex].(*List)

					list.underlyingList.append(nil, val)
					state = rstateListComma
					continue
				}
				state = rstateListComma
				continue
			case TupleVal:
				if val, errIndex, ok, specifiedError := getVal(i); ok {
					if errIndex >= 0 {
						return nil, errIndex, specifiedError
					}
					tuple := compoundValueStack[stackIndex].(*Tuple)

					tuple.elements = append(tuple.elements, val)
					state = rstateTupleComma
					continue
				}
				state = rstateTupleComma
				continue
			case ListPatternVal:
				if val, errIndex, ok, specifiedError := getVal(i); ok {
					if errIndex >= 0 {
						return nil, errIndex, specifiedError
					}
					pattern := compoundValueStack[stackIndex].(*ListPattern)

					pattern.elementPatterns = append(pattern.elementPatterns, toPattern(val))
					state = rstateListPatternComma
					continue
				}
				state = rstateListPatternComma
				continue
			case PatternCallVal:
				if val, errIndex, ok, specifiedError := getVal(i); ok {
					if errIndex >= 0 {
						return nil, errIndex, specifiedError
					}

					callArguments[len(callArguments)-1] = append(callArguments[len(callArguments)-1], val)
					state = rstatePatternCallComma
					continue
				}
				state = rstatePatternCallComma
				continue
			case CallVal:
				if val, errIndex, ok, specifiedError := getVal(i); ok {
					if errIndex >= 0 {
						return nil, errIndex, specifiedError
					}

					callArguments[len(callArguments)-1] = append(callArguments[len(callArguments)-1], val)
					state = rstateCallComma
					continue
				}
				state = rstateCallComma
				continue
			case UdataHiearchyEntryVal:
				entry := compoundValueStack[stackIndex].(*UDataHiearchyEntry)

				if entry.Value == nil {
					if val, errIndex, ok, specifiedError := getVal(i); ok {
						if errIndex >= 0 {
							return nil, errIndex, specifiedError
						}

						entry.Value = val
					}
				}

				switch p := compoundValueStack[stackIndex-1].(type) {
				case *UData:
					if entry.Value != nil {
						p.HiearchyEntries = append(p.HiearchyEntries, *entry)
					}
					state = rstateUdataBodyComma
				case *UDataHiearchyEntry:
					if entry.Value != nil {
						p.Children = append(p.Children, *entry)
					}
					state = rstateUdataHiearchyEntryBodyComma
				}

				*entry = UDataHiearchyEntry{} //reset
				hieararchyEntryHasBraces[stackIndex] = false
				continue
			case KLstVal:
				_state := state

				if state == rstateFinishedAtom {
					_state = prevAtomState
					prevAtomState = -1
				}

				switch _state {
				case rstateIdentLike:
					var end int = i
					if atomEndIndex > 0 {
						end = atomEndIndex
					}
					compoundValueStack[stackIndex] = append(compoundValueStack[stackIndex].(KeyList), string(b[atomStartIndex:end]))
					atomStartIndex = -1
					atomEndIndex = -1
					state = rstateKeyListComma
					continue
				default:
					state = rstateKeyListComma
					continue
				}
			}

			return nil, i, nil
		case '[':
			switch state {
			case rstateInit,
				rstateObjectColon, rstateObjOpeningBrace,
				rstateRecordColon, rstateRecordOpeningBrace,
				rstateObjPatternColon, rstateObjPatternOpeningBrace,
				rstateDictOpeningBrace, rstateDictColon,
				rstateListOpeningBracket, rstateListComma,
				rstateTupleOpeningBracket, rstateTupleComma,
				rstateListPatternOpeningBracket, rstateListPatternComma,
				rstatePatternCallOpeningParen, rstatePatternCallComma,
				rstateCallOpeningParen, rstateCallComma,
				rstatePatternConvOpeningParen,
				rstateUdata, rstateUdataOpeningBrace, rstateUdataBodyComma, rstateUdataHiearchyEntryOpeningBrace, rstateUdataHiearchyEntryBodyComma:

				if inPattern[len(inPattern)-1] && state != rstatePercent {
					return nil, i, nil
				}

				state = rstateListOpeningBracket
				stackIndex++
				stack[stackIndex] = LstVal
				newList := &List{underlyingList: &ValueList{}}
				compoundValueStack[stackIndex] = newList
			case rstate0x:
				state = rstateByteSliceBytes
			case rstateHash:
				if inPattern[len(inPattern)-1] {
					return nil, i, nil
				}

				atomStartIndex = -1
				state = rstateTupleOpeningBracket
				stackIndex++
				stack[stackIndex] = TupleVal
				compoundValueStack[stackIndex] = &Tuple{}
			case rstatePercent:
				atomStartIndex = -1
				state = rstateListPatternOpeningBracket
				stackIndex++
				stack[stackIndex] = ListPatternVal
				compoundValueStack[stackIndex] = &ListPattern{elementPatterns: []Pattern{}}
				inPattern = append(inPattern, true)
			default:
				return nil, i, nil
			}
		case ']':
			switch state {
			case rstateByteSliceBytes:
				state = rstateByteSliceClosingBracket
				atomEndIndex = i + 1
			default:
				if stack[stackIndex] != LstVal && stack[stackIndex] != TupleVal && stack[stackIndex] != ListPatternVal {
					return nil, i, nil
				}

				switch stack[stackIndex] {
				case LstVal:
					if state != rstateListComma {
						if val, errIndex, ok, specifiedError := getVal(i); ok {
							list := compoundValueStack[stackIndex].(*List)
							if errIndex >= 0 {
								return nil, errIndex, specifiedError
							}
							list.underlyingList.append(nil, val)
						}
					}

					lastCompoundValue = compoundValueStack[stackIndex]
					stack[stackIndex] = NoVal
					stackIndex--
					state = rstateListClosingBracket
				case TupleVal:
					if state != rstateTupleComma {
						if val, errIndex, ok, specifiedError := getVal(i); ok {
							tuple := compoundValueStack[stackIndex].(*Tuple)
							if errIndex >= 0 {
								return nil, errIndex, specifiedError
							}
							tuple.elements = append(tuple.elements, val)
						}
					}

					lastCompoundValue = compoundValueStack[stackIndex]
					stack[stackIndex] = NoVal
					stackIndex--
					state = rstateTupleClosingBracket
				case ListPatternVal:
					if state != rstateListPatternComma {
						if val, errIndex, ok, specifiedError := getVal(i); ok {
							pattern := compoundValueStack[stackIndex].(*ListPattern)
							if errIndex >= 0 {
								return nil, errIndex, specifiedError
							}
							pattern.elementPatterns = append(pattern.elementPatterns, toPattern(val))
						}
					}

					pattern := compoundValueStack[stackIndex].(*ListPattern)
					inPattern = inPattern[:len(inPattern)-1]

					if i < len(b)-2 && b[i+1] == '%' && b[i+2] == '(' { //general element
						if len(pattern.elementPatterns) > 0 {
							return nil, i + 1, nil
						}
						pattern.elementPatterns = nil
						state = rstateListPatternClosingBracket
					} else { //finished
						lastCompoundValue = compoundValueStack[stackIndex]
						stack[stackIndex] = NoVal
						stackIndex--
						state = rstateListPatternClosingBracket
					}
				}
			}
		case '(':
			switch state {
			case rstatePercent:
				switch compoundValueStack[stackIndex].(type) {
				case *ObjectPattern, *ListPattern:
				default:
					return nil, i, nil
				}
			case rstateTupleGeneralElementPercent:
			case rstatePercentAlpha:
				stackIndex++
				stack[stackIndex] = PatternCallVal
				pattern, index, ok, specifiedError := getVal(i)
				if !ok {
					return nil, index, specifiedError
				}
				compoundValueStack[stackIndex] = pattern.(Pattern)
				state = rstatePatternCallOpeningParen
				callArguments = append(callArguments, nil)
				continue
			case rstateIdentLike:
				stackIndex++
				stack[stackIndex] = CallVal

				functionName := string(b[atomStartIndex:i])
				atomStartIndex = -1
				atomEndIndex = -1

				compoundValueStack[stackIndex] = Str(functionName)
				state = rstateCallOpeningParen
				callArguments = append(callArguments, nil)
				continue
			default:
				return nil, i, nil
			}

			atomStartIndex = -1
			state = rstatePatternConvOpeningParen
			inPattern = append(inPattern, false)
		case ')':
			if stack[stackIndex] == PatternCallVal {
				if state != rstatePatternCallComma {
					if val, errIndex, ok, specifiedError := getVal(i); ok {
						if errIndex >= 0 {
							return nil, errIndex, specifiedError
						}
						callArguments[len(callArguments)-1] = append(callArguments[len(callArguments)-1], val)
					}
				}

				state = rstatePatternCallClosingParen
				pattern := compoundValueStack[stackIndex].(Pattern)
				result, err := pattern.Call(callArguments[len(callArguments)-1])
				if err != nil {
					return nil, i, nil
				}

				lastCompoundValue = result
				stack[stackIndex] = NoVal
				stackIndex--
				callArguments = callArguments[:len(callArguments)-1]
				continue
			} else if stack[stackIndex] == CallVal {
				if state != rstateCallComma {
					if val, errIndex, ok, specifiedError := getVal(i); ok {
						if errIndex >= 0 {
							return nil, errIndex, specifiedError
						}
						callArguments[len(callArguments)-1] = append(callArguments[len(callArguments)-1], val)
					}
				}

				state = rstateCallClosingParen
				fn := compoundValueStack[stackIndex].(Str).UnderlyingString()

				callArgs := callArguments[len(callArguments)-1]
				var result Serializable

				switch fn {
				case globalnames.FILEMODE_FN:
					result = FileModeFrom(ctx, callArgs[0])
				default:
					panic(fmt.Errorf("unknown function in representation call: %s", fn))
				}

				lastCompoundValue = result
				stack[stackIndex] = NoVal
				stackIndex--
				callArguments = callArguments[:len(callArguments)-1]
				continue
			}

			switch v := compoundValueStack[stackIndex].(type) {
			case *ObjectPattern:
			case *ListPattern:
				if v.elementPatterns == nil { //finish list pattern
					generalElem, errInd, ok, specifiedError := getVal(i)
					if !ok {
						return nil, errInd, specifiedError
					}
					v.generalElementPattern = toPattern(generalElem)

					lastCompoundValue = compoundValueStack[stackIndex]
					stack[stackIndex] = NoVal
					stackIndex--
					state = rstateListPatternClosingBracket //TODO: set other state ?
				}
			default:
				return nil, i, nil
			}

			inPattern = inPattern[:len(inPattern)-1]
		case ' ', '\t', '\r':
			switch state {
			case rstateDot, rstateTwoDots,
				rstatePercent, rstatePercentAlpha, rstatePercentDot, rstatePercentTwoDots,
				rstateSingleDash, rstateDoubleDash,
				rstateIntDot, rstateFloatE:
				return nil, i, nil
			case rstateUdataHiearchyEntryAfterVal:
				continue
			default:
				if atomStartIndex >= 0 && atomEndIndex < 0 {
					atomEndIndex = i
					prevAtomState = state
					state = rstateFinishedAtom
				}

				if stackIndex >= 0 {
					if stack[stackIndex] == UdataVal {
						udata := compoundValueStack[stackIndex].(*UData)
						if udata.Root == nil {
							val, ind, ok, specifiedError := getVal(i)
							if ind >= 0 {
								return nil, ind, specifiedError
							}
							if ok {
								udata.Root = val
								state = rstateUdataAfterRoot
							}
							goto after
						}
					} else if stack[stackIndex] == UdataHiearchyEntryVal {
						entry := compoundValueStack[stackIndex].(*UDataHiearchyEntry)
						if entry.Value == nil {
							val, ind, ok, specifiedError := getVal(i)
							if ind >= 0 {
								return nil, ind, specifiedError
							}
							if !ok {
								goto after
							}
							entry.Value = val
							state = rstateUdataHiearchyEntryAfterVal
							continue
						}
					}
				}

			after:
			}
		case '\n':
			switch state {
			case rstateObjectColon, rstateRecordColon, rstateObjPatternColon, rstateDictColon:
				return nil, i, nil
			case rstateObjectComma, rstateRecordComma, rstateObjectPatternComma, rstateDictComma, rstateListComma,
				rstateTupleComma, rstateKeyListComma, rstateListPatternComma,
				rstateUdataOpeningBrace, rstateUdataBodyComma, rstateUdataHiearchyEntryOpeningBrace, rstateUdataHiearchyEntryBodyComma:
			default:
				if atomStartIndex >= 0 {
					return nil, i, nil
				}
			}
		case '\'':
			switch state {
			case rstateRune:
				switch i - atomStartIndex {
				case 2:
					if b[i-1] == '\\' {
						continue
					}
				case 3:
					if b[atomStartIndex+1] != '\\' {
						return nil, atomStartIndex + 2, nil
					}
				case 4, 5:
				default:
					return nil, atomStartIndex + 2, nil
				}
				state = rstateClosingSimpleQuote
				atomEndIndex = i + 1
			default:

				if atomEndIndex >= 0 {
					return nil, atomEndIndex, nil
				}
				if atomStartIndex >= 0 || lastCompoundValue != nil {
					return nil, i, nil
				}

				atomStartIndex = i
				state = rstateRune
				prevAtomState = -1
			}

		case '"':
			switch state {
			case rstateString:
				if countPrevBackslashes(b, i)%2 == 0 {
					state = rstateClosingDoubleQuotes
					atomEndIndex = i + 1
				}
			default:
				if state == rstateIdentLike {
					ident := b[atomStartIndex:i]
					atomEndIndex = -1
					atomStartIndex = -1

					switch len(ident) {
					case 5:
						if bytes.Equal(ident, utils.StringAsBytes("Runes")) {
							call = CreateRunesInRepr
						}
					default:
						return nil, i, nil
					}
				}

				if atomEndIndex >= 0 {
					return nil, atomEndIndex, nil
				}
				if atomStartIndex >= 0 || lastCompoundValue != nil {
					return nil, i, nil
				}

				atomStartIndex = i
				state = rstateString
				prevAtomState = -1
			}
		case '`':
			switch state {
			case rstateQuotedPathLike, rstateQuotedPathPatternLike:
				atomEndIndex = i + 1
				if state == rstateQuotedPathLike {
					state = rstateFinishedQuotedPathLike
				} else {
					state = rstateFinishedQuotedPathPatternLike
				}
			case rstatePathLike, rstatePathPatternLike:
				panic(ErrUnreachable)
			default:
				return nil, i, nil
			}
		case '\\':
			switch state {

			default:
				return nil, i, nil
			}
		case '+':
			switch state {
			case rstateIdentLike:
				state = rstateEmailAddressUsername
			case rstateURLPatternInPath, rstateURLPatternInQuery, rstateURLPatternInFragment,
				rstateEmailAddressUsername:
			default:
				return nil, i, nil
			}
		case '@':
			switch state {
			case rstateIdentLike, rstateEmailAddressUsername:
				state = rstateEmailAddress
			case rstateURLPatternInPath, rstateURLPatternInQuery, rstateURLPatternInFragment:
			default:
				return nil, i, nil
			}
		case '#':

			if i < len(b)-1 && (parse.IsCommentFirstSpace(rune(b[i+1]))) { //comment
				if atomStartIndex >= 0 {
					return nil, i, nil
				}

				switch state {
				case rstateObjectColon, rstateRecordColon, rstateDictColon:
					return nil, i, nil
				}

				stateBeforeComment = state
				for commentEnd = i + 1; commentEnd < len(b) && b[commentEnd] != '\n'; commentEnd++ {

				}
			} else {
				switch state {
				case rstateInit,
					rstateObjectColon, rstateObjOpeningBrace, rstateObjectComma,
					rstateRecordColon, rstateRecordOpeningBrace, rstateRecordComma,
					rstateObjPatternColon, rstateObjPatternOpeningBrace, rstateObjectPatternComma,
					rstateDictOpeningBrace, rstateDictColon,
					rstateListOpeningBracket, rstateListComma,
					rstateTupleOpeningBracket, rstateTupleComma,
					rstateListPatternOpeningBracket, rstateListPatternComma,
					rstatePatternCallOpeningParen, rstatePatternCallComma,
					rstateCallOpeningParen, rstateCallComma,
					rstatePatternConvOpeningParen,
					rstateKeyListOpeningBrace, rstateKeyListComma,
					rstateUdata, rstateUdataOpeningBrace, rstateUdataBodyComma, rstateUdataHiearchyEntryOpeningBrace, rstateUdataHiearchyEntryBodyComma:

					if atomStartIndex >= 0 {
						return nil, i, nil
					}

					state = rstateHash
					atomStartIndex = i
				case rstateURLPatternInPath, rstateURLPatternInQuery, rstateURLPatternInFragment:
					state = rstateURLPatternInFragment
				default:
					return nil, i, nil
				}
			}
		default:
			if isAlpha(c) {
				switch state {
				case rstateInit,
					rstateObjectColon, rstateObjOpeningBrace, rstateObjectComma,
					rstateRecordColon, rstateRecordOpeningBrace, rstateRecordComma,
					rstateObjPatternColon, rstateObjPatternOpeningBrace, rstateObjectPatternComma,
					rstateDictOpeningBrace, rstateDictColon,
					rstateTupleOpeningBracket, rstateTupleComma,
					rstateListPatternOpeningBracket, rstateListPatternComma,
					rstatePatternConvOpeningParen,
					rstateKeyListOpeningBrace, rstateKeyListComma,
					rstateUdata, rstateUdataOpeningBrace, rstateUdataBodyComma, rstateUdataHiearchyEntryOpeningBrace, rstateUdataHiearchyEntryBodyComma:

					if atomEndIndex >= 0 {
						return nil, atomEndIndex, nil
					}
					atomStartIndex = i
					state = rstateIdentLike
					prevAtomState = -1
				case rstateHash:
					state = rstateIdentifier
				case rstateDot:
					state = rstatePropertyName
				case rstateInt:
					switch {
					case c == 'x' && (i-atomStartIndex) == 1 && b[i-1] == '0' && i < len(b)-1 && b[i+1] == '[':
						state = rstate0x
					case c == 'y' && i < len(b)-1 && b[i+1] == '-':
						state = rstateDate
					default:
						unitStart = i
						if ind, err := pushQuantityNumber(); ind >= 0 {
							return nil, ind, err
						}
						state = rstateQtyUnit
					}
				case rstateFloatDecimalPart:
					if c == 'e' {
						state = rstateFloatE
					} else {
						unitStart = i
						if ind, err := pushQuantityNumber(); ind >= 0 {
							return nil, ind, err
						}
						state = rstateQtyUnit
					}
				case rstateFloatExponentNumber:
					unitStart = i
					if ind, err := pushQuantityNumber(); ind >= 0 {
						return nil, ind, err
					}
					state = rstateQtyUnit
				case rstateQtyUnit:
					state = rstateQtyUnit
				case rstateRateSlash:
					rateUnitStart = i
					state = rstateRateUnit
				case rstateSingleDash, rstateDoubleDash:
					state = rstateFlagLitName
				case rstatePercent:
					state = rstatePercentAlpha
				case rstateScheme:
					state = rstateHostLike
				case rstateIdentLike:
					if c == 'a' && i-atomStartIndex == len("udata")-1 {
						state = rstateUdataIdent
					}
				case rstateUdataIdent:
					state = rstateIdentLike
				case rstateFlagLitName, rstateString,
					rstateUnquotedPathPatternLike,
					rstatePortSchemeName,
					rstateHostLike,
					rstateURLLike,
					rstatePercentAlpha,
					rstateHostPattern,
					rstateURLPatternInPath,
					rstateURLPatternInQuery,
					rstateURLPatternInFragment,
					rstateEmailAddressUsername, rstateEmailAddress,
					rstateDate,
					rstateIdentifier,
					rstatePropertyName,
					rstateRateUnit:
				default:
					return nil, i, nil
				}
			}

			switch c {
			case '~', '&', '=':
				switch state {
				default:
					return nil, i, nil
				}
			}
		}
	}

	i++

	switch state {
	case rstateInt,
		rstateFloatDecimalPart, rstateFloatExponentNumber,
		rstateIntInclusiveRange,
		rstatePortNumber, rstatePortSchemeName,
		rstateFlagLitName, rstateClosingDoubleQuotes, rstateClosingSimpleQuote, rstateIdentLike,
		rstateByteSliceClosingBracket,
		//paths
		rstatePathLike, rstateUnquotedPathLike, rstateFinishedQuotedPathLike,
		//path patterns
		rstatePathPatternLike, rstateUnquotedPathPatternLike, rstateFinishedQuotedPathPatternLike,
		//hosts
		rstateScheme,
		rstateHostLike,
		//urls
		rstateURLLike,
		//url patterns
		rstateURLPatternInPath, rstateURLPatternInQuery, rstateURLPatternInFragment,
		//host patterns
		rstateHostPattern,
		//email address
		rstateEmailAddress,
		//dates
		rstateDate,
		//quantities & rates
		rstateQtyUnit,
		rstateRateUnit,
		//identifiers
		rstateIdentifier,
		rstatePropertyName,
		//named patterns
		rstatePercentAlpha,
		//
		rstateFinishedAtom:

		if stackIndex != -1 {
			return nil, len(b), nil
		}
		v, ind, err := parseAtom()
		return v, ind, err
	case rstateObjClosingBrace, rstateRecordClosingBrace, rstateDictClosingBrace, rstateListClosingBracket, rstateTupleClosingBracket,
		rstateObjPatternClosingBrace,
		rstateListPatternClosingBracket,
		rstatePatternCallClosingParen,
		rstateCallClosingParen,
		rstateKeyListClosingBrace,
		rstateUdataClosingBrace:
		return lastCompoundValue, -1, nil
	default:
		return nil, len(b), nil
	}

}

func _parseIntRepr(b []byte) (val Int, errorIndex int) {
	i, err := strconv.ParseInt(string(b), 10, 64)
	if err == nil {
		return Int(i), -1
	}
	return -1, len(b)
}

func _parseFloatRepr(b []byte) (val Serializable, errorIndex int) {
	f, err := strconv.ParseFloat(string(b), 64)
	if err == nil {
		return Float(f), -1
	}
	return nil, len(b)
}

func _parsePortRepr(b []byte) (val Serializable, errorIndex int) {

	slashIndex := bytes.IndexRune(b, '/')
	numberEndIndex := slashIndex

	if slashIndex < 0 {
		numberEndIndex = len(b)
	}

	n, err := strconv.ParseUint(string(b[1:numberEndIndex]), 10, 16)
	if err != nil {
		return nil, len(b)
	}

	scheme := NO_SCHEME_SCHEME
	if slashIndex > 0 {
		scheme = string(b[slashIndex+1:])
		if scheme == "" {
			return nil, len(b)
		}
		scheme += "://"
	}

	return Port{
		Number: uint16(n),
		Scheme: Scheme(scheme),
	}, -1
}

func isAlpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isDigit(c byte) bool {
	return (c >= '0' && c <= '9')
}

func countPrevBackslashes(s []byte, i int) int {
	index := i - 1
	count := 0
	for ; index >= 0; index-- {
		if s[index] == '\\' {
			count += 1
		} else {
			break
		}
	}

	return count
}

func isNextForbiddenSpaceCharacter(i int, b []byte) bool {
	r, _ := utf8.DecodeRune(b[i:])

	return parse.IsForbiddenSpaceCharacter(r)
}
