package mapcoll

import (
	"errors"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/jsoniter"
)

func loadMap(ctx *core.Context, args core.FreeEntityLoadingParams) (core.UrlHolder, error) {
	path := args.Key
	pattern := args.Pattern
	storage := args.Storage
	mapPattern := pattern.(*MapPattern)
	initialValue := args.InitialValue

	var (
		m                *Map
		ok               bool
		serialized       string
		hasSerializedMap bool
	)

	if initialValue != nil {
		m, ok = initialValue.(*Map)
		if !ok {
			list, isList := initialValue.(*core.List)
			if !isList || list.Len() != 0 {
				return nil, fmt.Errorf("%w: a Set or an empty list is expected", core.ErrInvalidInitialValue)
			}
		}
	} else {
		serialized, hasSerializedMap = storage.GetSerialized(ctx, path)
		if !hasSerializedMap {
			if args.AllowMissing {
				serialized = "[]"
				hasSerializedMap = true
			} else {
				return nil, fmt.Errorf("%w: %s", core.ErrFailedToLoadNonExistingValue, path)
			}
		}
		//TODO: return an error if there are duplicate keys.
	}

	if m == nil { //true if there is no initial value or if the initial value is an empty list
		m = NewMapWithConfig(ctx, nil, mapPattern.config)
		m.pattern = mapPattern
		m.storage = storage
		m.path = path
	}

	if m.url != "" {
		return nil, errors.New("initial Set should not have a URL")
	}
	m.url = storage.BaseURL().AppendAbsolutePath(path)

	if hasSerializedMap {
		var finalErr error

		//TODO: lazy load if no migration
		it := jsoniter.ParseString(jsoniter.ConfigDefault, serialized)

		var key core.Serializable

		it.ReadArrayCB(func(it *jsoniter.Iterator) (cont bool) {
			defer func() {
				e := recover()

				if err, ok := e.(error); ok {
					finalErr = err
				} else if e != nil {
					cont = false
					finalErr = fmt.Errorf("%#v", e)
				}
			}()

			if key == nil {
				val, err := core.ParseNextJSONRepresentation(ctx, it, mapPattern.config.Key, false)
				if err != nil {
					finalErr = fmt.Errorf("failed to parse representation of one of the Map's key: %w", err)
					return false
				}

				if val.IsMutable() {
					finalErr = ErrKeysShouldBeImmutable
					cont = false
					return
				}
				key = val
				return true
			}

			//value

			value, err := core.ParseNextJSONRepresentation(ctx, it, mapPattern.config.Value, false)
			if err != nil {
				finalErr = fmt.Errorf("failed to parse representation of one of the Map's value: %w", err)
				return false
			}

			if value.IsMutable() {
				_, ok := value.(core.Watchable)
				if !ok {
					finalErr = fmt.Errorf("element should either be immutable or watchable")
					cont = false
					return
				}
				//mutation handler is added later in the function
			}

			m.putEntryInSharedMap(ctx, entry{
				key:   key,
				value: value,
			}, true)
			key = nil

			return true
		})

		if finalErr != nil {
			return nil, finalErr
		}
	}

	//we perform the migration before adding mutation handlers for obvious reasons
	if args.Migration != nil {
		next, err := m.Migrate(ctx, args.Key, args.Migration)
		if err != nil {
			return nil, fmt.Errorf("migration failed: %w", err)
		}

		if args.IsDeletion(ctx) {
			//TODO: recursively remove
			return nil, nil
		}

		nextMap, ok := next.(*Map)
		if !ok || m != nextMap {
			return core.LoadFreeEntity(ctx, core.FreeEntityLoadingParams{
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
	for _, entry := range m.entryByKey {
		if entry.value.IsMutable() {
			callbackFn := m.makePersistOnMutationCallback(entry.value)
			_, err := entry.value.(core.Watchable).OnMutation(ctx, callbackFn, core.MutationWatchingConfiguration{Depth: core.DeepWatching})
			if err != nil {
				return nil, err
			}
		}
	}

	m.Share(ctx.GetClosestState())

	return m, nil
}

func persistMap(ctx *core.Context, m *Map, path core.Path, storage core.DataStore) error {
	stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)
	err := m.WriteJSONRepresentation(ctx, stream, core.JSONSerializationConfig{
		ReprConfig: &core.ReprConfig{
			AllVisible: true,
		},
		Pattern: m.pattern,
	}, 9)

	if err != nil {
		return err
	}

	storage.SetSerialized(ctx, path, string(stream.Buffer()))
	return nil
}

func (m *Map) WriteJSONRepresentation(ctx *core.Context, w *jsoniter.Stream, config core.JSONSerializationConfig, depth int) error {
	w.WriteArrayStart()

	first := true
	for _, e := range m.entryByKey {
		if !first {
			w.WriteMore()
		}
		first = false

		//key

		if err := e.key.WriteJSONRepresentation(ctx, w, core.JSONSerializationConfig{
			Pattern:    m.config.Key,
			ReprConfig: config.ReprConfig,
		}, depth+1); err != nil {
			return err
		}

		//value

		w.WriteMore()
		if err := e.value.WriteJSONRepresentation(ctx, w, core.JSONSerializationConfig{
			Pattern:    m.config.Value,
			ReprConfig: config.ReprConfig,
		}, depth+1); err != nil {
			return err
		}
	}

	w.WriteArrayEnd()
	return nil
}

func (m *Map) Migrate(ctx *core.Context, key core.Path, migration *core.FreeEntityMigrationArgs) (core.Value, error) {
	return nil, errors.New("migrations are not supported by maps for now")
}
