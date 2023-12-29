package containers

import (
	"errors"
	"path/filepath"
	"reflect"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"

	"github.com/inoxlang/inox/internal/globals/containers/common"
	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
)

var (
	ErrSetCanOnlyContainRepresentableValues = errors.New("a Set can only contain representable values")
	ErrValueDoesMatchElementPattern         = errors.New("provided value does not match the element pattern")
	ErrValueWithSameKeyAlreadyPresent       = errors.New("provided value has the same key as an already present element")

	_ core.DefaultValuePattern   = (*SetPattern)(nil)
	_ core.MigrationAwarePattern = (*SetPattern)(nil)
	_ core.PotentiallySharable   = (*Set)(nil)
	_ core.SerializableIterable  = (*Set)(nil)
	_ core.MigrationCapable      = (*Set)(nil)
)

func init() {
	core.RegisterLoadInstanceFn(reflect.TypeOf((*SetPattern)(nil)), loadSet)
}

type Set struct {
	elements                       map[string]core.Serializable
	pathKeyToKey                   map[core.ElementKey]string
	pendingInclusions              map[*core.Transaction]map[string]core.Serializable
	pendingRemovals                map[*core.Transaction]map[string]struct{}
	transactionsWithSetEndCallback map[*core.Transaction]struct{}
	lock                           core.SmartLock

	config  SetConfig
	pattern *SetPattern

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
		elements:                       make(map[string]core.Serializable),
		pathKeyToKey:                   make(map[core.ElementKey]string),
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

func (s *Set) IsSharable(originState *core.GlobalState) (bool, string) {
	return true, ""
}

func (s *Set) Share(originState *core.GlobalState) {
	s.lock.Share(originState, func() {})
}

func (s *Set) IsShared() bool {
	return s.lock.IsValueShared()
}

func (s *Set) Lock(state *core.GlobalState) {
	s.lock.Lock(state, s)
}

func (s *Set) Unlock(state *core.GlobalState) {
	s.lock.Unlock(state, s)
}

func (s *Set) ForceLock() {
	s.lock.ForceLock()
}

func (s *Set) ForceUnlock() {
	s.lock.ForceUnlock()
}

func (set *Set) URL() (core.URL, bool) {
	if set.storage != nil {
		return set.url, true
	}
	return "", false
}

func (set *Set) GetElementPathKeyFromKey(key string) core.ElementKey {
	return common.GetElementPathKeyFromKey(key, set.config.Uniqueness.Type)
}

func (set *Set) SetURLOnce(ctx *core.Context, url core.URL) error {
	return core.ErrValueDoesNotAcceptURL
}

func (set *Set) Has(ctx *core.Context, elem core.Serializable) core.Bool {
	closestState := ctx.GetClosestState()
	set.lock.Lock(closestState, set)
	defer set.lock.Unlock(closestState, set)

	return set.hasNoLock(ctx, elem)
}

func (set *Set) hasNoLock(ctx *core.Context, elem core.Serializable) core.Bool {
	if set.config.Element != nil && !set.config.Element.Test(ctx, elem) {
		panic(ErrValueDoesMatchElementPattern)
	}

	key := common.GetUniqueKey(ctx, elem, set.config.Uniqueness, set)

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

	_, ok := set.elements[key]
	return core.Bool(ok)
}

func (set *Set) Get(ctx *core.Context, keyVal core.StringLike) (core.Value, core.Bool) {
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

	elem, ok := set.elements[key]
	if !ok {
		return nil, false
	}

	return elem, true
}

func (set *Set) Add(ctx *core.Context, elem core.Serializable) {
	set.addNoPersist(ctx, elem)

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

func (set *Set) addNoPersist(ctx *core.Context, elem core.Serializable) {
	if set.config.Element != nil && !set.config.Element.Test(ctx, elem) {
		panic(ErrValueDoesMatchElementPattern)
	}

	closestState := ctx.GetClosestState()
	elem = utils.Must(core.ShareOrClone(elem, closestState)).(core.Serializable)

	set.config.Uniqueness.AddUrlIfNecessary(ctx, set, elem)
	key := common.GetUniqueKey(ctx, elem, set.config.Uniqueness, set)

	set.lock.Lock(closestState, set)
	defer set.lock.Unlock(closestState, set)

	set.pathKeyToKey[set.GetElementPathKeyFromKey(key)] = key
	//TODO: from time to time .pathKeyToKey should be (safely !) cleaned up

	tx := ctx.GetTx()

	if tx == nil {
		if _, ok := set.elements[key]; ok {
			panic(ErrValueWithSameKeyAlreadyPresent)
		}
		set.elements[key] = elem
	} else {
		pendingInclusions := set.pendingInclusions[tx]
		_, added := pendingInclusions[key]
		if added {
			panic(ErrValueWithSameKeyAlreadyPresent)
		}

		curr, ok := set.elements[key]
		if ok && elem != curr {
			panic(ErrValueWithSameKeyAlreadyPresent)
		}

		pendingRemovals := set.pendingRemovals[tx]
		_, removed := pendingRemovals[key]
		if removed {
			delete(pendingRemovals, key)
		} else if _, ok := set.elements[key]; ok {
			panic(ErrValueWithSameKeyAlreadyPresent)
		}

		//add the element

		if pendingInclusions == nil {
			pendingInclusions = make(map[string]core.Serializable)
			set.pendingInclusions[tx] = pendingInclusions
		}

		pendingInclusions[key] = elem
	}

}

func (set *Set) Remove(ctx *core.Context, elem core.Serializable) {
	key := common.GetUniqueKey(ctx, elem, set.config.Uniqueness, set)

	closestState := ctx.GetClosestState()
	set.lock.Lock(closestState, set)
	defer set.lock.Unlock(closestState, set)

	tx := ctx.GetTx()

	if tx == nil {
		delete(set.elements, key)
		if set.storage != nil {
			utils.PanicIfErr(persistSet(ctx, set, set.path, set.storage))
		}
	} else {
		pendingRemovals, ok := set.pendingRemovals[tx]
		if !ok {
			pendingRemovals = make(map[string]struct{})
			set.pendingRemovals[tx] = pendingRemovals
		}

		pendingRemovals[key] = struct{}{}

		if _, ok := set.transactionsWithSetEndCallback[tx]; !ok {
			tx.OnEnd(set, set.makeTransactionEndCallback(ctx, closestState))
			set.transactionsWithSetEndCallback[tx] = struct{}{}
		}
	}
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
			set.elements[key] = value
		}

		for key := range set.pendingRemovals[tx] {
			delete(set.elements, key)
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

func (f *Set) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "has":
		return core.WrapGoMethod(f.Has), true
	case "add":
		return core.WrapGoMethod(f.Add), true
	case "remove":
		return core.WrapGoMethod(f.Remove), true
	case "get":
		return core.WrapGoMethod(f.Get), true
	}
	return nil, false
}

func (s *Set) Prop(ctx *core.Context, name string) core.Value {
	method, ok := s.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, s))
	}
	return method
}

func (*Set) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*Set) PropertyNames(ctx *core.Context) []string {
	return coll_symbolic.SET_PROPNAMES
}

type SetPattern struct {
	config SetConfig

	core.CallBasedPatternReprMixin

	core.NotCallablePatternMixin
}

func NewSetPattern(config SetConfig, callData core.CallBasedPatternReprMixin) *SetPattern {
	if config.Element == nil {
		config.Element = core.SERIALIZABLE_PATTERN
	}
	return &SetPattern{
		config:                    config,
		CallBasedPatternReprMixin: callData,
	}
}

func (patt *SetPattern) Test(ctx *core.Context, v core.Value) bool {
	set, ok := v.(*Set)
	if !ok {
		return false
	}

	return patt.config.Equal(ctx, set.config, map[uintptr]uintptr{}, 0)
}
func (p *SetPattern) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	return core.NewEmptyPatternIterator()
}

func (p *SetPattern) Random(ctx *core.Context, options ...core.Option) core.Value {
	panic(core.ErrNotImplementedYet)
}

func (p *SetPattern) StringPattern() (core.StringPattern, bool) {
	return nil, false
}

func (p *SetPattern) DefaultValue(ctx *core.Context) (core.Value, error) {
	return NewSetWithConfig(ctx, nil, p.config), nil
}

func (p *SetPattern) GetMigrationOperations(ctx *core.Context, next core.Pattern, pseudoPath string) ([]core.MigrationOp, error) {
	nextSet, ok := next.(*SetPattern)
	if !ok || nextSet.config.Uniqueness != p.config.Uniqueness {
		return []core.MigrationOp{core.ReplacementMigrationOp{
			Current:        p,
			Next:           next,
			MigrationMixin: core.MigrationMixin{PseudoPath: pseudoPath},
		}}, nil
	}

	return core.GetMigrationOperations(ctx, p.config.Element, nextSet.config.Element, filepath.Join(pseudoPath, "*"))
}
