package core

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oklog/ulid/v2"
)

const (
	DEFAULT_TRANSACTION_TIMEOUT = Duration(20 * time.Second)
	TX_TIMEOUT_OPTION_NAME      = "timeout"
)

var (
	ErrTransactionAlreadyStarted               = errors.New("transaction has already started")
	ErrTransactionShouldBeStartedBySameContext = errors.New("a transaction should be started by the same context that created it")
	ErrCannotAddIrreversibleEffect             = errors.New("cannot add irreversible effect to transaction")
	ErrCtxAlreadyHasTransaction                = errors.New("context already has a transaction")
	ErrFinishedTransaction                     = errors.New("transaction is finished")
	ErrAlreadySetTransactionEndCallback        = errors.New("transaction end callback is already set")
	ErrRunningTransactionExpected              = errors.New("running transaction expected")
)

// A Transaction represents a series of effects that are applied atomically.
type Transaction struct {
	ulid           ulid.ULID
	ctx            *Context
	lock           sync.RWMutex
	startTime      time.Time
	endTime        time.Time
	effects        []Effect
	values         map[any]any
	endCallbackFns map[any]TransactionEndCallbackFn
	finished       atomic.Bool
	timeout        Duration
	isReadonly     bool
}

type TransactionEndCallbackFn func(tx *Transaction, success bool)

func newTransaction(ctx *Context, options ...Option) *Transaction {
	tx := &Transaction{
		ctx:            ctx,
		ulid:           ulid.Make(),
		values:         make(map[any]any),
		endCallbackFns: make(map[any]TransactionEndCallbackFn),
		timeout:        DEFAULT_TRANSACTION_TIMEOUT,
	}

	for _, opt := range options {
		switch opt.Name {
		case TX_TIMEOUT_OPTION_NAME:
			tx.timeout = opt.Value.(Duration)
		}
	}

	return tx
}

// StartNewTransaction creates a new transaction and starts it immediately.
func StartNewTransaction(ctx *Context, options ...Option) *Transaction {
	tx := newTransaction(ctx, options...)
	tx.Start(ctx)
	return tx
}

func (tx *Transaction) IsFinished() bool {
	return tx.finished.Load()
}

func (tx *Transaction) Start(ctx *Context) error {
	if tx.IsFinished() {
		return ErrFinishedTransaction
	}

	if ctx != tx.ctx {
		panic(ErrTransactionShouldBeStartedBySameContext)
	}

	tx.lock.Lock()
	defer tx.lock.Unlock()

	if tx.startTime != (time.Time{}) {
		panic(ErrTransactionAlreadyStarted)
	}

	if tx.ctx.currentTx != nil {
		panic(ErrCtxAlreadyHasTransaction)
	}

	// spawn a goroutine that rollbacks the transaction when the associated context is done or
	// if the timeout duration has ellapsed.
	go func() {
		select {
		case <-ctx.Done():
			tx.Rollback(ctx)
		case <-time.After(time.Duration(tx.timeout)):
			if !tx.IsFinished() {
				ctx.Logger().Print(tx.ulid.String(), "transaction timed out")
				tx.Rollback(ctx)
			}
		}
	}()

	tx.startTime = time.Now()
	tx.ctx.setTx(tx)
	return nil
}

// OnEnd associates with k the callback function fn that will be called on the end of the transacion (success or failure),
// IMPORTANT NOTE: fn may be called in a goroutine different from the one that registered it.
// If a function is already associated with k the error ErrAlreadySetTransactionEndCallback is returned
func (tx *Transaction) OnEnd(k any, fn TransactionEndCallbackFn) error {
	if tx.IsFinished() {
		return ErrFinishedTransaction
	}

	tx.lock.Lock()
	defer tx.lock.Unlock()

	_, ok := tx.endCallbackFns[k]
	if ok {
		return ErrAlreadySetTransactionEndCallback
	}

	tx.endCallbackFns[k] = fn
	return nil
}

func (tx *Transaction) AddEffect(ctx *Context, effect Effect) error {
	if tx.IsFinished() {
		return ErrFinishedTransaction
	}

	tx.lock.Lock()
	defer tx.lock.Unlock()

	if effect.Reversability(ctx) == Irreversible {
		return ErrCannotAddIrreversibleEffect
	}
	tx.effects = append(tx.effects, effect)

	return nil
}

func (tx *Transaction) Commit(ctx *Context) error {
	if !tx.finished.CompareAndSwap(false, true) {
		return ErrFinishedTransaction
	}

	tx.lock.Lock()
	defer func() {
		tx.lock.Unlock()
		tx.ctx.setTx(nil)
	}()

	tx.endTime = time.Now()

	for _, effect := range tx.effects {
		if err := effect.Apply(ctx); err != nil {
			for _, fn := range tx.endCallbackFns {
				fn(tx, false)
			}
			return fmt.Errorf("error when applying effet %#v: %w", effect, err)
		}
	}

	for _, fn := range tx.endCallbackFns {
		fn(tx, true)
	}
	return nil
}

func (tx *Transaction) Rollback(ctx *Context) error {
	if !tx.finished.CompareAndSwap(false, true) {
		return ErrFinishedTransaction
	}

	tx.lock.Lock()
	defer func() {
		tx.lock.Unlock()
		tx.ctx.setTx(nil)
	}()

	tx.endTime = time.Now()

	for _, fn := range tx.endCallbackFns {
		fn(tx, false)
	}

	for _, effect := range tx.effects {
		if err := effect.Reverse(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (tx *Transaction) WaitFinished() <-chan struct{} {
	if tx.IsFinished() {
		return nil
	}
	finishedChan := make(chan struct{})

	tx.OnEnd(finishedChan, func(tx *Transaction, success bool) {
		finishedChan <- struct{}{}
	})
	return finishedChan
}

func (tx *Transaction) Prop(ctx *Context, name string) Value {
	method, ok := tx.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, tx))
	}
	return method
}

func (*Transaction) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (tx *Transaction) PropertyNames(ctx *Context) []string {
	return []string{"start", "commit", "rollback"}
}

func (tx *Transaction) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "start":
		return &GoFunction{fn: tx.Start}, true
	case "commit":
		return &GoFunction{fn: tx.Commit}, true
	case "rollback":
		return &GoFunction{fn: tx.Rollback}, true
	}
	return nil, false
}
