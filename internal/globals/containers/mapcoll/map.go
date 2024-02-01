package mapcoll

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/containers/common"
	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"

	"github.com/inoxlang/inox/internal/jsoniter"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	INITIAL_MAP_KEY_BUF = 2000
)

var (
	ErrMapEntryListShouldHaveEvenLength     = errors.New(`flat map entry list should have an even length: ["k1", 1,  "k2", 2]`)
	ErrMapCanOnlyContainKeysWithFastId      = errors.New("a Map can only contain keys having a fast id")
	ErrSetCanOnlyContainRepresentableValues = errors.New("a Map can only contain representable values")
	ErrKeysShouldBeImmutable                = errors.New("keys should be immutable")
	ErrKeyDoesMatchKeyPattern               = errors.New("provided key does not match the key pattern")
	ErrValueDoesMatchValuePattern           = errors.New("provided value does not match the value pattern")
	ErrEntryAlreadyExists                   = errors.New("entry already exists")
	ErrValueWithSameKeyAlreadyPresent       = errors.New("provided value has the same key as an already present element")

	_ core.Collection           = (*Map)(nil)
	_ core.PotentiallySharable  = (*Map)(nil)
	_ core.SerializableIterable = (*Map)(nil)
	_ core.MigrationCapable     = (*Map)(nil)
)

func init() {
	core.RegisterLoadFreeEntityFn(reflect.TypeOf((*MapPattern)(nil)), loadMap)

	core.RegisterDefaultPattern(MAP_PATTERN.Name, MAP_PATTERN)
	core.RegisterDefaultPattern(MAP_PATTERN_PATTERN.Name, MAP_PATTERN_PATTERN)
	core.RegisterPatternDeserializer(MAP_PATTERN_PATTERN, DeserializeMapPattern)
}

type Map struct {
	config             MapConfig
	pattern            *MapPattern //set for persisted maps.
	keyReprUniquenesss common.UniquenessConstraint

	//elements and keys

	entryByKey             map[string]entry
	keyBuf                 *jsoniter.Stream //used to write JSON representation of elements or key fields
	keySerializationConfig core.JSONSerializationConfig
	pathKeyToKey           map[core.ElementKey]string //nil on start, will be initialized during the first GetElementByKey call.

	//transactions and locking

	lock                           core.SmartLock
	txIsolator                     core.StrongTransactionIsolator
	transactionsWithSetEndCallback map[*core.Transaction]struct{}
	pendingInclusions              []inclusion
	pendingRemovals                []string

	//persistence
	storage core.DataStore //nillable
	url     core.URL       //set if .storage set
	path    core.Path

	//note: do not use nested map for pending inclusions when optimizations specific to URL-uniqueness
	//will be implemented.
}

type MapConfig struct {
	Key, Value core.Pattern
}

func (c MapConfig) Equal(ctx *core.Context, otherConfig MapConfig, alreadyCompared map[uintptr]uintptr, depth int) bool {
	return (c.Key == nil || c.Key.Equal(ctx, otherConfig.Key, alreadyCompared, depth+1)) && (c.Value == nil || c.Value.Equal(ctx, otherConfig.Value, alreadyCompared, depth+1))
}

func NewMap(ctx *core.Context, flatEntries *core.List, configParam *core.OptionalParam[*core.Object]) *Map {
	config := MapConfig{
		Key:   core.SERIALIZABLE_PATTERN,
		Value: core.SERIALIZABLE_PATTERN,
	}

	if configParam != nil {
		//iterate over the properties of the provided object

		obj := configParam.Value
		obj.ForEachEntry(func(k string, v core.Serializable) error {
			switch k {
			case coll_symbolic.MAP_CONFIG_KEY_PATTERN_KEY:
				pattern, ok := v.(core.Pattern)
				if !ok {
					panic(commonfmt.FmtInvalidValueForPropXOfArgY(k, "configuration", "a pattern is expected"))
				}
				config.Key = pattern
			case coll_symbolic.MAP_CONFIG_VALUE_PATTERN_KEY:
				pattern, ok := v.(core.Pattern)
				if !ok {
					panic(commonfmt.FmtInvalidValueForPropXOfArgY(k, "configuration", "a pattern is expected"))
				}
				config.Value = pattern
			default:
				panic(commonfmt.FmtUnexpectedPropInArgX(k, "configuration"))
			}
			return nil
		})
	}

	m := NewMapWithConfig(ctx, flatEntries, config)
	m.pattern = utils.Must(MAP_PATTERN.Call([]core.Serializable{
		m.config.Key,
		m.config.Value,
	})).(*MapPattern)

	return m
}

func NewMapWithConfig(ctx *core.Context, flatEntries *core.List, config MapConfig) *Map {
	map_ := &Map{
		entryByKey: make(map[string]entry),

		keyBuf:                         jsoniter.NewStream(jsoniter.ConfigDefault, nil, INITIAL_MAP_KEY_BUF),
		keySerializationConfig:         core.JSONSerializationConfig{Pattern: config.Key, ReprConfig: &core.ReprConfig{AllVisible: true}},
		transactionsWithSetEndCallback: make(map[*core.Transaction]struct{}, 0),

		config:             config,
		keyReprUniquenesss: *common.NewReprUniqueness(),
	}

	if flatEntries != nil {
		if flatEntries.Len()%2 != 0 {
			panic(ErrMapEntryListShouldHaveEvenLength)
		}

		for i := 0; i < flatEntries.Len(); i += 2 {
			key := flatEntries.At(ctx, i).(core.Serializable)
			value := flatEntries.At(ctx, i+1).(core.Serializable)

			map_.Insert(ctx, key, value)
		}
	}

	return map_
}

func (m *Map) URL() (core.URL, bool) {
	if m.storage != nil {
		return m.url, true
	}
	return "", false
}

func (m *Map) getElementPathKeyFromKey(key string) core.ElementKey {
	return common.GetElementPathKeyFromKey(key, m.keyReprUniquenesss.Type)
}

func (m *Map) SetURLOnce(ctx *core.Context, url core.URL) error {
	return core.ErrValueDoesNotAcceptURL
}

func (m *Map) GetElementByKey(ctx *core.Context, pathKey core.ElementKey) (core.Serializable, error) {
	if m.lock.IsValueShared() {
		if _, err := m.txIsolator.WaitForOtherTxsToTerminate(ctx, false); err != nil {
			panic(err)
		}
		closestState := ctx.GetClosestState()
		m._lock(closestState)
		defer m._unlock(closestState)
	}

	m.initPathKeyMap()
	key := m.pathKeyToKey[pathKey]

	entry, ok := m.entryByKey[key]
	if !ok {
		return nil, core.ErrCollectionElemNotFound
	}

	//TODO
	_ = entry
	return nil, errors.New("entry type not implemented yet")
}

func (m *Map) Contains(ctx *core.Context, value core.Serializable) bool {
	if m.lock.IsValueShared() {
		if _, err := m.txIsolator.WaitForOtherTxsToTerminate(ctx, false); err != nil {
			panic(err)
		}
		closestState := ctx.GetClosestState()
		m._lock(closestState)
		defer m._unlock(closestState)
	}

	alreadyCompared := map[uintptr]uintptr{}

	for serializedKey, entry := range m.entryByKey {
		for _, removedKey := range m.pendingRemovals {
			if serializedKey == removedKey {
				goto ignore_entry
			}
		}

		if value.Equal(ctx, entry.value, alreadyCompared, 0) {
			return true
		}

	ignore_entry:
	}

	for _, inclusion := range m.pendingInclusions {
		if value.Equal(ctx, inclusion.entry.value, alreadyCompared, 0) {
			return true
		}
	}

	return false
}

func (m *Map) Has(ctx *core.Context, keyVal core.Serializable) core.Bool {
	if m.lock.IsValueShared() {
		if _, err := m.txIsolator.WaitForOtherTxsToTerminate(ctx, false); err != nil {
			panic(err)
		}
	}

	closestState := ctx.GetClosestState()
	m._lock(closestState)
	defer m._unlock(closestState)

	return m.hasNoLock(ctx, keyVal)
}

func (m *Map) hasNoLock(ctx *core.Context, key core.Serializable) core.Bool {
	if m.config.Key != nil && !m.config.Key.Test(ctx, key) {
		panic(ErrKeyDoesMatchKeyPattern)
	}

	serializedKey := m.getUniqueKey(ctx, key)
	//we don't clone the key because it will not be stored.

	_, ok := m.getEntry(serializedKey)

	return core.Bool(ok)
}

func (m *Map) getEntry(key string) (entry, bool) {
	for _, removedKey := range m.pendingRemovals {
		if removedKey == key {
			return entry{}, false
		}
	}

	presentEntry, ok := m.entryByKey[key]

	if ok {
		return presentEntry, true
	}

	for _, inclusion := range m.pendingInclusions {
		if inclusion.serializedKey == key {
			return inclusion.entry, true
		}
	}

	return entry{}, false
}

func (m *Map) Get(ctx *core.Context, keyVal core.Serializable) (core.Value, core.Bool) {
	if m.lock.IsValueShared() {
		if _, err := m.txIsolator.WaitForOtherTxsToTerminate(ctx, false); err != nil {
			panic(err)
		}
		closestState := ctx.GetClosestState()
		m._lock(closestState)
		defer m._unlock(closestState)
	}

	serialiedKey := m.getUniqueKey(ctx, keyVal)

	entry, ok := m.getEntry(serialiedKey)
	if !ok {
		return nil, false
	}

	return entry.value, true
}

func (m *Map) Insert(ctx *core.Context, key, value core.Serializable) error {
	return m.set(ctx, entry{key, value}, true)
}

func (m *Map) Set(ctx *core.Context, key, value core.Serializable) {
	m.set(ctx, entry{key, value}, false)
}

func (m *Map) set(ctx *core.Context, entry entry, insert bool) error {

	if entry.key.IsMutable() {
		panic(fmt.Errorf("invalid key: %w", core.ErrReprOfMutableValueCanChange))
	}

	if !m.lock.IsValueShared() {
		// No locking required.
		// Transactions are ignored.

		if m.config.Key != nil && !m.config.Key.Test(ctx, entry.key) {
			panic(ErrKeyDoesMatchKeyPattern)
		}

		if m.config.Value != nil && !m.config.Value.Test(ctx, entry.value) {
			panic(ErrValueDoesMatchValuePattern)
		}

		serializedKey := m.getUniqueKey(ctx, entry.key)

		_, alreadyPresent := m.entryByKey[serializedKey]
		if alreadyPresent {
			if insert {
				panic(ErrEntryAlreadyExists)
			}
			//no need to clone the key.
			return nil
		}

		serializedKey = strings.Clone(serializedKey)
		m.entryByKey[serializedKey] = entry

		if m.pathKeyToKey != nil {
			m.pathKeyToKey[m.getElementPathKeyFromKey(serializedKey)] = serializedKey
		}
		return nil
	}

	/* ====== SHARED MAP ====== */

	tx, err := m.txIsolator.WaitForOtherTxsToTerminate(ctx, false)

	if err != nil {
		panic(err)
	}

	if tx != nil && tx.IsReadonly() {
		panic(core.ErrEffectsNotAllowedInReadonlyTransaction)
	}

	m.putEntryInSharedMap(ctx, entry, false)

	//determine when to persist the Map and make the changes visible to other transactions

	if tx == nil {
		if m.storage != nil {
			utils.PanicIfErr(persistMap(ctx, m, m.path, m.storage))
		}
	} else if _, ok := m.transactionsWithSetEndCallback[tx]; !ok {
		closestState := ctx.GetClosestState()
		m._lock(closestState)
		defer m._unlock(closestState)

		tx.OnEnd(m, m.makeTransactionEndCallback(ctx, closestState))
		m.transactionsWithSetEndCallback[tx] = struct{}{}
	}

	return nil
}

func (m *Map) putEntryInSharedMap(ctx *core.Context, entry entry, ignoreTx bool) {
	if m.config.Key != nil && !m.config.Key.Test(ctx, entry.key) {
		panic(ErrKeyDoesMatchKeyPattern)
	}

	if m.config.Value != nil && !m.config.Value.Test(ctx, entry.value) {
		panic(ErrValueDoesMatchValuePattern)
	}

	closestState := ctx.GetClosestState()
	entry.value = utils.Must(core.ShareOrClone(entry.value, closestState)).(core.Serializable)

	m._lock(closestState)
	defer m._unlock(closestState)

	serializedKey := strings.Clone(m.getUniqueKey(ctx, entry.key))

	if m.pathKeyToKey != nil {
		m.pathKeyToKey[m.getElementPathKeyFromKey(serializedKey)] = serializedKey
	}

	//TODO: from time to time .pathKeyToKey should be (safely !) cleaned up

	tx := ctx.GetTx()

	if tx == nil || ignoreTx {
		if _, ok := m.entryByKey[serializedKey]; ok {
			panic(ErrValueWithSameKeyAlreadyPresent)
		}
		m.entryByKey[serializedKey] = entry
	} else {
		//Check that another value with the same key has not already been added.
		curr, ok := m.entryByKey[serializedKey]
		if ok && entry.value != curr.value {
			panic(ErrValueWithSameKeyAlreadyPresent)
		}

		//Remove the key from the pending removals of the tx.
		if index := slices.Index(m.pendingRemovals, serializedKey); index >= 0 {
			m.pendingRemovals = slices.Delete(m.pendingRemovals, index, index+1)
		}

		//Add the entry to the pending inclusions.
		if index := slices.IndexFunc(m.pendingInclusions, func(i inclusion) bool { return i.serializedKey == serializedKey }); index < 0 {
			m.pendingInclusions = append(m.pendingInclusions, inclusion{
				serializedKey: serializedKey,
				entry:         entry,
			})
		}
	}

}

func (m *Map) Remove(ctx *core.Context, key core.Serializable) {

	if !m.lock.IsValueShared() {
		// No locking required.
		// Transactions are ignored.

		serializedKey := m.getUniqueKey(ctx, key)

		delete(m.entryByKey, serializedKey)
		//TODO: remove path key (ElementKey) efficiently
		return
	}

	/* ====== SHARED MAP ====== */

	tx := ctx.GetTx()

	if tx != nil && tx.IsReadonly() {
		panic(core.ErrEffectsNotAllowedInReadonlyTransaction)
	}

	if _, err := m.txIsolator.WaitForOtherTxsToTerminate(ctx, false); err != nil {
		panic(err)
	}

	serializedKey := m.getUniqueKey(ctx, key)
	closestState := ctx.GetClosestState()

	m._lock(closestState)
	defer m._unlock(closestState)

	if tx == nil {
		delete(m.entryByKey, serializedKey)
		if m.storage != nil {
			utils.PanicIfErr(persistMap(ctx, m, m.path, m.storage))
		}
	} else {
		serializedKey = strings.Clone(serializedKey)

		//Add the key in the pending removals.
		if index := slices.Index(m.pendingRemovals, serializedKey); index < 0 {
			m.pendingRemovals = append(m.pendingRemovals, serializedKey)
		}

		//Register a transaction end handler if none is present.
		if _, ok := m.transactionsWithSetEndCallback[tx]; !ok {
			tx.OnEnd(m, m.makeTransactionEndCallback(ctx, closestState))
			m.transactionsWithSetEndCallback[tx] = struct{}{}
		}
	}
}

func (m *Map) initPathKeyMap() {
	if m.pathKeyToKey != nil {
		//already initialized
		return
	}
	m.pathKeyToKey = make(map[core.ElementKey]string, len(m.entryByKey))
	for elemKey := range m.entryByKey {
		m.pathKeyToKey[m.getElementPathKeyFromKey(elemKey)] = elemKey
	}
}

// getUniqueKey returns a key that should be cloned if it is stored.
func (m *Map) getUniqueKey(ctx *core.Context, v core.Serializable) string {
	key := common.GetUniqueKey(ctx, common.KeyRetrievalParams{
		Value:                   v,
		Config:                  m.keyReprUniquenesss,
		Container:               m,
		JSONSerializationConfig: m.keySerializationConfig,
		Stream:                  m.keyBuf,
	})
	return key
}

func (m *Map) makeTransactionEndCallback(ctx *core.Context, closestState *core.GlobalState) core.TransactionEndCallbackFn {
	return func(tx *core.Transaction, success bool) {

		//note: closestState is passed instead of being retrieved from ctx because ctx.GetClosestState()
		//will panic if the context is done.

		m.lock.AssertValueShared()

		m._lock(closestState)
		defer m._unlock(closestState)

		defer func() {
			m.pendingInclusions = m.pendingInclusions[:0]
			m.pendingRemovals = m.pendingRemovals[:0]
		}()

		if !success {
			return
		}

		for _, inclusion := range m.pendingInclusions {
			m.entryByKey[inclusion.serializedKey] = inclusion.entry
		}

		for _, key := range m.pendingRemovals {
			delete(m.entryByKey, key)
		}

		if m.storage != nil {
			utils.PanicIfErr(persistMap(ctx, m, m.path, m.storage))
		}
	}
}

func (m *Map) makePersistOnMutationCallback(elem core.Serializable) core.MutationCallbackMicrotask {
	return func(ctx *core.Context, mutation core.Mutation) (registerAgain bool) {
		registerAgain = true

		tx := ctx.GetTx()
		if tx != nil {
			//if there is a transaction the set will be persisted when the transaction is finished.
			return
		}

		closestState := ctx.GetClosestState()
		m._lock(closestState)
		defer m._unlock(closestState)

		if !m.hasNoLock(ctx, elem) {
			registerAgain = false
			return
		}

		utils.PanicIfErr(persistMap(ctx, m, m.path, m.storage))

		return
	}
}

type inclusion struct {
	entry         entry
	serializedKey string
}

type entry struct {
	key   core.Serializable
	value core.Serializable
}
