package setcoll

import (
	"errors"
	"reflect"
	"strings"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/jsoniter"
	"github.com/inoxlang/inox/internal/utils"

	"github.com/inoxlang/inox/internal/globals/containers/common"
	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
)

const (
	INITIAL_SET_KEY_BUF = 2000
)

var (
	ErrSetCanOnlyContainRepresentableValues           = errors.New("a Set can only contain representable values")
	ErrValueDoesMatchElementPattern                   = errors.New("provided value does not match the element pattern")
	ErrValueWithSameKeyAlreadyPresent                 = errors.New("provided value has the same key as an already present element")
	ErrURLUniquenessOnlySupportedIfPersistedSharedSet = errors.New("URL uniqueness is only supported if the Set is persisted and shared")
	ErrCannotAddDifferentElemWithSamePropertyValue    = errors.New("cannot add different element with same property value")

	_ core.Collection           = (*Set)(nil)
	_ core.PotentiallySharable  = (*Set)(nil)
	_ core.SerializableIterable = (*Set)(nil)
	_ core.MigrationCapable     = (*Set)(nil)
)

func init() {
	core.RegisterLoadFreeEntityFn(reflect.TypeOf((*SetPattern)(nil)), loadSet)

	core.RegisterDefaultPattern(SET_PATTERN.Name, SET_PATTERN)
	core.RegisterDefaultPattern(SET_PATTERN_PATTERN.Name, SET_PATTERN_PATTERN)
	core.RegisterPatternDeserializer(SET_PATTERN_PATTERN, DeserializeSetPattern)

}

type Set struct {
	config  SetConfig
	pattern *SetPattern

	//elements and keys

	elementByKey        map[string]core.Serializable
	keyBuf              *jsoniter.Stream //used to write JSON representation of elements or key fields
	serializationConfig core.JSONSerializationConfig
	pathKeyToKey        map[core.ElementKey]string //nil on start, will be initialized during the first GetElementByKey call.

	//transactions

	pendingInclusions              map[*core.Transaction]map[string]core.Serializable
	pendingRemovals                map[*core.Transaction]map[string]struct{}
	transactionsWithSetEndCallback map[*core.Transaction]struct{}
	lock                           core.SmartLock

	//persistence
	storage core.DataStore //nillable
	url     core.URL       //set if .storage set
	path    core.Path
}

func NewSet(ctx *core.Context, elements core.Iterable, configParam *core.OptionalParam[*core.Object]) *Set {
	config := SetConfig{
		Uniqueness: common.UniquenessConstraint{
			Type: common.UniqueRepr,
		},
		Element: core.SERIALIZABLE_PATTERN,
	}

	if configParam != nil {
		//iterate over the properties of the provided object

		obj := configParam.Value
		obj.ForEachEntry(func(k string, v core.Serializable) error {
			switch k {
			case coll_symbolic.SET_CONFIG_ELEMENT_PATTERN_PROP_KEY:
				pattern, ok := v.(core.Pattern)
				if !ok {
					panic(commonfmt.FmtInvalidValueForPropXOfArgY(k, "configuration", "a pattern is expected"))
				}
				config.Element = pattern
			case coll_symbolic.SET_CONFIG_UNIQUE_PROP_KEY:
				uniqueness, ok := common.UniquenessConstraintFromValue(v)
				if !ok {
					panic(commonfmt.FmtInvalidValueForPropXOfArgY(k, "configuration", "?"))
				}
				config.Uniqueness = uniqueness
			default:
				panic(commonfmt.FmtUnexpectedPropInArgX(k, "configuration"))
			}
			return nil
		})
	}

	set := NewSetWithConfig(ctx, elements, config)
	set.pattern = utils.Must(SET_PATTERN.Call([]core.Serializable{set.config.Element, set.config.Uniqueness.ToValue()})).(*SetPattern)
	return set
}

type SetConfig struct {
	Element    core.Pattern
	Uniqueness common.UniquenessConstraint
}

func (c SetConfig) Equal(ctx *core.Context, otherConfig SetConfig, alreadyCompared map[uintptr]uintptr, depth int) bool {
	if !c.Uniqueness.Equal(otherConfig.Uniqueness) {
		return false
	}

	//TODO: check Repr config
	if (c.Element == nil) != (otherConfig.Element == nil) {
		return false
	}

	return c.Element == nil || c.Element.Equal(ctx, otherConfig.Element, alreadyCompared, depth+1)
}

func NewSetWithConfig(ctx *core.Context, elements core.Iterable, config SetConfig) *Set {
	set := &Set{
		elementByKey: make(map[string]core.Serializable),

		keyBuf:              jsoniter.NewStream(jsoniter.ConfigDefault, nil, INITIAL_SET_KEY_BUF),
		serializationConfig: core.JSONSerializationConfig{Pattern: config.Element, ReprConfig: &core.ReprConfig{AllVisible: true}},

		pendingInclusions:              make(map[*core.Transaction]map[string]core.Serializable, 0),
		pendingRemovals:                make(map[*core.Transaction]map[string]struct{}, 0),
		transactionsWithSetEndCallback: make(map[*core.Transaction]struct{}, 0),
		config:                         config,
	}

	if elements != nil {
		it := elements.Iterator(ctx, core.IteratorConfiguration{})
		for it.Next(ctx) {
			e := it.Value(ctx)
			set.Add(ctx, e.(core.Serializable))
		}
	}

	return set
}

func (set *Set) URL() (core.URL, bool) {
	if set.storage != nil {
		return set.url, true
	}
	return "", false
}

func (set *Set) getElementPathKeyFromKey(key string) core.ElementKey {
	return common.GetElementPathKeyFromKey(key, set.config.Uniqueness.Type)
}

func (set *Set) SetURLOnce(ctx *core.Context, url core.URL) error {
	return core.ErrValueDoesNotAcceptURL
}

func (set *Set) GetElementByKey(ctx *core.Context, pathKey core.ElementKey) (core.Serializable, error) {
	set.initPathKeyMap()
	key := set.pathKeyToKey[pathKey]

	tx := ctx.GetTx()

	if tx != nil {
		pendingRemovals := set.pendingRemovals[tx]
		_, removed := pendingRemovals[key]
		if removed {
			return nil, core.ErrCollectionElemNotFound
		}

		pendingInclusions := set.pendingInclusions[tx]
		elem, added := pendingInclusions[key]
		if added {
			return elem, nil
		}
	}

	elem, ok := set.elementByKey[key]
	if !ok {
		return nil, core.ErrCollectionElemNotFound
	}
	return elem, nil
}

func (set *Set) Contains(ctx *core.Context, value core.Serializable) bool {
	return bool(set.Has(ctx, value))
}

func (set *Set) Has(ctx *core.Context, elem core.Serializable) core.Bool {
	set.assertPersistedAndSharedIfURLUniqueness()

	closestState := ctx.GetClosestState()
	set.lock.Lock(closestState, set)
	defer set.lock.Unlock(closestState, set)

	return set.hasNoLock(ctx, elem)
}

func (set *Set) hasNoLock(ctx *core.Context, elem core.Serializable) core.Bool {
	if set.config.Element != nil && !set.config.Element.Test(ctx, elem) {
		panic(ErrValueDoesMatchElementPattern)
	}

	key := set.getUniqueKey(ctx, elem)
	//we don't clone the key because it will not be stored.

	tx := ctx.GetTx()

	if tx != nil {
		pendingRemovals := set.pendingRemovals[tx]
		_, removed := pendingRemovals[key]
		if removed {
			return false
		}

		pendingInclusions := set.pendingInclusions[tx]
		_, added := pendingInclusions[key]
		if added {
			return true
		}
	}

	presentElem, ok := set.elementByKey[key]

	if ok && set.config.Uniqueness.Type != common.UniqueRepr && !core.Same(presentElem, elem) {
		return false
	}
	return core.Bool(ok)
}

func (set *Set) Get(ctx *core.Context, keyVal core.StringLike) (core.Value, core.Bool) {
	set.assertPersistedAndSharedIfURLUniqueness()

	key := keyVal.GetOrBuildString()

	tx := ctx.GetTx()

	if tx != nil {
		pendingRemovals := set.pendingRemovals[tx]
		_, removed := pendingRemovals[key]
		if removed {
			return nil, false
		}

		pendingInclusions := set.pendingInclusions[tx]
		elem, added := pendingInclusions[key]
		if added {
			return elem, true
		}
	}

	elem, ok := set.elementByKey[key]
	if !ok {
		return nil, false
	}

	return elem, true
}

func (set *Set) Add(ctx *core.Context, elem core.Serializable) {
	set.assertPersistedAndSharedIfURLUniqueness()

	if !set.lock.IsValueShared() {
		// no locking required.

		if set.config.Element != nil && !set.config.Element.Test(ctx, elem) {
			panic(ErrValueDoesMatchElementPattern)
		}

		set.config.Uniqueness.AddUrlIfNecessary(ctx, set, elem)
		key := set.getUniqueKey(ctx, elem)

		presentElem, alreadyPresent := set.elementByKey[key]
		if alreadyPresent {
			if set.config.Uniqueness.Type == common.UniquePropertyValue && !core.Same(elem, presentElem) {
				panic(ErrCannotAddDifferentElemWithSamePropertyValue)
			}

			//no need to clone the key.
			return
		}

		key = strings.Clone(key)
		set.elementByKey[key] = elem

		if set.pathKeyToKey != nil {
			set.pathKeyToKey[set.getElementPathKeyFromKey(key)] = key
		}
		return
	}

	set.addToSharedSetNoPersist(ctx, elem)

	//determine when to persist the Set and make the changes visible to other transactions

	tx := ctx.GetTx()

	if tx == nil {
		if set.storage != nil {
			utils.PanicIfErr(persistSet(ctx, set, set.path, set.storage))
		}
	} else if _, ok := set.transactionsWithSetEndCallback[tx]; !ok {
		closestState := ctx.GetClosestState()
		set.lock.Lock(closestState, set)
		defer set.lock.Unlock(closestState, set)

		tx.OnEnd(set, set.makeTransactionEndCallback(ctx, closestState))
		set.transactionsWithSetEndCallback[tx] = struct{}{}
	}
}

func (set *Set) addToSharedSetNoPersist(ctx *core.Context, elem core.Serializable) {

	if set.config.Element != nil && !set.config.Element.Test(ctx, elem) {
		panic(ErrValueDoesMatchElementPattern)
	}

	closestState := ctx.GetClosestState()
	elem = utils.Must(core.ShareOrClone(elem, closestState)).(core.Serializable)

	set.config.Uniqueness.AddUrlIfNecessary(ctx, set, elem)
	key := strings.Clone(set.getUniqueKey(ctx, elem))

	set.lock.Lock(closestState, set)
	defer set.lock.Unlock(closestState, set)

	if set.pathKeyToKey != nil {
		set.pathKeyToKey[set.getElementPathKeyFromKey(key)] = key
	}

	//TODO: from time to time .pathKeyToKey should be (safely !) cleaned up

	tx := ctx.GetTx()

	if tx == nil {
		presentElem, alreadyPresent := set.elementByKey[key]
		if alreadyPresent && set.config.Uniqueness.Type == common.UniquePropertyValue && !core.Same(elem, presentElem) {
			panic(ErrCannotAddDifferentElemWithSamePropertyValue)
		}

		if _, ok := set.elementByKey[key]; ok {
			panic(ErrValueWithSameKeyAlreadyPresent)
		}
		set.elementByKey[key] = elem
	} else {
		//Check that a value (than can be different) with the same key is not already being added by the same tx.
		pendingInclusions := set.pendingInclusions[tx]
		_, added := pendingInclusions[key]
		if added {
			panic(ErrValueWithSameKeyAlreadyPresent)
		}

		//Check that another value with the same key has not already been added.
		curr, ok := set.elementByKey[key]
		if ok && elem != curr {
			panic(ErrValueWithSameKeyAlreadyPresent)
		}

		//Remove the key from the pending removals of the tx.
		pendingRemovals := set.pendingRemovals[tx]
		_, removed := pendingRemovals[key]
		if removed {
			delete(pendingRemovals, key)
		}

		//Add the key and value to the pending inclusions of the tx.
		if pendingInclusions == nil {
			pendingInclusions = make(map[string]core.Serializable)
			set.pendingInclusions[tx] = pendingInclusions
		}

		pendingInclusions[key] = elem
	}

}

func (set *Set) Remove(ctx *core.Context, elem core.Serializable) {
	set.assertPersistedAndSharedIfURLUniqueness()

	key := set.getUniqueKey(ctx, elem)
	closestState := ctx.GetClosestState()

	set.lock.Lock(closestState, set)
	defer set.lock.Unlock(closestState, set)

	tx := ctx.GetTx()

	if tx == nil {
		presentElem, ok := set.elementByKey[key]
		if !ok {
			return
		}

		if set.config.Uniqueness.Type != common.UniqueRepr &&
			!core.Same(presentElem, elem) {
			return
		}

		delete(set.elementByKey, key)
		if set.storage != nil {
			utils.PanicIfErr(persistSet(ctx, set, set.path, set.storage))
		}
	} else {
		key = strings.Clone(key)

		//Add the key in the pending removals of the tx.
		pendingRemovals, ok := set.pendingRemovals[tx]
		if !ok {
			pendingRemovals = make(map[string]struct{})
			set.pendingRemovals[tx] = pendingRemovals
		}

		pendingRemovals[key] = struct{}{}

		//Register a transaction end handler if none is present.
		if _, ok := set.transactionsWithSetEndCallback[tx]; !ok {
			tx.OnEnd(set, set.makeTransactionEndCallback(ctx, closestState))
			set.transactionsWithSetEndCallback[tx] = struct{}{}
		}
	}
}

func (set *Set) initPathKeyMap() {
	if set.pathKeyToKey != nil {
		//already initialized
		return
	}
	set.pathKeyToKey = make(map[core.ElementKey]string, len(set.elementByKey))
	for elemKey := range set.elementByKey {
		set.pathKeyToKey[set.getElementPathKeyFromKey(elemKey)] = elemKey
	}
}

// getUniqueKey returns a key that should be cloned if it is stored.
func (set *Set) getUniqueKey(ctx *core.Context, v core.Serializable) string {
	key := common.GetUniqueKey(ctx, common.KeyRetrievalParams{
		Value:                   v,
		Config:                  set.config.Uniqueness,
		Container:               set,
		JSONSerializationConfig: set.serializationConfig,
		Stream:                  set.keyBuf,
	})
	return key
}

func (set *Set) makeTransactionEndCallback(ctx *core.Context, closestState *core.GlobalState) core.TransactionEndCallbackFn {
	return func(tx *core.Transaction, success bool) {

		//note: closestState is passed instead of being retrieved from ctx because ctx.GetClosestState()
		//will panic if the context is done.

		set.lock.AssertValueShared()

		set.lock.Lock(closestState, set)
		defer set.lock.Unlock(closestState, set)

		defer func() {
			delete(set.pendingInclusions, tx)
			delete(set.pendingRemovals, tx)
		}()

		if !success {
			return
		}

		for key, value := range set.pendingInclusions[tx] {
			set.elementByKey[key] = value
		}

		for key := range set.pendingRemovals[tx] {
			delete(set.elementByKey, key)
		}

		if set.storage != nil {
			utils.PanicIfErr(persistSet(ctx, set, set.path, set.storage))
		}
	}
}

func (set *Set) makePersistOnMutationCallback(elem core.Serializable) core.MutationCallbackMicrotask {
	return func(ctx *core.Context, mutation core.Mutation) (registerAgain bool) {
		registerAgain = true

		tx := ctx.GetTx()
		if tx != nil {
			//if there is a transaction the set will be persisted when the transaction is finished.
			return
		}

		closestState := ctx.GetClosestState()
		set.lock.Lock(closestState, set)
		defer set.lock.Unlock(closestState, set)

		if !set.hasNoLock(ctx, elem) {
			registerAgain = false
			return
		}

		utils.PanicIfErr(persistSet(ctx, set, set.path, set.storage))

		return
	}
}

func (set *Set) assertPersistedAndSharedIfURLUniqueness() {
	if set.config.Uniqueness.Type == common.UniqueURL && (!set.lock.IsValueShared() || set.storage == nil) {
		panic(ErrURLUniquenessOnlySupportedIfPersistedSharedSet)
	}
}
