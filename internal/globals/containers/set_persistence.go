package containers

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/containers/common"
	"github.com/inoxlang/inox/internal/jsoniter"
	"github.com/inoxlang/inox/internal/utils/pathutils"
)

func loadSet(ctx *core.Context, args core.InstanceLoadArgs) (core.UrlHolder, error) {
	path := args.Key
	pattern := args.Pattern
	storage := args.Storage
	setPattern := pattern.(*SetPattern)
	initialValue := args.InitialValue

	var (
		set              *Set
		ok               bool
		serialized       string
		hasSerializedSet bool
	)

	if initialValue != nil {
		set, ok = initialValue.(*Set)
		if !ok {
			list, isList := initialValue.(*core.List)
			if !isList || list.Len() != 0 {
				return nil, fmt.Errorf("%w: a Set or an empty list is expected", core.ErrInvalidInitialValue)
			}
		}
	} else {
		serialized, hasSerializedSet = storage.GetSerialized(ctx, path)
		if !hasSerializedSet {
			if args.AllowMissing {
				serialized = "[]"
				hasSerializedSet = true
			} else {
				return nil, fmt.Errorf("%w: %s", core.ErrFailedToLoadNonExistingValue, path)
			}
		}
	}

	if set == nil { //true if there is no initial value or if the initial value is an empty list
		set = NewSetWithConfig(ctx, nil, setPattern.config)
		set.pattern = setPattern
		set.storage = storage
		set.path = path
	}

	if set.url != "" {
		return nil, errors.New("initial Set should not have a URL")
	}
	set.url = storage.BaseURL().AppendAbsolutePath(path)

	if hasSerializedSet {
		var finalErr error

		//TODO: lazy load if no migration
		it := jsoniter.ParseString(jsoniter.ConfigDefault, serialized)
		it.ReadArrayCB(func(i *jsoniter.Iterator) (cont bool) {
			val, err := core.ParseNextJSONRepresentation(ctx, it, setPattern.config.Element, false)
			if err != nil {
				finalErr = fmt.Errorf("failed to parse representation of one of the Set's element: %w", err)
				return false
			}

			defer func() {
				e := recover()

				if err, ok := e.(error); ok {
					finalErr = err
				} else if e != nil {
					cont = false
					finalErr = fmt.Errorf("%#v", e)
				}
			}()
			set.addNoPersist(ctx, val)
			if val.IsMutable() {
				_, ok := val.(core.Watchable)
				if !ok {
					finalErr = fmt.Errorf("element should either be immutable or watchable")
					cont = false
					return
				}
				//mutation handler is added later in the function
			}
			return true
		})

		if finalErr != nil {
			return nil, finalErr
		}
	}

	//we perform the migration before adding mutation handlers for obvious reasons
	if args.Migration != nil {
		next, err := set.Migrate(ctx, args.Key, args.Migration)
		if err != nil {
			return nil, fmt.Errorf("migration failed: %w", err)
		}

		if args.IsDeletion(ctx) {
			//TODO: recursively remove
			return nil, nil
		}

		nextSet, ok := next.(*Set)
		if !ok || set != nextSet {
			return core.LoadInstance(ctx, core.InstanceLoadArgs{
				Key:          args.Key,
				Storage:      args.Storage,
				Pattern:      args.Migration.NextPattern,
				InitialValue: next.(core.Serializable),
				AllowMissing: false,
				Migration:    nil,
			})
		}
	}

	//add mutation handlers
	for _, elem := range set.elements {
		if elem.IsMutable() {
			callbackFn := set.makePersistOnMutationCallback(elem)
			_, err := elem.(core.Watchable).OnMutation(ctx, callbackFn, core.MutationWatchingConfiguration{Depth: core.DeepWatching})
			if err != nil {
				return nil, err
			}
		}
	}

	set.Share(ctx.GetClosestState())

	return set, nil
}

func persistSet(ctx *core.Context, set *Set, path core.Path, storage core.DataStore) error {
	stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)
	set.WriteJSONRepresentation(ctx, stream, core.JSONSerializationConfig{
		ReprConfig: &core.ReprConfig{
			AllVisible: true,
		},
		Pattern: set.pattern,
	}, 9)

	storage.SetSerialized(ctx, path, string(stream.Buffer()))
	return nil
}

func (set *Set) WriteRepresentation(ctx *core.Context, w io.Writer, config *core.ReprConfig, depth int) error {
	if depth > core.MAX_REPR_WRITING_DEPTH {
		return core.ErrMaximumReprWritingDepthReached
	}

	buff := bytes.NewBufferString("[")

	first := true
	for _, e := range set.elements {
		if !first {
			buff.WriteByte(',')
		}
		first = false

		if err := e.WriteRepresentation(ctx, buff, config, depth+1); err != nil {
			return err
		}
	}

	buff.WriteByte(']')
	_, err := w.Write(buff.Bytes())
	return err
}

func (set *Set) WriteJSONRepresentation(ctx *core.Context, w *jsoniter.Stream, config core.JSONSerializationConfig, depth int) error {
	w.WriteArrayStart()

	first := true
	for _, e := range set.elements {
		if !first {
			w.WriteMore()
		}
		first = false

		if err := e.WriteJSONRepresentation(ctx, w, core.JSONSerializationConfig{
			Pattern:    set.config.Element,
			ReprConfig: config.ReprConfig,
		}, depth+1); err != nil {
			return err
		}
	}

	w.WriteArrayEnd()
	return nil
}

func (s *Set) Migrate(ctx *core.Context, key core.Path, migration *core.InstanceMigrationArgs) (core.Value, error) {
	if len(s.pendingInclusions) > 0 || len(s.pendingRemovals) > 0 {
		panic(core.ErrUnreachable)
	}

	if ctx.GetTx() != nil {
		panic(core.ErrUnreachable)
	}

	depth := len(pathutils.GetPathSegments(string(key)))
	migrationHanders := migration.MigrationHandlers
	state := ctx.GetClosestState()

	for pathPattern, handler := range migrationHanders.Deletions {
		pathPatternSegments := pathutils.GetPathSegments(string(pathPattern))
		pathPatternDepth := len(pathPatternSegments)

		lastSegment := ""
		if pathPatternDepth == 0 {
			if string(pathPattern) != string(key) {
				panic(core.ErrUnreachable)
			}
		} else {
			lastSegment = pathPatternSegments[pathPatternDepth-1]
		}

		switch {
		case string(pathPattern) == string(key): //Set deletion
			if handler == nil {
				return nil, nil
			}

			if handler.Function != nil {
				_, err := handler.Function.Call(state, nil, []core.Value{s}, nil)
				return nil, err
			} else {
				panic(core.ErrUnreachable)
			}
		case pathPatternDepth == 1+depth: //element deletion
			if lastSegment == "*" { //delete all elements
				if handler != nil {
					if handler.Function != nil {
						_, err := handler.Function.Call(state, nil, []core.Value{s}, nil)
						if err != nil {
							return nil, err
						}
					} else {
						panic(core.ErrUnreachable)
					}
				}

				clear(s.elements)
				return s, nil
			} else {
				elementPathKey := core.MustElementKeyFrom(lastSegment)
				elementKey := s.pathKeyToKey[elementPathKey]
				elemToRemove, ok := s.elements[elementKey]
				if !ok {
					return nil, commonfmt.FmtValueAtPathSegmentsDoesNotExist(pathPatternSegments)
				}
				if handler != nil {
					if handler.Function != nil {
						_, err := handler.Function.Call(state, nil, []core.Value{elemToRemove}, nil)
						if err != nil {
							return nil, err
						}
					} else {
						panic(core.ErrUnreachable)
					}
				}
				delete(s.elements, elementKey)
				delete(s.pathKeyToKey, elementPathKey)
			}
		case pathPatternDepth > 1+depth: //deletion inside element
			elementPathPattern := pathPatternSegments[:depth+1]
			elementPathKey := core.MustElementKeyFrom(elementPathPattern[len(elementPathPattern)-1])
			elementKey := s.pathKeyToKey[elementPathKey]
			element, ok := s.elements[elementKey]
			if !ok {
				return nil, commonfmt.FmtValueAtPathSegmentsDoesNotExist(elementPathPattern)
			}
			delete(s.elements, elementKey)

			migrationCapable, ok := element.(core.MigrationCapable)
			if !ok {
				return nil, commonfmt.FmtValueAtPathSegmentsIsNotMigrationCapable(elementPathPattern)
			}

			propertyValuePath := "/" + core.Path(strings.Join(elementPathPattern, ""))
			nextElementValue, err := migrationCapable.Migrate(ctx, propertyValuePath, &core.InstanceMigrationArgs{
				NextPattern:       nil,
				MigrationHandlers: migrationHanders.FilterByPrefix(propertyValuePath),
			})

			if err != nil {
				return nil, err
			}
			nextElementKey := common.GetUniqueKey(ctx, nextElementValue.(core.Serializable), s.config.Uniqueness, s)
			s.pathKeyToKey[s.GetElementPathKeyFromKey(nextElementKey)] = nextElementKey
			s.elements[nextElementKey] = nextElementValue.(core.Serializable)
		}
	}

	handle := func(pathPattern core.PathPattern, handler *core.MigrationOpHandler, kind core.MigrationOpKind) (outerFunctionResult core.Value, outerFunctionError error) {
		pathPatternSegments := pathutils.GetPathSegments(string(pathPattern))
		pathPatternDepth := len(pathPatternSegments)
		lastSegment := ""
		if pathPatternDepth == 0 {
			if string(pathPattern) != string(key) {
				panic(core.ErrUnreachable)
			}
		} else {
			lastSegment = pathPatternSegments[pathPatternDepth-1]
		}

		switch {
		case string(pathPattern) == string(key): //Set replacement
			if kind != core.ReplacementMigrationOperation {
				panic(core.ErrUnreachable)
			}

			if handler.Function != nil {
				return handler.Function.Call(state, nil, []core.Value{s}, nil)
			} else {
				initialValue := handler.InitialValue

				clone, err := core.RepresentationBasedClone(ctx, initialValue)
				if err != nil {
					return nil, commonfmt.FmtErrWhileCloningValueFor(pathPatternSegments, err)
				}
				return clone, nil
			}
		case pathPatternDepth == 1+depth: //element replacement|inclusion|init
			_ = lastSegment
			panic(core.ErrUnreachable)
		case pathPatternDepth > 1+depth: //migration inside element
			elementKey := pathPatternSegments[depth]

			if elementKey == "*" {
				for elemKey, elem := range s.elements {
					elementPathPatternSegments := append(slices.Clone(pathPatternSegments[:depth]), elemKey)
					delete(s.elements, elemKey)

					migrationCapable, ok := elem.(core.MigrationCapable)
					if !ok {
						return nil, commonfmt.FmtValueAtPathSegmentsIsNotMigrationCapable(elementPathPatternSegments)
					}

					elementValuePath := core.Path("/" + strings.Join(elementPathPatternSegments, ""))
					nextElementValue, err := migrationCapable.Migrate(ctx, elementValuePath, &core.InstanceMigrationArgs{
						NextPattern:       nil,
						MigrationHandlers: migrationHanders.FilterByPrefix(elementValuePath),
					})
					if err != nil {
						return nil, err
					}
					nextElementKey := common.GetUniqueKey(ctx, nextElementValue.(core.Serializable), s.config.Uniqueness, s)
					s.elements[nextElementKey] = nextElementValue.(core.Serializable)
				}
			} else {
				panic(core.ErrUnreachable)
			}
		}

		return nil, nil
	}

	for pathPattern, handler := range migrationHanders.Replacements {
		result, err := handle(pathPattern, handler, core.ReplacementMigrationOperation)
		if err != nil {
			return nil, err
		}
		if result != nil {
			return result, nil
		}
	}

	for pathPattern, handler := range migrationHanders.Inclusions {
		result, err := handle(pathPattern, handler, core.InclusionMigrationOperation)
		if err != nil {
			return nil, err
		}
		if result != nil {
			return result, nil
		}
	}

	for pathPattern, handler := range migrationHanders.Initializations {
		result, err := handle(pathPattern, handler, core.InitializationMigrationOperation)
		if err != nil {
			return nil, err
		}
		if result != nil {
			return result, nil
		}
	}

	return s, nil
}
