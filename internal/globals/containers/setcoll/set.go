package setcoll

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
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
	ErrPropertyUsedForUniquenessNoPresentInPattern    = errors.New("property used for uniqueness is not present in element pattern")

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
	pattern *SetPattern //set for persisted sets.

	//elements and keys

	elementByKey           map[string]core.Serializable
	keyBuf                 *jsoniter.Stream             //used to write JSON representation of elements or key fields
	keySerializationConfig core.JSONSerializationConfig //not set if URL-uniqueness
	pathKeyToKey           map[core.ElementKey]string   //nil on start, will be initialized during the first GetElementByKey call.

	//transactions and locking

	lock                           core.SmartLock
	txIsolator                     core.StrongTransactionIsolator
	transactionsWithSetEndCallback map[*core.Transaction]struct{}
	pendingInclusions              []inclusion
	pendingRemovals                []string
	// /	hasPendingRemovals             atomic.Bool //only used if URL-uniqueness

	//persistence
	storage core.DataStore //nillable
	url     core.URL       //set if .storage set
	path    core.Path

	//note: do not use nested map for pending inclusions when optimizations specific to URL-uniqueness
	//will be implemented.

	//TODO: update watching logic

	//watching
	watchers              *core.ValueWatchers
	mutationCallbacks     *core.MutationCallbacks
	watchingDepth         core.WatchingDepth
	propMutationCallbacks []core.CallbackHandle
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

	if config.Uniqueness.Type == common.UniquePropertyValue && !utils.Implements[core.IPropsPattern](config.Element) {
		panic(ErrPropertyUsedForUniquenessNoPresentInPattern)
	}

	set := NewSetWithConfig(ctx, elements, config)
	set.pattern = utils.Must(SET_PATTERN.Call(ctx, []core.Serializable{set.config.Element, set.config.Uniqueness.ToValue()})).(*SetPattern)
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

		keyBuf:                         jsoniter.NewStream(jsoniter.ConfigDefault, nil, INITIAL_SET_KEY_BUF),
		transactionsWithSetEndCallback: make(map[*core.Transaction]struct{}, 0),

		config: config,
	}

	switch config.Uniqueness.Type {
	case common.UniqueRepr:
		set.keySerializationConfig = core.JSONSerializationConfig{Pattern: config.Element, ReprConfig: &core.ReprConfig{AllVisible: true}}
	case common.UniquePropertyValue:
		pattern, _, ok := config.Element.(core.IPropsPattern).ValuePropPattern(string(config.Uniqueness.PropertyName))
		if !ok {
			panic(ErrPropertyUsedForUniquenessNoPresentInPattern)
		}
		set.keySerializationConfig = core.JSONSerializationConfig{Pattern: pattern, ReprConfig: &core.ReprConfig{AllVisible: true}}
	case common.UniqueURL:
		//empty
	default:
		panic(core.ErrUnreachable)
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
	if set.lock.IsValueShared() {
		if _, err := set.txIsolator.WaitForOtherTxsToTerminate(ctx, false); err != nil {
			panic(err)
		}
		closestState := ctx.MustGetClosestState()
		set._lock(closestState)
		defer set._unlock(closestState)
	}

	set.initPathKeyMap()
	key := set.pathKeyToKey[pathKey]

	elem, ok := set.getElem(key)
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
	if set.lock.IsValueShared() {
		if _, err := set.txIsolator.WaitForOtherTxsToTerminate(ctx, false); err != nil {
			panic(err)
		}
	}

	closestState := ctx.MustGetClosestState()
	set._lock(closestState)
	defer set._unlock(closestState)

	return set.hasNoLock(ctx, elem)
}

func (set *Set) hasNoLock(ctx *core.Context, elem core.Serializable) core.Bool {
	if set.config.Element != nil && !set.config.Element.Test(ctx, elem) {
		panic(ErrValueDoesMatchElementPattern)
	}

	key := set.getUniqueKey(ctx, elem)
	//we don't clone the key because it will not be stored.

	presentElem, ok := set.getElem(key)

	if ok && set.config.Uniqueness.Type != common.UniqueRepr && !core.Same(presentElem, elem) {
		return false
	}
	return core.Bool(ok)
}

// $key is guaranteed to not be stored.
func (set *Set) getElem(key string) (core.Serializable, bool) {
	for _, removedKey := range set.pendingRemovals {
		if removedKey == key {
			return nil, false
		}
	}

	presentElem, ok := set.elementByKey[key]

	if ok {

		return presentElem, true
	}

	for _, inclusion := range set.pendingInclusions {
		if inclusion.key == key {
			return inclusion.value, true
		}
	}

	return nil, false
}

func (set *Set) IsEmpty(ctx *core.Context) bool {
	set.assertPersistedAndSharedIfURLUniqueness()
	if set.lock.IsValueShared() {
		if _, err := set.txIsolator.WaitForOtherTxsToTerminate(ctx, false); err != nil {
			panic(err)
		}
	}

	closestState := ctx.MustGetClosestState()
	set._lock(closestState)
	defer set._unlock(closestState)

	for key := range set.elementByKey {
		isPresent := true
		for _, removedKey := range set.pendingRemovals {
			if removedKey == key {
				isPresent = false
				break
			}
		}
		if isPresent {
			return false //not empty
		}
	}

	for _, inclusion := range set.pendingInclusions {
		isPresent := true
		for _, removedKey := range set.pendingRemovals {
			if removedKey == inclusion.key {
				isPresent = false
				break
			}
		}
		if isPresent {
			return false //not empty
		}
	}

	return true
}

func (set *Set) Get(ctx *core.Context, keyVal core.StringLike) (core.Value, core.Bool) {
	set.assertPersistedAndSharedIfURLUniqueness()

	if set.lock.IsValueShared() {
		if _, err := set.txIsolator.WaitForOtherTxsToTerminate(ctx, false); err != nil {
			panic(err)
		}
		closestState := ctx.MustGetClosestState()
		set._lock(closestState)
		defer set._unlock(closestState)
	}

	key := keyVal.GetOrBuildString()

	elem, ok := set.getElem(key)
	if !ok {
		return nil, false
	}

	return elem, true
}

func (set *Set) Add(ctx *core.Context, elem core.Serializable) {
	set.assertPersistedAndSharedIfURLUniqueness()

	mutation := NewAddElemMutation("/")

	if !set.lock.IsValueShared() {
		// No locking required.
		// Transactions are ignored.

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

		set.informAboutMutation(ctx, mutation)
		return
	}

	/* ====== SHARED SET ====== */

	tx, err := set.txIsolator.WaitForOtherTxsToTerminate(ctx, false)
	if err != nil {
		panic(err)
	}

	if tx != nil && tx.IsReadonly() {
		panic(core.ErrEffectsNotAllowedInReadonlyTransaction)
	}

	set.addToSharedSetNoPersist(ctx, elem, false)

	//determine when to persist the Set and make the changes visible to other transactions

	if tx == nil {
		if set.storage != nil {
			utils.PanicIfErr(persistSet(ctx, set, set.path, set.storage))
		}
		set.informAboutMutation(ctx, mutation)
	} else {
		mutation.Tx = tx
		set.informAboutMutation(ctx, mutation)

		if _, ok := set.transactionsWithSetEndCallback[tx]; !ok {
			closestState := ctx.MustGetClosestState()
			set._lock(closestState)
			defer set._unlock(closestState)

			tx.OnEnd(set, set.makeTransactionEndCallback(ctx, closestState))
			set.transactionsWithSetEndCallback[tx] = struct{}{}
		}
	}
}

func (set *Set) addToSharedSetNoPersist(ctx *core.Context, elem core.Serializable, ignoreTx bool) {
	if set.config.Element != nil && !set.config.Element.Test(ctx, elem) {
		panic(ErrValueDoesMatchElementPattern)
	}

	closestState := ctx.MustGetClosestState()
	elem = utils.Must(core.ShareOrClone(elem, closestState)).(core.Serializable)

	set.config.Uniqueness.AddUrlIfNecessary(ctx, set, elem)

	set._lock(closestState)
	defer set._unlock(closestState)

	key := strings.Clone(set.getUniqueKey(ctx, elem))

	if set.pathKeyToKey != nil {
		set.pathKeyToKey[set.getElementPathKeyFromKey(key)] = key
	}

	//TODO: from time to time .pathKeyToKey should be (safely !) cleaned up

	tx := ctx.GetTx()

	if tx == nil || ignoreTx {
		presentElem, alreadyPresent := set.elementByKey[key]
		if alreadyPresent && set.config.Uniqueness.Type == common.UniquePropertyValue && !core.Same(elem, presentElem) {
			panic(ErrCannotAddDifferentElemWithSamePropertyValue)
		}

		if _, ok := set.elementByKey[key]; ok {
			panic(fmt.Errorf("%w, internal key: %s, element: %s", ErrValueWithSameKeyAlreadyPresent, key, core.Stringify(elem, ctx)))
		}
		set.elementByKey[key] = elem
	} else {
		//Check that another value with the same key has not already been added.
		curr, ok := set.elementByKey[key]
		if ok && elem != curr {
			panic(ErrValueWithSameKeyAlreadyPresent)
		}

		//Remove the key from the pending removals of the tx.
		if index := slices.Index(set.pendingRemovals, key); index >= 0 {
			set.pendingRemovals = slices.Delete(set.pendingRemovals, index, index+1)
		}

		//Add the key and value to the pending inclusions.
		if index := slices.IndexFunc(set.pendingInclusions, func(i inclusion) bool { return i.key == key }); index < 0 {
			set.pendingInclusions = append(set.pendingInclusions, inclusion{
				key:   key,
				value: elem,
			})
		}
	}

}

func (set *Set) Remove(ctx *core.Context, elem core.Serializable) {
	set.assertPersistedAndSharedIfURLUniqueness()

	mutation := NewRemoveElemMutation("/")

	if !set.lock.IsValueShared() {
		// No locking required.
		// Transactions are ignored.

		key := set.getUniqueKey(ctx, elem)

		presentElem, ok := set.elementByKey[key]
		if !ok {
			return
		}

		if set.config.Uniqueness.Type == common.UniquePropertyValue && !core.Same(elem, presentElem) {
			//present element is not elem.
			return
		}

		delete(set.elementByKey, key)
		//TODO: remove path key (ElementKey) efficiently

		set.informAboutMutation(ctx, mutation)
		return
	}

	/* ====== SHARED SET ====== */

	tx := ctx.GetTx()

	if tx != nil && tx.IsReadonly() {
		panic(core.ErrEffectsNotAllowedInReadonlyTransaction)
	}

	if _, err := set.txIsolator.WaitForOtherTxsToTerminate(ctx, false); err != nil {
		panic(err)
	}

	//set.hasPendingRemovals.Store(true)

	key := set.getUniqueKey(ctx, elem)
	closestState := ctx.MustGetClosestState()

	set._lock(closestState)
	defer set._unlock(closestState)

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
		set.informAboutMutation(ctx, mutation)
	} else {
		key = strings.Clone(key)

		//Add the key in the pending removals.
		if index := slices.Index(set.pendingRemovals, key); index < 0 {
			set.pendingRemovals = append(set.pendingRemovals, key)
		}

		//Register a transaction end handler if none is present.
		if _, ok := set.transactionsWithSetEndCallback[tx]; !ok {
			tx.OnEnd(set, set.makeTransactionEndCallback(ctx, closestState))
			set.transactionsWithSetEndCallback[tx] = struct{}{}
		}

		mutation.Tx = tx
		set.informAboutMutation(ctx, mutation)
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
	set.keyBuf.SetBuffer(set.keyBuf.Buffer()[:0])
	key := common.GetUniqueKey(ctx, common.KeyRetrievalParams{
		Value:                   v,
		Config:                  set.config.Uniqueness,
		Container:               set,
		JSONSerializationConfig: set.keySerializationConfig,
		Stream:                  set.keyBuf,
	})
	return key
}

func (set *Set) makeTransactionEndCallback(ctx *core.Context, closestState *core.GlobalState) core.TransactionEndCallbackFn {
	return func(tx *core.Transaction, success bool) {

		//note: closestState is passed instead of being retrieved from ctx because ctx.GetClosestState()
		//will panic if the context is done.

		set.lock.AssertValueShared()

		set._lock(closestState)
		defer set._unlock(closestState)

		defer func() {
			set.pendingInclusions = set.pendingInclusions[:0]
			set.pendingRemovals = set.pendingRemovals[:0]
			//set.hasPendingRemovals.Store(true)
		}()

		if !success {
			return
		}

		for _, inclusion := range set.pendingInclusions {
			set.elementByKey[inclusion.key] = inclusion.value
		}

		for _, key := range set.pendingRemovals {
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

		//TODO: always persist if mutation.tx == nil

		tx := ctx.GetTx()
		if tx != nil {
			//TODO: if tx == mutation.tx record in element changes to apply during commit
			//What should be done if tx is readonly ?
		}

		closestState := ctx.MustGetClosestState()
		set._lock(closestState)
		defer set._unlock(closestState)

		if !set.hasNoLock(ctx, elem) {
			registerAgain = false
			return
		}

		utils.PanicIfErr(persistSet(ctx, set, set.path, set.storage))

		return
	}
}

func (set *Set) hasURLUniqueness() bool {
	return set.config.Uniqueness.Type == common.UniqueURL
}

func (set *Set) assertPersistedAndSharedIfURLUniqueness() {
	if set.hasURLUniqueness() && (!set.lock.IsValueShared() || set.storage == nil) {
		panic(ErrURLUniquenessOnlySupportedIfPersistedSharedSet)
	}
}

type inclusion struct {
	key   string
	value core.Serializable
}
