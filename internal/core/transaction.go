package core

import (
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/utils"
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
	ErrFinishingTransaction                    = errors.New("transaction is finishing")
	ErrAlreadySetTransactionEndCallback        = errors.New("transaction end callback is already set")
	ErrRunningTransactionExpected              = errors.New("running transaction expected")
	ErrEffectsNotAllowedInReadonlyTransaction  = errors.New("effects are not allowed in a readonly transaction")

	// closedchan is a reusable closed channel.
	closedchan = make(chan struct{})
)

func init() {
	close(closedchan)
}

// A Transaction is analogous to a database transaction but behaves a little bit differently.
// A Transaction can be started, commited and rolled back. Effects (reversible or not) such as FS changes are added to it.
// Actual database transactions or data containers can also register a callback with the OnEnd method, in order to execute logic
// when the transaction commits or rolls back.
type Transaction struct {
	ulid           ULID
	ctx            *Context
	lock           sync.RWMutex
	startTime      time.Time
	endTime        time.Time
	effects        []Effect
	values         map[any]any
	endCallbackFns map[any]TransactionEndCallbackFn
	finished       atomic.Value //chan
	finishing      atomic.Bool
	timeout        Duration
	isReadonly     bool
}

type TransactionEndCallbackFn func(tx *Transaction, success bool)

// newTransaction creates a new empty unstarted transaction.
// ctx will not be aware of it until the transaction is started.
func newTransaction(ctx *Context, readonly bool, options ...Option) *Transaction {
	tx := &Transaction{
		ctx:            ctx,
		isReadonly:     readonly,
		ulid:           NewULID(),
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
	tx := newTransaction(ctx, false, options...)
	tx.Start(ctx)
	return tx
}

// StartNewReadonlyTransaction creates a new readonly transaction and starts it immediately.
func StartNewReadonlyTransaction(ctx *Context, options ...Option) *Transaction {
	tx := newTransaction(ctx, true, options...)
	tx.Start(ctx)
	return tx
}

// Finished returns an unbuffered channel that closes when tx is finished.
func (tx *Transaction) Finished() DoneChan {
	d := tx.finished.Load()
	if d != nil {
		return d.(chan struct{})
	}
	tx.lock.Lock()
	defer tx.lock.Unlock()
	d = tx.finished.Load()
	if d == nil {
		d = make(chan struct{})
		tx.finished.Store(d)
	}
	return d.(chan struct{})
}

func (tx *Transaction) IsFinished() bool {
	select {
	case <-tx.Finished():
		return true
	default:
		return false
	}
}

func (tx *Transaction) IsFinishing() bool {
	return tx.finishing.Load()
}

func (tx *Transaction) IsReadonly() bool {
	return tx.isReadonly
}

func (tx *Transaction) ID() ULID {
	return tx.ulid
}

// Start attaches tx to the passed context and creates a goroutine that will roll it back on timeout or context cancellation.
// The passed context must be the same context that created the transaction.
// ErrFinishedTransaction will be returned if Start is called on a finished transaction.
func (tx *Transaction) Start(ctx *Context) error {
	if tx.IsFinished() {
		return ErrFinishedTransaction
	}

	if tx.IsFinishing() {
		return ErrFinishingTransaction
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

	if tx.IsFinishing() {
		return ErrFinishingTransaction
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

	if tx.IsFinishing() {
		return ErrFinishingTransaction
	}

	if tx.isReadonly {
		return ErrEffectsNotAllowedInReadonlyTransaction
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

	if tx.IsFinished() {
		return ErrFinishedTransaction
	}

	if !tx.finishing.CompareAndSwap(false, true) {
		return ErrFinishingTransaction
	}

	tx.lock.Lock()
	defer func() {
		tx.ctx.setTx(nil)

		d, _ := tx.finished.Load().(chan struct{})
		if d == nil {
			tx.finished.Store(closedchan)
		} else {
			close(d)
		}

		tx.finishing.Store(true)
		tx.lock.Unlock()

		//The lock should not be released before updating tx.finished
		//because another goroutine calling Finished() could obtain
		//a different channel.
	}()

	tx.endTime = time.Now()

	for _, effect := range tx.effects {
		if err := effect.Apply(ctx); err != nil {
			for _, fn := range tx.endCallbackFns {
				fn(tx, true)
			}
			return fmt.Errorf("error when applying effet %#v: %w", effect, err)
		}
	}

	var callbackErrors []error

	for _, fn := range tx.endCallbackFns {
		func() {
			defer func() {
				if e := recover(); e != nil {
					defer utils.Recover()
					err := fmt.Errorf("%w: %s", utils.ConvertPanicValueToError(e), string(debug.Stack()))
					callbackErrors = append(callbackErrors, err)
				}
			}()
			fn(tx, true)
		}()
	}

	tx.endCallbackFns = nil

	return utils.CombineErrorsWithPrefixMessage("callback errors", callbackErrors...)
}

func (tx *Transaction) Rollback(ctx *Context) error {
	//$ctx may be done.

	if tx.IsFinished() {
		return ErrFinishedTransaction
	}

	if !tx.finishing.CompareAndSwap(false, true) {
		return ErrFinishingTransaction
	}

	tx.lock.Lock()
	defer func() {
		tx.ctx.setTx(nil)

		d, _ := tx.finished.Load().(chan struct{})
		if d == nil {
			tx.finished.Store(closedchan)
		} else {
			close(d)
		}

		tx.finishing.Store(true)
		tx.lock.Unlock()

		//The lock should not be released before updating tx.finished
		//because another goroutine calling Finished() could obtain
		//a different channel.
	}()

	tx.endTime = time.Now()

	var callbackErrors []error
	for _, fn := range tx.endCallbackFns {
		func() {
			defer func() {
				if e := recover(); e != nil {
					defer utils.Recover()
					err := fmt.Errorf("%w: %s", utils.ConvertPanicValueToError(e), string(debug.Stack()))
					callbackErrors = append(callbackErrors, err)
				}
			}()
			fn(tx, false)
		}()
	}

	tx.endCallbackFns = nil

	for _, effect := range tx.effects {
		if err := effect.Reverse(ctx); err != nil {
			return err
		}
	}

	return utils.CombineErrorsWithPrefixMessage("callback errors", callbackErrors...)
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
		return WrapGoMethod(tx.Start), true
	case "commit":
		return WrapGoMethod(tx.Commit), true
	case "rollback":
		return WrapGoMethod(tx.Rollback), true
	}
	return nil, false
}

type DoneChan <-chan struct{}

func (c DoneChan) IsDone() bool {
	select {
	case <-c:
		return true
	default:
		return false
	}
}

func (c DoneChan) IsNotDone() bool {
	return !c.IsDone()
}
