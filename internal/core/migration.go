package core

import (
	"errors"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ = []MigrationAwarePattern{
		(*ObjectPattern)(nil), (*RecordPattern)(nil), (*ListPattern)(nil),
	}

	_ = []MigrationCapable{
		(*Object)(nil), (*Record)(nil), //(*List)(nil),
	}
)

var (
	ErrInvalidMigrationPseudoPath = errors.New("invalid migration pseudo path")
)

// TODO: improve name
type MigrationAwarePattern interface {
	Pattern
	GetMigrationOperations(ctx *Context, next Pattern, pseudoPath string) ([]MigrationOp, error)
}

type MigrationCapable interface {
	Serializable

	//Migrate recursively perfoms a migration, it calls the passed handlers,
	//if a migration operation is a deletion of the MigrationCapable nil should be returned.
	//This method should be called before any change to the MigrationCapable.
	Migrate(ctx *Context, key Path, migration *InstanceMigrationArgs) (Value, error)
}

type MigrationOp interface {
	GetPseudoPath() string
	ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) symbolic.MigrationOp
}

type MigrationOpKind int

const (
	RemovalMigrationOperation MigrationOpKind = iota + 1
	ReplacementMigrationOperation
	InclusionMigrationOperation
	InitializationMigrationOperation
)

type MigrationMixin struct {
	PseudoPath string
}

func (m MigrationMixin) GetPseudoPath() string {
	return m.PseudoPath
}

type ReplacementMigrationOp struct {
	Current, Next Pattern
	MigrationMixin
}

func (op ReplacementMigrationOp) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) symbolic.MigrationOp {
	return symbolic.ReplacementMigrationOp{
		Current:        utils.Must(op.Current.ToSymbolicValue(ctx, encountered)).(symbolic.Pattern),
		Next:           utils.Must(op.Next.ToSymbolicValue(ctx, encountered)).(symbolic.Pattern),
		MigrationMixin: symbolic.MigrationMixin{PseudoPath: op.PseudoPath},
	}
}

type RemovalMigrationOp struct {
	Value Pattern
	MigrationMixin
}

func (op RemovalMigrationOp) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) symbolic.MigrationOp {
	return symbolic.RemovalMigrationOp{
		Value:          utils.Must(op.Value.ToSymbolicValue(ctx, encountered)).(symbolic.Pattern),
		MigrationMixin: symbolic.MigrationMixin{PseudoPath: op.PseudoPath},
	}
}

type NillableInitializationMigrationOp struct {
	Value Pattern
	MigrationMixin
}

func (op NillableInitializationMigrationOp) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) symbolic.MigrationOp {
	return symbolic.NillableInitializationMigrationOp{
		Value:          utils.Must(op.Value.ToSymbolicValue(ctx, encountered)).(symbolic.Pattern),
		MigrationMixin: symbolic.MigrationMixin{PseudoPath: op.PseudoPath},
	}
}

type InclusionMigrationOp struct {
	Value    Pattern
	Optional bool
	MigrationMixin
}

func (op InclusionMigrationOp) ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) symbolic.MigrationOp {
	return symbolic.InclusionMigrationOp{
		Value:          utils.Must(op.Value.ToSymbolicValue(ctx, encountered)).(symbolic.Pattern),
		MigrationMixin: symbolic.MigrationMixin{PseudoPath: op.PseudoPath},
	}
}

func (patt *ObjectPattern) GetMigrationOperations(ctx *Context, next Pattern, pseudoPath string) (migrations []MigrationOp, _ error) {
	nextObject, ok := next.(*ObjectPattern)
	if !ok {
		return []MigrationOp{ReplacementMigrationOp{Current: patt, Next: next, MigrationMixin: MigrationMixin{PseudoPath: pseudoPath}}}, nil
	}

	if patt.entryPatterns == nil {
		return []MigrationOp{ReplacementMigrationOp{Current: patt, Next: next, MigrationMixin: MigrationMixin{PseudoPath: pseudoPath}}}, nil
	}

	if nextObject.entryPatterns == nil {
		return nil, nil
	}

	removedPropertyCount := 0

	for propName := range patt.entryPatterns {
		_, presentInOther := nextObject.entryPatterns[propName]
		if !presentInOther {
			removedPropertyCount++
		}
	}

	if removedPropertyCount == len(patt.entryPatterns) && removedPropertyCount > 0 && len(nextObject.entryPatterns) != 0 {
		return []MigrationOp{ReplacementMigrationOp{Current: patt, Next: next, MigrationMixin: MigrationMixin{PseudoPath: pseudoPath}}}, nil
	}

	for propName, propPattern := range patt.entryPatterns {
		propPseudoPath := filepath.Join(pseudoPath, propName)
		_, isOptional := patt.optionalEntries[propName]
		_, isOptionalInOther := nextObject.optionalEntries[propName]

		nextPropPattern, presentInOther := nextObject.entryPatterns[propName]

		if !presentInOther {
			migrations = append(migrations, RemovalMigrationOp{
				Value:          propPattern,
				MigrationMixin: MigrationMixin{propPseudoPath},
			})
			continue
		}

		list, err := GetMigrationOperations(ctx, propPattern, nextPropPattern, propPseudoPath)
		if err != nil {
			return nil, err
		}

		if len(list) == 0 && isOptional && !isOptionalInOther {
			list = append(list, NillableInitializationMigrationOp{
				Value:          propPattern,
				MigrationMixin: MigrationMixin{propPseudoPath},
			})
		}
		migrations = append(migrations, list...)
	}

	for propName, nextPropPattern := range nextObject.entryPatterns {
		_, presentInCurrent := patt.entryPatterns[propName]

		if presentInCurrent {
			//already handled
			continue
		}
		propPseudoPath := filepath.Join(pseudoPath, propName)
		_, isOptional := nextObject.optionalEntries[propName]

		migrations = append(migrations, InclusionMigrationOp{
			Value:          nextPropPattern,
			Optional:       isOptional,
			MigrationMixin: MigrationMixin{propPseudoPath},
		})
	}

	return migrations, nil
}

func (patt *RecordPattern) GetMigrationOperations(ctx *Context, next Pattern, pseudoPath string) (migrations []MigrationOp, _ error) {
	nextRecord, ok := next.(*RecordPattern)
	if !ok {
		return []MigrationOp{ReplacementMigrationOp{Current: patt, Next: next, MigrationMixin: MigrationMixin{PseudoPath: pseudoPath}}}, nil
	}

	if patt.entryPatterns == nil {
		return []MigrationOp{ReplacementMigrationOp{Current: patt, Next: next, MigrationMixin: MigrationMixin{PseudoPath: pseudoPath}}}, nil
	}

	if nextRecord.entryPatterns == nil {
		return nil, nil
	}

	removedPropertyCount := 0

	for propName := range patt.entryPatterns {
		_, presentInOther := nextRecord.entryPatterns[propName]
		if !presentInOther {
			removedPropertyCount++
		}
	}

	if removedPropertyCount == len(patt.entryPatterns) && removedPropertyCount > 0 && len(nextRecord.entryPatterns) != 0 {
		return []MigrationOp{ReplacementMigrationOp{Current: patt, Next: next, MigrationMixin: MigrationMixin{PseudoPath: pseudoPath}}}, nil
	}

	for propName, propPattern := range patt.entryPatterns {
		propPseudoPath := filepath.Join(pseudoPath, propName)
		_, isOptional := patt.optionalEntries[propName]
		_, isOptionalInOther := nextRecord.optionalEntries[propName]

		nextPropPattern, presentInOther := nextRecord.entryPatterns[propName]

		if !presentInOther {
			migrations = append(migrations, RemovalMigrationOp{
				Value:          propPattern,
				MigrationMixin: MigrationMixin{propPseudoPath},
			})
			continue
		}

		list, err := GetMigrationOperations(ctx, propPattern, nextPropPattern, propPseudoPath)
		if err != nil {
			return nil, err
		}

		if len(list) == 0 && isOptional && !isOptionalInOther {
			list = append(list, NillableInitializationMigrationOp{
				Value:          propPattern,
				MigrationMixin: MigrationMixin{propPseudoPath},
			})
		}
		migrations = append(migrations, list...)
	}

	for propName, nextPropPattern := range nextRecord.entryPatterns {
		_, presentInCurrent := patt.entryPatterns[propName]

		if presentInCurrent {
			//already handled
			continue
		}
		propPseudoPath := filepath.Join(pseudoPath, propName)
		_, isOptional := nextRecord.optionalEntries[propName]

		migrations = append(migrations, InclusionMigrationOp{
			Value:          nextPropPattern,
			Optional:       isOptional,
			MigrationMixin: MigrationMixin{propPseudoPath},
		})
	}

	return migrations, nil
}

func (patt *ListPattern) GetMigrationOperations(ctx *Context, next Pattern, pseudoPath string) (migrations []MigrationOp, _ error) {
	nextList, ok := next.(*ListPattern)
	if !ok {
		return []MigrationOp{ReplacementMigrationOp{Current: patt, Next: next, MigrationMixin: MigrationMixin{PseudoPath: pseudoPath}}}, nil
	}

	anyElemPseudoPath := filepath.Join(pseudoPath, "*")

	if patt.generalElementPattern != nil {
		if nextList.generalElementPattern != nil {
			return GetMigrationOperations(ctx, patt.generalElementPattern, nextList.generalElementPattern, anyElemPseudoPath)
		}
		return []MigrationOp{ReplacementMigrationOp{Current: patt, Next: next, MigrationMixin: MigrationMixin{PseudoPath: pseudoPath}}}, nil
	}
	//else pattern has element patterns

	if nextList.generalElementPattern != nil {
		//add operation for each current element that does not match the next general element pattern
		for i, currentElemPattern := range patt.elementPatterns {
			elemPseudoPath := filepath.Join(pseudoPath, strconv.Itoa(i))

			list, err := GetMigrationOperations(ctx, currentElemPattern, nextList.generalElementPattern, elemPseudoPath)
			if err != nil {
				return nil, err
			}
			migrations = append(migrations, list...)
		}
	} else {
		if len(nextList.elementPatterns) != len(patt.elementPatterns) {
			return []MigrationOp{ReplacementMigrationOp{Current: patt, Next: next, MigrationMixin: MigrationMixin{PseudoPath: pseudoPath}}}, nil
		}

		//add operation for each current element that does not match the element pattern at the corresponding index
		for i, nextElemPattern := range nextList.elementPatterns {
			elemPseudoPath := filepath.Join(pseudoPath, strconv.Itoa(i))
			currentElemPattern := patt.elementPatterns[i]

			list, err := GetMigrationOperations(ctx, currentElemPattern, nextElemPattern, elemPseudoPath)
			if err != nil {
				return nil, err
			}
			migrations = append(migrations, list...)
		}
	}

	return migrations, nil
}

func GetTopLevelEntitiesMigrationOperations(ctx *Context, current, next *ObjectPattern) ([]MigrationOp, error) {
	return GetMigrationOperations(ctx, current, next, "/")
}

func GetMigrationOperations(ctx *Context, current, next Pattern, pseudoPath string) ([]MigrationOp, error) {
	if pseudoPath != "/" && pseudoPath[len(pseudoPath)-1] == '/' {
		return nil, ErrInvalidMigrationPseudoPath
	}

	for _, segment := range strings.Split(pseudoPath, "/") {
		if strings.ContainsAny(segment, "*?[]") && segment != "*" {
			return nil, ErrInvalidMigrationPseudoPath
		}
	}

	if current == next || isSubType(current, next, ctx, map[uintptr]symbolic.Value{}) {
		return nil, nil
	}

	m1, ok := current.(MigrationAwarePattern)

	if !ok {
		return []MigrationOp{ReplacementMigrationOp{Current: current, Next: next, MigrationMixin: MigrationMixin{PseudoPath: pseudoPath}}}, nil
	}

	return m1.GetMigrationOperations(ctx, next, pseudoPath)
}

func (o *Object) Migrate(ctx *Context, key Path, migration *InstanceMigrationArgs) (Value, error) {
	if o.IsShared() {
		panic(ErrUnreachable)
	}

	if o.mutationCallbacks != nil {
		panic(ErrUnreachable)
	}

	if o.currentTransaction != nil {
		panic(ErrUnreachable)
	}

	if o.jobs != nil {
		panic(ErrNotImplementedYet)
	}

	if ctx.GetTx() != nil {
		panic(ErrUnreachable)
	}

	return migrateObjectOrRecord(ctx, o, true, &o.keys, &o.values, key, migration)
}

func (o *Record) Migrate(ctx *Context, key Path, migration *InstanceMigrationArgs) (Value, error) {
	if ctx.GetTx() != nil {
		panic(ErrUnreachable)
	}

	return migrateObjectOrRecord(ctx, o, false, &o.keys, &o.values, key, migration)
}

func migrateObjectOrRecord(
	ctx *Context, o Value, isObject bool, propKeys *[]string, propValues *[]Serializable,
	key Path, migration *InstanceMigrationArgs) (Value, error) {
	depth := len(GetPathSegments(string(key)))
	migrationHanders := migration.MigrationHandlers
	state := ctx.GetClosestState()

	//a record is immutable so it cannot be updated in-place
	var nextRecordKeys []string
	var nextRecordValues []Serializable
	//note: nextRecordKeys should not be use to check for the presence of a next record because it could be nil
	var nextRecord bool

	for pathPattern, handler := range migrationHanders.Deletions {
		pathPatternSegments := GetPathSegments(string(pathPattern))
		pathPatternDepth := len(pathPatternSegments)

		lastSegment := ""
		if pathPatternDepth == 0 {
			if string(pathPattern) != string(key) {
				panic(ErrUnreachable)
			}
		} else {
			lastSegment = pathPatternSegments[pathPatternDepth-1]
		}

		switch {
		case string(pathPattern) == string(key): //Object deletion
			if handler == nil {
				return nil, nil
			}

			if handler.Function != nil {
				_, err := handler.Function.Call(state, nil, []Value{o}, nil)
				return nil, err
			} else {
				panic(ErrUnreachable)
			}
		case pathPatternDepth == 1+depth: //property deletion
			if lastSegment == "*" {
				if handler != nil {
					if handler.Function != nil {
						_, err := handler.Function.Call(state, nil, []Value{o}, nil)
						return nil, err
					} else {
						panic(ErrUnreachable)
					}
				}

				if o, ok := o.(*Object); ok {
					obj := NewObjectFromMap(nil, ctx)
					obj.url = o.url
					obj.visibilityId = o.visibilityId
					return obj, nil
				}

				rec := NewEmptyRecord()
				rec.visibilityId = o.(*Record).visibilityId
				return rec, nil
			} else {
				propIndex := -1

				for i, key := range *propKeys {
					if key == lastSegment {
						propIndex = i
						break
					}
				}

				if propIndex < 0 {
					return nil, commonfmt.FmtValueAtPathSegmentsDoesNotExist(pathPatternSegments[:pathPatternDepth])
				}

				propValueToRemove := (*propValues)[propIndex]
				if handler != nil {
					if handler.Function != nil {
						_, err := handler.Function.Call(state, nil, []Value{propValueToRemove}, nil)
						return nil, err
					} else {
						panic(ErrUnreachable)
					}
				}

				if isObject {
					*propKeys = slices.Delete(*propKeys, propIndex, propIndex+1)
					*propValues = slices.Delete(*propValues, propIndex, propIndex+1)
				} else {
					if !nextRecord {
						nextRecord = true
						nextRecordKeys = slices.Clone(*propKeys)
						nextRecordValues = slices.Clone(*propValues)
					}
					nextRecordKeys = slices.Delete(nextRecordKeys, propIndex, propIndex+1)
					nextRecordValues = slices.Delete(nextRecordValues, propIndex, propIndex+1)
				}
			}
		case pathPatternDepth > 1+depth: //deletion inside property value
			propertyName := pathPatternSegments[depth]

			propIndex := -1

			for i, key := range *propKeys {
				if key == propertyName {
					propIndex = i
					break
				}
			}

			if propIndex < 0 {
				return nil, commonfmt.FmtValueAtPathSegmentsDoesNotExist(pathPatternSegments[:depth+1])
			}

			propValue := (*propValues)[propIndex]
			migrationCapable, ok := propValue.(MigrationCapable)
			if !ok {
				return nil, commonfmt.FmtValueAtPathSegmentsIsNotMigrationCapable(pathPatternSegments[:depth+1])
			}

			propertyValuePath := "/" + Path(strings.Join(pathPatternSegments[:depth+1], ""))
			nextPropValue, err := migrationCapable.Migrate(ctx, propertyValuePath, &InstanceMigrationArgs{
				NextPattern:       nil,
				MigrationHandlers: migrationHanders.FilterByPrefix(propertyValuePath),
			})
			if err != nil {
				return nil, err
			}

			//if o is a record a new record with the updated property has to be created
			if !isObject {
				if !nextRecord {
					nextRecord = true
					nextRecordKeys = slices.Clone(*propKeys)
					nextRecordValues = slices.Clone(*propValues)
				}
				nextRecordValues[propIndex] = nextPropValue.(Serializable)
			}
		default:
			panic(ErrUnreachable)
		}
	}

	handle := func(pathPattern PathPattern, handler *MigrationOpHandler, kind MigrationOpKind) (outerFunctionResult Value, outerFunctionError error) {
		pathPatternSegments := GetPathSegments(string(pathPattern))
		pathPatternDepth := len(pathPatternSegments)
		lastSegment := ""
		if pathPatternDepth == 0 {
			if string(pathPattern) != string(key) {
				panic(ErrUnreachable)
			}
		} else {
			lastSegment = pathPatternSegments[pathPatternDepth-1]
		}

		switch {
		case string(pathPattern) == string(key): //Object replacement
			if kind != ReplacementMigrationOperation {
				panic(ErrUnreachable)
			}

			if handler.Function != nil {
				return handler.Function.Call(state, nil, []Value{o}, nil)
			} else {
				var initialValue Serializable
				if isObject {
					initialValue = handler.InitialValue.(*Object)
				} else {
					initialValue = handler.InitialValue.(*Record)
				}
				clone, err := RepresentationBasedClone(ctx, initialValue)
				if err != nil {
					return nil, commonfmt.FmtErrWhileCloningValueFor(pathPatternSegments, err)
				}

				return clone, nil
			}
		case pathPatternDepth == 1+depth: //property replacement|inclusion|init
			if lastSegment == "*" {
				panic(ErrUnreachable)
			} else {
				if !isObject && !nextRecord {
					nextRecord = true
					nextRecordKeys = slices.Clone(*propKeys)
					nextRecordValues = slices.Clone(*propValues)
				}

				propIndex := -1

				for i, key := range *propKeys {
					if key == lastSegment {
						propIndex = i
						break
					}
				}

				propertyPathSegments := pathPatternSegments[:pathPatternDepth]

				if propIndex < 0 { //previous value not found
					if kind == ReplacementMigrationOperation {
						return nil, commonfmt.FmtValueAtPathSegmentsDoesNotExist(propertyPathSegments)
					}

					var newPropValue Value
					prevValue := Nil

					if handler.Function != nil {
						val, err := handler.Function.Call(state, nil, []Value{prevValue}, nil)
						if err != nil {
							return nil, commonfmt.FmtErrWhileCallingMigrationHandler(propertyPathSegments, err)
						}
						newPropValue = val
					} else {
						clone, err := RepresentationBasedClone(ctx, handler.InitialValue)
						if err != nil {
							return nil, commonfmt.FmtErrWhileCloningValueFor(propertyPathSegments, err)
						}
						newPropValue = clone
					}
					if isObject {
						*propKeys = append(*propKeys, lastSegment)
						*propValues = append(*propValues, newPropValue.(Serializable))
					} else {
						nextRecordKeys = append(nextRecordKeys, lastSegment)
						nextRecordValues = append(nextRecordValues, newPropValue.(Serializable))
					}
					return nil, nil
				}

				prevPropValue := (*propValues)[propIndex]
				var nextPropValue Value
				var err error

				if handler.Function != nil {
					nextPropValue, err = handler.Function.Call(state, nil, []Value{prevPropValue}, nil)
					if err != nil {
						return nil, commonfmt.FmtErrWhileCallingMigrationHandler(propertyPathSegments, err)
					}
				} else {
					clone, err := RepresentationBasedClone(ctx, handler.InitialValue)
					if err != nil {
						return nil, commonfmt.FmtErrWhileCloningValueFor(propertyPathSegments, err)
					}
					nextPropValue = clone
				}

				if isObject {
					(*propValues)[propIndex] = nextPropValue.(Serializable)
				} else {
					nextRecordValues[propIndex] = nextPropValue.(Serializable)
				}
			}
		case pathPatternDepth > 1+depth: //migration inside property value
			propertyName := pathPatternSegments[depth]
			propertyPathPatternSegments := pathPatternSegments[:depth+1]

			propIndex := -1

			for i, key := range *propKeys {
				if key == propertyName {
					propIndex = i
					break
				}
			}

			if propIndex < 0 {
				return nil, commonfmt.FmtValueAtPathSegmentsDoesNotExist(propertyPathPatternSegments)
			}

			propValue := (*propValues)[propIndex]
			migrationCapable, ok := propValue.(MigrationCapable)
			if !ok {
				return nil, commonfmt.FmtValueAtPathSegmentsIsNotMigrationCapable(propertyPathPatternSegments)
			}

			propertyValuePath := Path("/" + strings.Join(propertyPathPatternSegments, ""))
			nextPropValue, err := migrationCapable.Migrate(ctx, propertyValuePath, &InstanceMigrationArgs{
				NextPattern:       nil,
				MigrationHandlers: migrationHanders.FilterByPrefix(propertyValuePath),
			})
			if err != nil {
				return nil, err
			}

			if isObject {
				(*propValues)[propIndex] = nextPropValue.(Serializable)
			} else { //if o is a record a new record with the updated property has to be created

				if !nextRecord {
					nextRecord = true
					nextRecordKeys = slices.Clone(*propKeys)
					nextRecordValues = slices.Clone(*propValues)
				}
				nextRecordValues[propIndex] = nextPropValue.(Serializable)
			}
		}

		return nil, nil
	}

	for pathPattern, handler := range migrationHanders.Replacements {
		result, err := handle(pathPattern, handler, ReplacementMigrationOperation)
		if err != nil {
			return nil, err
		}
		if result != nil {
			return result, nil
		}
	}

	for pathPattern, handler := range migrationHanders.Inclusions {
		result, err := handle(pathPattern, handler, InclusionMigrationOperation)
		if err != nil {
			return nil, err
		}
		if result != nil {
			return result, nil
		}
	}

	for pathPattern, handler := range migrationHanders.Initializations {
		result, err := handle(pathPattern, handler, InitializationMigrationOperation)
		if err != nil {
			return nil, err
		}
		if result != nil {
			return result, nil
		}
	}

	if nextRecord {
		return NewRecordFromKeyValLists(nextRecordKeys, nextRecordValues), nil
	}
	return o, nil
}

func (list *List) Migrate(ctx *Context, key Path, migration *InstanceMigrationArgs) (Value, error) {
	if list.mutationCallbacks != nil {
		panic(ErrUnreachable)
	}

	if ctx.GetTx() != nil {
		panic(ErrUnreachable)
	}

	return migrateListOrTuple(ctx, list, true, key, migration)
}

func (tuple *Tuple) Migrate(ctx *Context, key Path, migration *InstanceMigrationArgs) (Value, error) {
	if ctx.GetTx() != nil {
		panic(ErrUnreachable)
	}

	return migrateListOrTuple(ctx, tuple, false, key, migration)
}

func migrateListOrTuple(
	ctx *Context, o Sequence, isList bool,
	key Path, migration *InstanceMigrationArgs) (Value, error) {
	depth := len(GetPathSegments(string(key)))

	migrationHanders := migration.MigrationHandlers
	state := ctx.GetClosestState()

	//a tuple is immutable so it cannot be updated in-place
	var nextTupleElements []Serializable
	//note: nextRecordKeys should not be use to check for the presence of a next record because it could be nil
	var nextTuple bool

	getIndex := func(elementPath []string) (int, error) {
		index, err := strconv.Atoi(elementPath[len(elementPath)-1])

		if err != nil {
			return -1, commonfmt.FmtInvalidLastSegmentOfMigrationPathShouldbeAnInteger(elementPath)
		}

		if index < 0 || index >= o.Len() {
			return -1, commonfmt.FmtLastSegmentOfMigrationPathIsOutOfBounds(elementPath)
		}

		return index, nil
	}

	for pathPattern, handler := range migrationHanders.Deletions {
		pathPatternSegments := GetPathSegments(string(pathPattern))
		pathPatternDepth := len(pathPatternSegments)

		lastSegment := ""
		if pathPatternDepth == 0 {
			if string(pathPattern) != string(key) {
				panic(ErrUnreachable)
			}
		} else {
			lastSegment = pathPatternSegments[pathPatternDepth-1]
		}

		switch {
		case string(pathPattern) == string(key): //list deletion
			if handler == nil {
				return nil, nil
			}

			if handler.Function != nil {
				_, err := handler.Function.Call(state, nil, []Value{o}, nil)
				return nil, err
			} else {
				panic(ErrUnreachable)
			}
		case pathPatternDepth == 1+depth: //element deletion
			if lastSegment == "*" {
				if handler != nil {
					if handler.Function != nil {
						_, err := handler.Function.Call(state, nil, []Value{o}, nil)
						return nil, err
					} else {
						panic(ErrUnreachable)
					}
				}

				if o, ok := o.(*List); ok {
					list := newValueList()
					list.constraintId = o.ConstraintId()
					return WrapUnderlyingList(list), nil
				}

				tuple := NewTuple(nil)
				tuple.constraintId = o.(*Tuple).constraintId
				return tuple, nil
			} else {
				index, err := getIndex(pathPatternSegments)
				if err != nil {
					return nil, err
				}

				elemToRemove := o.At(ctx, index)
				if handler != nil {
					if handler.Function != nil {
						_, err := handler.Function.Call(state, nil, []Value{elemToRemove}, nil)
						return nil, err
					} else {
						panic(ErrUnreachable)
					}
				}

				if list, ok := o.(*List); ok {
					list.removePosition(ctx, Int(index))
				} else {
					if !nextTuple {
						nextTuple = true
						nextTupleElements = slices.Clone(o.(*Tuple).elements)
					}
					nextTupleElements = slices.Delete(nextTupleElements, index, index+1)
				}
			}
		case pathPatternDepth > 1+depth: //deletion inside element value
			elementPathPattern := pathPatternSegments[:depth+1]
			index, err := getIndex(elementPathPattern)
			if err != nil {
				return nil, err
			}

			elementValue := o.At(ctx, index)
			migrationCapable, ok := elementValue.(MigrationCapable)
			if !ok {
				return nil, commonfmt.FmtValueAtPathSegmentsIsNotMigrationCapable(elementPathPattern)
			}

			elementValuePath := "/" + Path(strings.Join(elementPathPattern, ""))
			nextElementValue, err := migrationCapable.Migrate(ctx, elementValuePath, &InstanceMigrationArgs{
				NextPattern:       nil,
				MigrationHandlers: migrationHanders.FilterByPrefix(elementValuePath),
			})

			if err != nil {
				return nil, err
			}
			//if o is a tuple a new tuple with the element removed has to be created
			if !isList {
				if !nextTuple {
					nextTuple = true
					nextTupleElements = slices.Clone(o.(*Tuple).elements)
				}
				nextTupleElements[index] = nextElementValue.(Serializable)
			}
		default:
			panic(ErrUnreachable)
		}
	}

	handle := func(pathPattern PathPattern, handler *MigrationOpHandler, kind MigrationOpKind) (outerFunctionResult Value, outerFunctionError error) {
		pathPatternSegments := GetPathSegments(string(pathPattern))
		pathPatternDepth := len(pathPatternSegments)
		lastSegment := ""
		if pathPatternDepth == 0 {
			if string(pathPattern) != string(key) {
				panic(ErrUnreachable)
			}
		} else {
			lastSegment = pathPatternSegments[pathPatternDepth-1]
		}

		switch {
		case string(pathPattern) == string(key): //list replacement
			if kind != ReplacementMigrationOperation {
				panic(ErrUnreachable)
			}

			if handler.Function != nil {
				return handler.Function.Call(state, nil, []Value{o}, nil)
			} else {
				var initialValue Serializable
				if isList {
					initialValue = handler.InitialValue.(*List)
				} else {
					initialValue = handler.InitialValue.(*Tuple)
				}
				clone, err := RepresentationBasedClone(ctx, initialValue)
				if err != nil {
					return nil, commonfmt.FmtErrWhileCloningValueFor(pathPatternSegments, err)
				}

				return clone, nil
			}
		case pathPatternDepth == 1+depth: //element replacement
			if kind != ReplacementMigrationOperation {
				panic(ErrUnreachable)
			}

			if lastSegment == "*" {
				panic(ErrUnreachable)
			} else {
				elementPathSegments := pathPatternSegments[:pathPatternDepth]

				index, err := getIndex(elementPathSegments)
				if err != nil {
					return nil, err
				}

				prevElementValue := o.At(ctx, index)

				var nextElementValue Value

				if handler.Function != nil {
					nextElementValue, err = handler.Function.Call(state, nil, []Value{prevElementValue}, nil)
					if err != nil {
						return nil, commonfmt.FmtErrWhileCallingMigrationHandler(elementPathSegments, err)
					}
				} else {
					clone, err := RepresentationBasedClone(ctx, handler.InitialValue)
					if err != nil {
						return nil, commonfmt.FmtErrWhileCloningValueFor(elementPathSegments, err)
					}
					nextElementValue = clone
				}

				if isList {
					o.(*List).set(ctx, index, nextElementValue)
				} else {
					if !nextTuple {
						nextTuple = true
						nextTupleElements = slices.Clone(o.(*Tuple).elements)
					}
					nextTupleElements[index] = nextElementValue.(Serializable)
				}
			}
		case pathPatternDepth > 1+depth: //migration inside property value
			elementPathSegments := pathPatternSegments[:depth+1]

			index, err := getIndex(elementPathSegments)
			if err != nil {
				return nil, err
			}

			elementValue := o.At(ctx, index)
			migrationCapable, ok := elementValue.(MigrationCapable)
			if !ok {
				return nil, commonfmt.FmtValueAtPathSegmentsIsNotMigrationCapable(elementPathSegments)
			}

			propertyValuePath := Path("/" + strings.Join(elementPathSegments, ""))
			nextElementValue, err := migrationCapable.Migrate(ctx, propertyValuePath, &InstanceMigrationArgs{
				NextPattern:       nil,
				MigrationHandlers: migrationHanders.FilterByPrefix(propertyValuePath),
			})

			if err != nil {
				return nil, err
			}

			if isList {
				o.(*List).set(ctx, index, nextElementValue)
			} else { //if o is a tuple a new tuple with the element replacement has to be created
				if !nextTuple {
					nextTuple = true
					nextTupleElements = slices.Clone(o.(*Tuple).elements)
				}
				nextTupleElements[index] = nextElementValue.(Serializable)
			}
		}

		return nil, nil
	}

	for pathPattern, handler := range migrationHanders.Replacements {
		result, err := handle(pathPattern, handler, ReplacementMigrationOperation)
		if err != nil {
			return nil, err
		}
		if result != nil {
			return result, nil
		}
	}

	for pathPattern, handler := range migrationHanders.Inclusions {
		result, err := handle(pathPattern, handler, InclusionMigrationOperation)
		if err != nil {
			return nil, err
		}
		if result != nil {
			return result, nil
		}
	}

	for pathPattern, handler := range migrationHanders.Initializations {
		result, err := handle(pathPattern, handler, InitializationMigrationOperation)
		if err != nil {
			return nil, err
		}
		if result != nil {
			return result, nil
		}
	}

	if nextTuple {
		nextTuple := NewTuple(nextTupleElements)
		nextTuple.constraintId = o.(*Tuple).constraintId
		return nextTuple, nil
	}

	return o, nil
}

func isSubType(sub, super Pattern, ctx *Context, encountered map[uintptr]symbolic.Value) bool {
	symbolicSub := utils.Must(sub.ToSymbolicValue(ctx, encountered))
	symbolicSuper := utils.Must(super.ToSymbolicValue(ctx, encountered))

	if !symbolic.IsConcretizable(symbolicSub) {
		panic(ErrUnreachable)
	}

	if !symbolic.IsConcretizable(symbolicSuper) {
		panic(ErrUnreachable)
	}

	return symbolicSuper.Test(symbolicSub, symbolic.RecTestCallState{})
}

func isSuperType(super, sub Pattern, ctx *Context, encountered map[uintptr]symbolic.Value) bool {
	return isSubType(sub, super, ctx, encountered)
}
