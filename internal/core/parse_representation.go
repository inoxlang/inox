package internal

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	parse "github.com/inox-project/inox/internal/parse"
	"github.com/inox-project/inox/internal/utils"
)

func ParseRepr(ctx *Context, b []byte) (Value, error) {
	v, errIndex := _parseRepr(b, ctx)
	if errIndex < 0 {
		return v, nil
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
	rstateUdataBody
	rstateUdataClosingBrace
	rstateUdataHiearchyEntryOpeningBrace
	rstateUdataHiearchyEntryBody
	rstateUdataHiearchyEntryClosingBrace

	rstatePropertyName

	//email addresses
	rstateEmailAddressUsername
	rstateEmailAddress

	//numbers, quantities
	rstateInt
	rstateFloatDot
	rstateFloatDecimalPart
	rstateFloatE
	rstateFloatExponentNumber
	rstatePortNumber
	rstatePortSchemeName

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

	//dict
	rstateDictOpeningBrace
	rstateDictColon
	rstateDictClosingBrace
	rstateDictComma

	//list
	rstateListOpeningBracket
	rstateListClosingBracket
	rstateListComma

	//key list
	rstateKeyListOpeningBrace
	rstateKeyListComma
	rstateKeyListClosingBrace

	//byte slice
	rstate0x
	rstateByteSliceBytes
	rstateByteSliceClosingBracket
)

type CompoundValueKind int

const (
	NoVal CompoundValueKind = iota
	ObjVal
	LstVal
	KLstVal
	DictVal
	RecordVal
	UdataVal
	UdataHiearchyEntryVal
)

type InReprCall int

const (
	CreateRunesInRepr InReprCall = iota + 1
)

func _parseRepr(b []byte, ctx *Context) (val Value, errorIndex int) {

	if len(b) == 0 {
		return nil, 0
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
		compoundValueStack       [stackHeight]Value
		objectKeyStack           [stackHeight]string
		dictKeyStack             [stackHeight]Value
		hieararchyEntryChildren  [stackHeight][]Value
		hieararchyEntryHasBraces [stackHeight]bool
		byteSliceDigits          []byte
		quantityValues           []float64
		quantityUnits            []string
		lastCompoundValue        Value
		i                        = -1
		c                        = byte(0)
	)

	defer func() {
		if v := recover(); v != nil {
			val = nil
			errorIndex = i
		}
	}()

	parseAtom := func() (Value, int) {
		var end = i

		if atomEndIndex > 0 {
			end = atomEndIndex
		}

		atomBytes := b[atomStartIndex:end]

		var v Value
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
				return nil, atomStartIndex + len(atomBytes)
			}

			hasGlobbing := false

			for i, c := range b {
				if (c == '[' || c == '*' || c == '?') && countPrevBackslashes(b, i)%2 == 0 {
					hasGlobbing = true
					break
				}
			}

			if isPrefixPattern && hasGlobbing {
				return nil, atomStartIndex + len(atomBytes)
			}
		}

		if index >= 0 {
			return nil, atomStartIndex + index
		}

		atomStartIndex = -1
		atomEndIndex = -1
		return v, -1
	}

	pushQuantityNumber := func() (errIndex int) {

		if len(quantityValues) == 0 {
			quantityRateStart = atomStartIndex
		}

		number, errInd := parseAtom()
		if errInd >= 0 {
			return errInd
		}
		switch n := number.(type) {
		case Float:
			quantityValues = append(quantityValues, float64(n))
		case Int:
			quantityValues = append(quantityValues, float64(n))
		}

		atomStartIndex = quantityRateStart
		return -1
	}

	getVal := func(i int) (Value, int, bool) {
		switch state {
		case rstateInt,
			rstateFloatDecimalPart, rstateFloatExponentNumber,
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

			v, ind := parseAtom()
			return v, ind, true
		case rstateObjClosingBrace, rstateRecordClosingBrace, rstateDictClosingBrace, rstateListClosingBracket, rstateKeyListClosingBrace,
			rstateUdataClosingBrace, rstateUdataHiearchyEntryClosingBrace:
			defer func() {
				lastCompoundValue = nil
			}()
			return lastCompoundValue, -1, true
		default:
			return nil, -1, false
		}
	}

	pushUdataHiearchyEntryIfNecessary := func(i int) int {

		if stackIndex >= 0 && stack[stackIndex] == UdataHiearchyEntryVal {
			entry := compoundValueStack[stackIndex].(UDataHiearchyEntry)

			if entry.Value == nil {
				val, ind, ok := getVal(i)
				if ind >= 0 {
					return ind
				}
				if ok { //we store the previous entry
					entry.Value = val
					stack[stackIndex] = NoVal
					hieararchyEntryHasBraces[stackIndex] = false
					stackIndex--

					if stack[stackIndex] == UdataVal {
						udata := compoundValueStack[stackIndex].(*UData)
						udata.HiearchyEntries = append(udata.HiearchyEntries, entry)
						state = rstateUdataBody
					} else {
						hieararchyEntryChildren[stackIndex] = append(hieararchyEntryChildren[stackIndex], entry)
						state = rstateUdataHiearchyEntryBody
					}
				}
			}
		}

		switch state {
		case rstateUdataOpeningBrace, rstateUdataHiearchyEntryOpeningBrace, rstateUdataBody, rstateUdataHiearchyEntryBody,
			rstateUdataHiearchyEntryClosingBrace:
			//we create a new entry
			stackIndex++
			stack[stackIndex] = UdataHiearchyEntryVal
			compoundValueStack[stackIndex] = UDataHiearchyEntry{}
			hieararchyEntryHasBraces[stackIndex] = false
		}
		return -1
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
				return nil, i
			case ' ', '\t', '\r', ']', '}', ')', ',', ':', '|':
			default:
				if isNextForbiddenSpaceCharacter(i, b) {
					return nil, i
				}
				state = rstateUnquotedPathLike
				continue
			}
		case rstatePathPatternLike:
			if atomEndIndex >= 0 {
				return nil, i
			}
			switch c {
			case '`':
				state = rstateQuotedPathPatternLike
				continue
			case '"', '{', '\n':
				return nil, i
			case ' ', '\t', '\r', ']', '}', ')', ',', ':', '|':
			default:
				if isNextForbiddenSpaceCharacter(i, b) {
					return nil, i
				}
				state = rstateUnquotedPathPatternLike
				continue
			}
		case rstateRune:
			switch c {
			case '\n':
				return nil, i
			case '\'':
			default:
				continue
			}
		case rstateString:
			switch c {
			case '\n':
				return nil, i
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
				return nil, i
			}
			switch c {
			case '"', '{', '\n':
				return nil, i
			case ' ', '\t', '\r', ']', '}', ')', ',', ':', '|':
			default:
				if isNextForbiddenSpaceCharacter(i, b) {
					return nil, i
				}
				continue
			}
		case rstateQuotedPathLike, rstateQuotedPathPatternLike:
			if c == '{' || c == '\n' {
				return nil, i
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
					return nil, i
				}
			case '\n':
				return nil, i
			default:
				return nil, atomEndIndex
			}
		}

		switch c {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			switch state {
			case rstateInit,
				rstateObjectColon, rstateObjectComma,
				rstateRecordColon, rstateRecordComma,
				rstateDictOpeningBrace, rstateDictColon,
				rstateListOpeningBracket, rstateListComma,
				rstateUdata, rstateUdataOpeningBrace, rstateUdataBody,
				rstateUdataHiearchyEntryOpeningBrace, rstateUdataHiearchyEntryBody, rstateUdataHiearchyEntryClosingBrace:

				if ind := pushUdataHiearchyEntryIfNecessary(i); ind >= 0 {
					return nil, ind
				}

				if atomEndIndex >= 0 {
					return nil, atomEndIndex
				}

				atomStartIndex = i
				state = rstateInt
				prevAtomState = -1
			case rstateSingleDash:
				if atomEndIndex >= 0 {
					return nil, atomEndIndex
				}
				atomStartIndex = i - 1
				state = rstateInt
				prevAtomState = -1
			case rstateFloatDot:
				state = rstateFloatDecimalPart
			case rstateFloatE:
				state = rstateFloatExponentNumber
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
				rstatePortNumber, rstatePortSchemeName,
				rstateString,
				rstateUnquotedPathPatternLike,
				rstateHostLike,
				rstateHostPattern,
				rstateURLPatternInPath, rstateURLPatternInQuery, rstateURLPatternInFragment,
				rstateEmailAddress,
				rstateDate,
				rstateIdentifier,
				rstatePropertyName:
			default:
				return nil, i
			}
		case '-':
			switch state {
			case rstateInit,
				rstateObjectColon, rstateObjectComma,
				rstateRecordColon, rstateRecordComma,
				rstateDictOpeningBrace, rstateDictColon,
				rstateListOpeningBracket, rstateListComma:

				if ind := pushUdataHiearchyEntryIfNecessary(i); ind >= 0 {
					return nil, ind
				}

				if atomEndIndex >= 0 {
					return nil, atomEndIndex
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
				return nil, i
			}
		case '_':
			switch state {
			case rstateInit,
				rstateObjectColon, rstateObjectComma,
				rstateRecordColon, rstateRecordComma,
				rstateDictOpeningBrace, rstateDictColon,
				rstateListOpeningBracket, rstateListComma,
				rstateUdata, rstateUdataOpeningBrace, rstateUdataBody, rstateUdataHiearchyEntryOpeningBrace, rstateUdataHiearchyEntryBody:

				if ind := pushUdataHiearchyEntryIfNecessary(i); ind >= 0 {
					return nil, ind
				}

				if atomEndIndex >= 0 {
					return nil, atomEndIndex
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
				return nil, i
			}
		case '.':
			switch state {
			case rstateInit,
				rstateObjectColon, rstateObjectComma,
				rstateRecordColon, rstateRecordComma,
				rstateDictOpeningBrace, rstateDictColon,
				rstateListOpeningBracket, rstateListComma,
				rstateUdata, rstateUdataOpeningBrace, rstateUdataBody, rstateUdataHiearchyEntryOpeningBrace, rstateUdataHiearchyEntryBody:

				if ind := pushUdataHiearchyEntryIfNecessary(i); ind >= 0 {
					return nil, ind
				}

				if atomEndIndex >= 0 {
					return nil, atomEndIndex
				}

				atomStartIndex = i
				state = rstateDot
				prevAtomState = -1
			case rstateInt:
				state = rstateFloatDot
			case rstateDot:
				state = rstateTwoDots
			case rstatePercent:
				state = rstatePercentDot
			case rstatePercentDot:
				state = rstatePercentTwoDots
			case rstateHostPattern:
				if b[i-1] == '.' {
					return nil, i
				}
			case rstateIdentLike:
				state = rstateEmailAddressUsername
			case rstateHostLike, rstateURLLike, rstateURLPatternInPath,
				rstateEmailAddressUsername, rstateEmailAddress:
			default:
				return nil, i
			}
		case '/':
			switch state {
			case rstateInit,
				rstateObjectColon, rstateObjectComma,
				rstateRecordColon, rstateRecordComma,
				rstateDictOpeningBrace, rstateDictColon,
				rstateListOpeningBracket, rstateListComma,
				rstateUdata, rstateUdataOpeningBrace, rstateUdataBody, rstateUdataHiearchyEntryOpeningBrace, rstateUdataHiearchyEntryBody:

				if ind := pushUdataHiearchyEntryIfNecessary(i); ind >= 0 {
					return nil, ind
				}

				if atomEndIndex >= 0 {
					return nil, atomEndIndex
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
				return nil, i
			}
		case '%':
			switch state {
			case rstateInit,
				rstateObjectColon, rstateObjectComma,
				rstateRecordColon, rstateRecordComma,
				rstateDictOpeningBrace, rstateDictColon,
				rstateListOpeningBracket, rstateListComma,
				rstateUdata, rstateUdataOpeningBrace, rstateUdataBody, rstateUdataHiearchyEntryOpeningBrace, rstateUdataHiearchyEntryBody:

				if ind := pushUdataHiearchyEntryIfNecessary(i); ind >= 0 {
					return nil, ind
				}

				if atomEndIndex >= 0 {
					return nil, atomEndIndex
				}

				atomStartIndex = i
				state = rstatePercent
				prevAtomState = -1
			case rstateIdentLike:
				state = rstateEmailAddressUsername
			case rstateHostLike, rstateURLPatternInPath, rstateURLPatternInQuery, rstateURLPatternInFragment,
				rstateEmailAddressUsername:
			case rstateInt, rstateFloatDecimalPart, rstateFloatExponentNumber:
				unitStart = i
				if ind := pushQuantityNumber(); ind >= 0 {
					return nil, ind
				}
				state = rstateQtyUnit
			default:
				return nil, i
			}
		case '?':
			switch state {
			case rstateURLLike,
				rstateHostPattern:
			case rstateURLPatternInPath:
				state = rstateURLPatternInQuery
			default:
				return nil, i
			}
		case '*':
			switch state {
			case rstateURLLike,
				rstateHostPattern,
				rstateURLPatternInPath, rstateURLPatternInQuery, rstateURLPatternInFragment:
			default:
				return nil, i
			}
		case '{':
			switch state {
			case rstateInit,
				rstateObjectColon, rstateObjectComma,
				rstateRecordColon, rstateRecordComma,
				rstateDictOpeningBrace, rstateDictColon,
				rstateListOpeningBracket, rstateListComma,
				rstateUdataOpeningBrace, rstateUdataBody, rstateUdataHiearchyEntryOpeningBrace, rstateUdataHiearchyEntryBody:

				if ind := pushUdataHiearchyEntryIfNecessary(i); ind >= 0 {
					return nil, ind
				}

				state = rstateObjOpeningBrace
				stackIndex++
				stack[stackIndex] = ObjVal
				compoundValueStack[stackIndex] = &Object{}
			case rstateDot:
				if atomEndIndex >= 0 {
					return nil, atomEndIndex
				}
				atomStartIndex = -1
				state = rstateKeyListOpeningBrace
				stackIndex++
				stack[stackIndex] = KLstVal
				compoundValueStack[stackIndex] = make(KeyList, 0)
			case rstateColon:
				state = rstateDictOpeningBrace
				stackIndex++
				stack[stackIndex] = DictVal
				compoundValueStack[stackIndex] = NewDictionary(nil)
			case rstateHash:
				atomStartIndex = -1
				state = rstateRecordOpeningBrace
				stackIndex++
				stack[stackIndex] = RecordVal
				compoundValueStack[stackIndex] = &Record{}
			case rstateUdata, rstateUdataAfterRoot:
				state = rstateUdataOpeningBrace
			default:
				switch stack[stackIndex] {
				case UdataVal:
					val, index, ok := getVal(i)
					if index > 0 {
						return nil, index
					}

					if ok {
						hieararchyEntryChildren[stackIndex] = append(hieararchyEntryChildren[stackIndex], val)
					}
					state = rstateUdataOpeningBrace
				case UdataHiearchyEntryVal:
					val, index, ok := getVal(i)
					if index > 0 {
						return nil, index
					}
					if ok {
						entry := compoundValueStack[stackIndex].(UDataHiearchyEntry)
						entry.Value = val
						compoundValueStack[stackIndex] = entry
					}
					state = rstateUdataHiearchyEntryOpeningBrace
					hieararchyEntryHasBraces[stackIndex] = true
				default:
					return nil, i
				}
			}
		case '}':
			if state == rstateString {
				continue
			}

			var entryWithoutBraces UDataHiearchyEntry

			if stackIndex >= 0 && stack[stackIndex] == UdataHiearchyEntryVal && !hieararchyEntryHasBraces[stackIndex] {
				entryWithoutBraces = compoundValueStack[stackIndex].(UDataHiearchyEntry)
				if val, errIndex, ok := getVal(i); ok {
					if errIndex >= 0 {
						return nil, errIndex
					}

					entryWithoutBraces.Value = val
				}
				stack[stackIndex] = NoVal
				hieararchyEntryChildren[stackIndex] = hieararchyEntryChildren[stackIndex][:0]
				stackIndex--
			}

			switch stack[stackIndex] {
			case ObjVal:
				switch state {
				case rstateObjOpeningBrace, rstateObjectComma:
				case rstateObjectColon:
					return nil, i
				default:
					key := objectKeyStack[stackIndex]
					if key == "" {
						return nil, i
					}
					if val, errIndex, ok := getVal(i); ok {
						if errIndex >= 0 {
							return nil, errIndex
						}
						obj := compoundValueStack[stackIndex].(*Object)
						obj.keys = append(obj.keys, key)
						obj.values = append(obj.values, val)
						objectKeyStack[stackIndex] = ""
					} else {
						return nil, i
					}
				}

				obj := compoundValueStack[stackIndex].(*Object)
				obj.sortProps()
				obj.initPartList(ctx)
				// add handlers before because jobs can mutate the object
				if err := obj.addMessageHandlers(ctx); err != nil {
					return nil, i
				}
				if err := obj.instantiateLifetimeJobs(ctx); err != nil {
					return nil, i
				}
				lastCompoundValue = obj
				stack[stackIndex] = NoVal
				stackIndex--
				state = rstateObjClosingBrace

			case RecordVal:
				switch state {
				case rstateRecordOpeningBrace, rstateRecordComma:
				case rstateRecordColon:
					return nil, i
				default:
					key := objectKeyStack[stackIndex]
					if key == "" {
						return nil, i
					}
					if val, errIndex, ok := getVal(i); ok {
						if errIndex >= 0 {
							return nil, errIndex
						}
						record := compoundValueStack[stackIndex].(*Record)
						record.keys = append(record.keys, key)
						record.values = append(record.values, val)
						objectKeyStack[stackIndex] = ""
					} else {
						return nil, i
					}
				}

				rec := compoundValueStack[stackIndex].(*Record)
				rec.sortProps()
				lastCompoundValue = rec
				stack[stackIndex] = NoVal
				stackIndex--
				state = rstateRecordClosingBrace

			case DictVal:
				switch state {
				case rstateDictOpeningBrace, rstateDictComma:
				case rstateDictColon:
					return nil, i
				default:
					key := dictKeyStack[stackIndex]
					if key == nil {
						return nil, i
					}
					if val, errIndex, ok := getVal(i); ok {
						if errIndex >= 0 {
							return nil, errIndex
						}
						keyRepr := string(GetRepresentation(key, ctx)) // representation is context-dependent -> possible issues
						dict := compoundValueStack[stackIndex].(*Dictionary)
						dict.Keys[keyRepr] = key
						dict.Entries[keyRepr] = val
						dictKeyStack[stackIndex] = nil
					} else {
						return nil, i
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

			case UdataVal:
				switch state {
				case rstateUdataOpeningBrace:
				default:
					compoundValue := compoundValueStack[stackIndex].(*UData)

					if entryWithoutBraces.Value != nil {
						compoundValue.HiearchyEntries = append(compoundValue.HiearchyEntries, entryWithoutBraces)
					} else if val, errIndex, ok := getVal(i); ok {
						if errIndex >= 0 {
							return nil, errIndex
						}

						entry, ok := val.(UDataHiearchyEntry)
						if ok {
							compoundValue.HiearchyEntries = append(compoundValue.HiearchyEntries, entry)
						} else {
							compoundValue.HiearchyEntries = append(compoundValue.HiearchyEntries, UDataHiearchyEntry{
								Value: val,
							})
						}
					}
				}

				lastCompoundValue = compoundValueStack[stackIndex]
				stack[stackIndex] = NoVal
				stackIndex--
				state = rstateUdataClosingBrace

			case UdataHiearchyEntryVal:

				entry := compoundValueStack[stackIndex].(UDataHiearchyEntry)

				if entryWithoutBraces.Value != nil {
					hieararchyEntryChildren[stackIndex] = append(hieararchyEntryChildren[stackIndex], entryWithoutBraces)
				} else if val, errIndex, ok := getVal(i); ok {
					if errIndex >= 0 {
						return nil, errIndex
					}

					hieararchyEntryChildren[stackIndex] = append(hieararchyEntryChildren[stackIndex], val)
				}

				if len(hieararchyEntryChildren) > 0 {
					entry.Children = make([]UDataHiearchyEntry, len(hieararchyEntryChildren[stackIndex]))

					for i, child := range hieararchyEntryChildren[stackIndex] {
						switch v := child.(type) {
						case UDataHiearchyEntry:
							entry.Children[i] = v
						default:
							entry.Children[i] = UDataHiearchyEntry{Value: v}
						}
					}
				}

				compoundValueStack[stackIndex] = entry

				lastCompoundValue = compoundValueStack[stackIndex]
				hieararchyEntryChildren[stackIndex] = hieararchyEntryChildren[stackIndex][:0]
				stack[stackIndex] = NoVal
				stackIndex--
				state = rstateUdataHiearchyEntryClosingBrace
			default:
				return nil, i
			}

		case ':':
			if prevAtomState == rstateIdentLike {
				state = prevAtomState
				prevAtomState = -1
			}

			switch state {
			case rstateInit,
				rstateObjectColon,
				rstateRecordColon,
				rstateDictOpeningBrace, rstateDictColon,
				rstateListOpeningBracket, rstateListComma:
				state = rstateColon
			case rstateUdataOpeningBrace, rstateUdataBody:
				return nil, i
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
						return nil, i + 2
					}
				}

				if objectKeyStack[stackIndex] != "" {
					return nil, i
				}
				var end int = i
				if atomEndIndex > 0 {
					end = atomEndIndex
				}
				objectKeyStack[stackIndex] = string(b[atomStartIndex:end])
				atomStartIndex = -1
				atomEndIndex = -1
				state = rstateObjectColon
			case rstatePercent:
				state = rstateURLPatternNoPostSchemeSlash
			case rstatePercentAlpha:
				scheme := b[atomStartIndex+1 : i]
				if !parse.IsSupportedSchemeName(string(scheme)) {
					if len(scheme) > parse.MAX_SCHEME_NAME_LEN {
						return nil, i
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
						return nil, i
					}
					if val, errIndex, ok := getVal(i); ok {
						if errIndex >= 0 {
							return nil, errIndex
						}
						switch v := val.(type) {
						case Str, Path, PathPattern, URL, URLPattern, Host, HostPattern, Bool:
							dictKeyStack[stackIndex] = v
							state = rstateDictColon
							continue
						}
					}
					return nil, i
				case ObjVal:
					key := objectKeyStack[stackIndex]
					if key != "" {
						return nil, i
					}
					if val, errIndex, ok := getVal(i); ok {
						if errIndex >= 0 {
							return nil, errIndex
						}
						switch v := val.(type) {
						case Str:
							objectKeyStack[stackIndex] = string(v)
							state = rstateObjectColon
							continue
						}
					}
					return nil, i
				case RecordVal:
					key := objectKeyStack[stackIndex]
					if key != "" {
						return nil, i
					}
					if val, errIndex, ok := getVal(i); ok {
						if errIndex >= 0 {
							return nil, errIndex
						}
						switch v := val.(type) {
						case Str:
							objectKeyStack[stackIndex] = string(v)
							state = rstateObjectColon
							continue
						}
					}
					return nil, i
				default:
					if prevAtomState >= 0 {
						return nil, atomEndIndex
					}
					return nil, i
				}
			}

		case ',':
			switch state {
			case rstateHostLike, rstateURLPatternNoPostSchemeSlash,
				rstateObjectComma, rstateRecordComma, rstateDictComma, rstateListComma, rstateKeyListComma:
				continue
			}

			switch stack[stackIndex] {
			case ObjVal:
				key := objectKeyStack[stackIndex]
				if key == "" {
					state = rstateObjectComma
					continue
				}

				if val, errIndex, ok := getVal(i); ok {
					if errIndex >= 0 {
						return nil, errIndex
					}
					obj := compoundValueStack[stackIndex].(*Object)
					obj.keys = append(obj.keys, key)
					obj.values = append(obj.values, val)
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

				if val, errIndex, ok := getVal(i); ok {
					if errIndex >= 0 {
						return nil, errIndex
					}
					record := compoundValueStack[stackIndex].(*Record)
					record.keys = append(record.keys, key)
					record.values = append(record.values, val)
					objectKeyStack[stackIndex] = ""
					state = rstateRecordComma
					continue
				}
			case DictVal:
				key := dictKeyStack[stackIndex]
				if key == nil {
					state = rstateDictComma
					continue
				}
				if val, errIndex, ok := getVal(i); ok {
					if errIndex >= 0 {
						return nil, errIndex
					}
					keyRepr := string(GetRepresentation(key, ctx)) // representation is context-dependent -> possible issues
					dict := compoundValueStack[stackIndex].(*Dictionary)
					dict.Keys[keyRepr] = key
					dict.Entries[keyRepr] = val
					dictKeyStack[stackIndex] = nil
					state = rstateDictComma
					continue
				}
			case LstVal:
				if val, errIndex, ok := getVal(i); ok {
					if errIndex >= 0 {
						return nil, errIndex
					}
					list := compoundValueStack[stackIndex].(*List)

					list.append(nil, val)
					state = rstateListComma
					continue
				}
				state = rstateListComma
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

			return nil, i
		case '[':
			switch state {
			case rstateInit,
				rstateObjectColon, rstateObjOpeningBrace,
				rstateRecordColon, rstateRecordOpeningBrace,
				rstateDictOpeningBrace, rstateDictColon,
				rstateListOpeningBracket, rstateListComma,
				rstateUdata, rstateUdataOpeningBrace, rstateUdataBody, rstateUdataHiearchyEntryOpeningBrace, rstateUdataHiearchyEntryBody:

				if ind := pushUdataHiearchyEntryIfNecessary(i); ind >= 0 {
					return nil, ind
				}

				state = rstateListOpeningBracket
				stackIndex++
				stack[stackIndex] = LstVal
				newList := &List{underylingList: &ValueList{}}
				compoundValueStack[stackIndex] = newList
			case rstate0x:
				state = rstateByteSliceBytes
			default:
				return nil, i
			}
		case ']':
			switch state {
			case rstateByteSliceBytes:
				state = rstateByteSliceClosingBracket
				atomEndIndex = i + 1
			default:
				if stack[stackIndex] != LstVal {
					return nil, i
				}

				if state != rstateListComma {
					if val, errIndex, ok := getVal(i); ok {
						list := compoundValueStack[stackIndex].(*List)
						if errIndex >= 0 {
							return nil, errIndex
						}
						list.append(nil, val)
					}
				}

				lastCompoundValue = compoundValueStack[stackIndex]
				stack[stackIndex] = NoVal
				stackIndex--
				state = rstateListClosingBracket
			}

		case ' ', '\t', '\r':
			switch state {
			case rstateDot, rstateTwoDots,
				rstatePercent, rstatePercentAlpha, rstatePercentDot, rstatePercentTwoDots,
				rstateSingleDash, rstateDoubleDash,
				rstateFloatDot, rstateFloatE:
				return nil, i
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
							val, ind, ok := getVal(i)
							if ind >= 0 {
								return nil, ind
							}
							if ok {
								udata.Root = val
								state = rstateUdataAfterRoot
							}
							goto after
						}
					}

					if stack[stackIndex] == UdataHiearchyEntryVal && !hieararchyEntryHasBraces[stackIndex] {
						entryWithoutBraces := compoundValueStack[stackIndex].(UDataHiearchyEntry)
						if val, errIndex, ok := getVal(i); ok {
							if errIndex >= 0 {
								return nil, errIndex
							}

							entryWithoutBraces.Value = val

							stack[stackIndex] = NoVal
							hieararchyEntryChildren[stackIndex] = hieararchyEntryChildren[stackIndex][:0]
							stackIndex--

							switch stack[stackIndex] {
							case UdataVal:
								compoundValue := compoundValueStack[stackIndex].(*UData)
								compoundValue.HiearchyEntries = append(compoundValue.HiearchyEntries, entryWithoutBraces)
								state = rstateUdataBody
							case UdataHiearchyEntryVal:
								entry := compoundValueStack[stackIndex].(UDataHiearchyEntry)
								hieararchyEntryChildren[stackIndex] = append(hieararchyEntryChildren[stackIndex], entryWithoutBraces)
								compoundValueStack[stackIndex] = entry
								state = rstateUdataHiearchyEntryBody
							}
						}
					} else if entry, ok := lastCompoundValue.(UDataHiearchyEntry); ok &&
						(stack[stackIndex] == UdataVal || stack[stackIndex] == UdataHiearchyEntryVal) {
						lastCompoundValue = nil

						switch stack[stackIndex] {
						case UdataVal:
							compoundValue := compoundValueStack[stackIndex].(*UData)
							compoundValue.HiearchyEntries = append(compoundValue.HiearchyEntries, entry)
							state = rstateUdataBody
						case UdataHiearchyEntryVal:
							entry := compoundValueStack[stackIndex].(UDataHiearchyEntry)
							hieararchyEntryChildren[stackIndex] = append(hieararchyEntryChildren[stackIndex], entry)
							compoundValueStack[stackIndex] = entry
							state = rstateUdataHiearchyEntryBody
						}
					}
				}

			after:
			}
		case '\n':
			switch state {
			case rstateObjectColon, rstateRecordColon, rstateDictColon:
				return nil, i
			case rstateObjectComma, rstateRecordComma, rstateDictComma, rstateListComma, rstateKeyListComma,
				rstateUdataOpeningBrace, rstateUdataBody, rstateUdataHiearchyEntryOpeningBrace, rstateUdataHiearchyEntryBody:
			default:
				if stackIndex >= 0 {
					if stack[stackIndex] == UdataHiearchyEntryVal && !hieararchyEntryHasBraces[stackIndex] {
						entryWithoutBraces := compoundValueStack[stackIndex].(UDataHiearchyEntry)
						if val, errIndex, ok := getVal(i); ok {
							if errIndex >= 0 {
								return nil, errIndex
							}

							entryWithoutBraces.Value = val

							stack[stackIndex] = NoVal
							hieararchyEntryChildren[stackIndex] = hieararchyEntryChildren[stackIndex][:0]
							stackIndex--

							switch stack[stackIndex] {
							case UdataVal:
								compoundValue := compoundValueStack[stackIndex].(*UData)
								compoundValue.HiearchyEntries = append(compoundValue.HiearchyEntries, entryWithoutBraces)
								state = rstateUdataBody
							case UdataHiearchyEntryVal:
								entry := compoundValueStack[stackIndex].(UDataHiearchyEntry)
								hieararchyEntryChildren[stackIndex] = append(hieararchyEntryChildren[stackIndex], entryWithoutBraces)
								compoundValueStack[stackIndex] = entry
								state = rstateUdataHiearchyEntryBody
							}
						}
						goto newline_handling_end
					} else if entry, ok := lastCompoundValue.(UDataHiearchyEntry); ok &&
						(stack[stackIndex] == UdataVal || stack[stackIndex] == UdataHiearchyEntryVal) {
						lastCompoundValue = nil

						switch stack[stackIndex] {
						case UdataVal:
							compoundValue := compoundValueStack[stackIndex].(*UData)
							compoundValue.HiearchyEntries = append(compoundValue.HiearchyEntries, entry)
							state = rstateUdataBody
						case UdataHiearchyEntryVal:
							entry := compoundValueStack[stackIndex].(UDataHiearchyEntry)
							hieararchyEntryChildren[stackIndex] = append(hieararchyEntryChildren[stackIndex], entry)
							compoundValueStack[stackIndex] = entry
							state = rstateUdataHiearchyEntryBody
						}
						goto newline_handling_end
					}
				}

				if atomStartIndex >= 0 {
					return nil, i
				}

			newline_handling_end:
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
						return nil, atomStartIndex + 2
					}
				case 4, 5:
				default:
					return nil, atomStartIndex + 2
				}
				state = rstateClosingSimpleQuote
				atomEndIndex = i + 1
			default:
				if ind := pushUdataHiearchyEntryIfNecessary(i); ind >= 0 {
					return nil, ind
				}

				if atomEndIndex >= 0 {
					return nil, atomEndIndex
				}
				if atomStartIndex >= 0 || lastCompoundValue != nil {
					return nil, i
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
				if ind := pushUdataHiearchyEntryIfNecessary(i); ind >= 0 {
					return nil, ind
				} else if state == rstateIdentLike {
					ident := b[atomStartIndex:i]
					atomEndIndex = -1
					atomStartIndex = -1

					switch len(ident) {
					case 5:
						if bytes.Equal(ident, utils.StringAsBytes("Runes")) {
							call = CreateRunesInRepr
						}
					default:
						return nil, i
					}
				}

				if atomEndIndex >= 0 {
					return nil, atomEndIndex
				}
				if atomStartIndex >= 0 || lastCompoundValue != nil {
					return nil, i
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
				return nil, i
			}
		case '\\':
			switch state {

			default:
				return nil, i
			}
		case '+':
			switch state {
			case rstateIdentLike:
				state = rstateEmailAddressUsername
			case rstateURLPatternInPath, rstateURLPatternInQuery, rstateURLPatternInFragment,
				rstateEmailAddressUsername:
			default:
				return nil, i
			}
		case '@':
			switch state {
			case rstateIdentLike, rstateEmailAddressUsername:
				state = rstateEmailAddress
			case rstateURLPatternInPath, rstateURLPatternInQuery, rstateURLPatternInFragment:
			default:
				return nil, i
			}
		case '#':

			if i < len(b)-1 && (b[i+1] == ' ' || b[i+1] == '\t' || b[i+1] == '\r') { //comment
				if atomStartIndex >= 0 {
					return nil, i
				}

				switch state {
				case rstateObjectColon, rstateRecordColon, rstateDictColon:
					return nil, i
				}

				stateBeforeComment = state
				for commentEnd = i + 1; commentEnd < len(b) && b[commentEnd] != '\n'; commentEnd++ {

				}
			} else {
				switch state {
				case rstateInit,
					rstateObjectColon, rstateObjOpeningBrace, rstateObjectComma,
					rstateRecordColon, rstateRecordOpeningBrace, rstateRecordComma,
					rstateDictOpeningBrace, rstateDictColon,
					rstateKeyListOpeningBrace, rstateKeyListComma,
					rstateUdata, rstateUdataOpeningBrace, rstateUdataBody, rstateUdataHiearchyEntryOpeningBrace, rstateUdataHiearchyEntryBody:

					if ind := pushUdataHiearchyEntryIfNecessary(i); ind >= 0 {
						return nil, ind
					}

					if atomStartIndex >= 0 {
						return nil, i
					}

					state = rstateHash
					atomStartIndex = i
				case rstateURLPatternInPath, rstateURLPatternInQuery, rstateURLPatternInFragment:
					state = rstateURLPatternInFragment
				default:
					return nil, i
				}
			}
		default:
			if isAlpha(c) {
				switch state {
				case rstateInit,
					rstateObjectColon, rstateObjOpeningBrace, rstateObjectComma,
					rstateRecordColon, rstateRecordOpeningBrace, rstateRecordComma,
					rstateDictOpeningBrace, rstateDictColon,
					rstateKeyListOpeningBrace, rstateKeyListComma,
					rstateUdata, rstateUdataOpeningBrace, rstateUdataBody, rstateUdataHiearchyEntryOpeningBrace, rstateUdataHiearchyEntryBody:

					if ind := pushUdataHiearchyEntryIfNecessary(i); ind >= 0 {
						return nil, ind
					}

					if atomEndIndex >= 0 {
						return nil, atomEndIndex
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
						if ind := pushQuantityNumber(); ind >= 0 {
							return nil, ind
						}
						state = rstateQtyUnit
					}
				case rstateFloatDecimalPart:
					if c == 'e' {
						state = rstateFloatE
					} else {
						unitStart = i
						if ind := pushQuantityNumber(); ind >= 0 {
							return nil, ind
						}
						state = rstateQtyUnit
					}
				case rstateFloatExponentNumber:
					unitStart = i
					if ind := pushQuantityNumber(); ind >= 0 {
						return nil, ind
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
					return nil, i
				}
			}

			switch c {
			case '~', '&', '=':
				switch state {
				default:
					return nil, i
				}
			}
		}
	}

	i++

	switch state {
	case rstateInt,
		rstateFloatDecimalPart, rstateFloatExponentNumber,
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
			return nil, len(b)
		}
		v, ind := parseAtom()
		return v, ind
	case rstateObjClosingBrace, rstateRecordClosingBrace, rstateDictClosingBrace, rstateListClosingBracket, rstateKeyListClosingBrace,
		rstateUdataClosingBrace:
		return lastCompoundValue, -1
	default:
		return nil, len(b)
	}

}

func _parseIntRepr(b []byte) (val Value, errorIndex int) {
	i, err := strconv.ParseInt(string(b), 10, 64)
	if err == nil {
		return Int(i), -1
	}
	return nil, len(b)
}

func _parseFloatRepr(b []byte) (val Value, errorIndex int) {
	f, err := strconv.ParseFloat(string(b), 64)
	if err == nil {
		return Float(f), -1
	}
	return nil, len(b)
}

func _parsePortRepr(b []byte) (val Value, errorIndex int) {

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
