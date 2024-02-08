package threadcoll

import (
	"errors"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/jsoniter"
	"github.com/inoxlang/inox/internal/utils"
)

func loadThread(ctx *core.Context, args core.FreeEntityLoadingParams) (core.UrlHolder, error) {
	path := args.Key
	pattern := args.Pattern
	storage := args.Storage
	threadPattern := pattern.(*ThreadPattern)
	initialValue := args.InitialValue

	var (
		thread              *MessageThread
		ok                  bool
		serialized          string
		hasSerializedThread bool
	)

	if initialValue != nil {
		thread, ok = initialValue.(*MessageThread)
		if !ok {
			list, isList := initialValue.(*core.List)
			if !isList || list.Len() != 0 {
				return nil, fmt.Errorf("%w: a MessageThread or an empty list is expected", core.ErrInvalidInitialValue)
			}
		}
	} else {
		serialized, hasSerializedThread = storage.GetSerialized(ctx, path)
		if !hasSerializedThread {
			if args.AllowMissing {
				serialized = "[]"
				hasSerializedThread = true
			} else {
				return nil, fmt.Errorf("%w: %s", core.ErrFailedToLoadNonExistingValue, path)
			}
		}
		//TODO: return an error if there are duplicate keys.
	}

	url := storage.BaseURL().AppendAbsolutePath(path)

	if thread == nil { //true if there is no initial value or if the initial value is an empty list
		thread = newEmptyThread(ctx, url, threadPattern)
		thread.storage = storage
		thread.path = path
	} else if thread.url != url {
		return nil, fmt.Errorf("initial thread has not the expected URL (expected %s): %s", url, thread.url)
	}

	if hasSerializedThread {
		var finalErr error

		//TODO: lazy load if no migration
		it := jsoniter.ParseString(jsoniter.ConfigDefault, serialized)
		it.ReadArrayCB(func(i *jsoniter.Iterator) (cont bool) {
			val, err := core.ParseObjectJSONrepresentation(ctx, it, threadPattern.config.Element, false)
			if err != nil {
				finalErr = fmt.Errorf("failed to parse representation of one of the thread's element: %w", err)
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

			thread.addNoLock(ctx, nil, val, true)
			cont = true
			return
		})

		if finalErr != nil {
			return nil, finalErr
		}
	}

	//we perform the migration before adding mutation handlers for obvious reasons
	if args.Migration != nil {
		next, err := thread.Migrate(ctx, args.Key, args.Migration)
		if err != nil {
			return nil, fmt.Errorf("migration failed: %w", err)
		}

		if args.IsDeletion(ctx) {
			//TODO: recursively remove
			return nil, nil
		}

		nextThread, ok := next.(*MessageThread)
		if !ok || thread != nextThread {
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
	for _, internalElem := range thread.elements {
		callbackFn := thread.makePersistOnMutationCallback(internalElem.actualElement)
		_, err := internalElem.actualElement.OnMutation(ctx, callbackFn, core.MutationWatchingConfiguration{Depth: core.DeepWatching})
		if err != nil {
			return nil, err
		}
	}

	thread.Share(ctx.GetClosestState())

	return thread, nil
}

func (t *MessageThread) makePersistOnMutationCallback(elem *core.Object) core.MutationCallbackMicrotask {
	return func(ctx *core.Context, mutation core.Mutation) (registerAgain bool) {
		registerAgain = true

		//TODO: always persist if mutation.tx == nil

		tx := ctx.GetTx()
		if tx != nil {
			//TODO: if tx == mutation.tx record in element changes to apply during commit
			//What should be done if tx is readonly ?
		}

		closestState := ctx.GetClosestState()
		t._lock(closestState)
		defer t._unlock(closestState)

		if !t.Contains(ctx, elem) {
			registerAgain = false
			return
		}

		utils.PanicIfErr(persistThread(ctx, t, t.path, t.storage))

		return
	}
}

func persistThread(ctx *core.Context, thread *MessageThread, path core.Path, storage core.DataStore) error {
	stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)
	err := thread.WriteJSONRepresentation(ctx, stream, core.JSONSerializationConfig{
		ReprConfig: &core.ReprConfig{
			AllVisible: true,
		},
		Pattern: thread.pattern,
	}, 9)

	if err != nil {
		return err
	}

	storage.SetSerialized(ctx, path, string(stream.Buffer()))
	return nil
}

func (t *MessageThread) WriteJSONRepresentation(ctx *core.Context, w *jsoniter.Stream, config core.JSONSerializationConfig, depth int) error {
	w.WriteArrayStart()

	first := true
	for _, e := range t.elements {
		if !first {
			w.WriteMore()
		}
		first = false

		if err := e.actualElement.WriteJSONRepresentation(ctx, w, core.JSONSerializationConfig{
			Pattern:    t.config.Element,
			ReprConfig: config.ReprConfig,
		}, depth+1); err != nil {
			return err
		}
	}

	w.WriteArrayEnd()
	return nil
}

func (s *MessageThread) Migrate(ctx *core.Context, key core.Path, migration *core.FreeEntityMigrationArgs) (core.Value, error) {
	return nil, errors.New("migrations are not supported by message threads for now")
}
