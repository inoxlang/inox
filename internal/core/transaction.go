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
	ErrTransactionAlreadyStarted   = errors.New("transaction has already started")
	ErrCannotAddIrreversibleEffect = errors.New("cannot add irreversible effect to transaction")
	ErrCtxAlreadyHasTransaction    = errors.New("context already has a transaction")
	ErrFinishedTransaction         = errors.New("transaction is finished")
)

// A Transaction represents a series of effects that are applied atomically.
type Transaction struct {
	NotClonableMixin

	ulid           ulid.ULID
	ctx            *Context
	lock           sync.RWMutex
	startTime      time.Time
	endTime        time.Time
	effects        []Effect
	values         map[any]any
	endCallbackFns map[any]func(*Transaction, bool)
	finished       uint32
	timeout        Duration
	isReadonly     bool
}

func newTransaction(ctx *Context, options ...Option) *Transaction {
	tx := &Transaction{
		ctx:            ctx,
		values:         make(map[any]any),
		endCallbackFns: make(map[any]func(*Transaction, bool)),
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
	return atomic.LoadUint32(&tx.finished) == 1
}

func (tx *Transaction) Start(ctx *Context) error {
	if tx.IsFinished() {
		return ErrFinishedTransaction
	}

	if ctx != tx.ctx {
		panic(errors.New("a transaction should be started by the same context that created it"))
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

func (tx *Transaction) SetValue(k, v any) error {
	if tx.IsFinished() {
		return ErrFinishedTransaction
	}

	tx.lock.Lock()
	defer tx.lock.Unlock()
	tx.values[k] = v
	return nil
}

func (tx *Transaction) GetValue(k any) (any, error) {
	if tx.IsFinished() {
		return nil, ErrFinishedTransaction
	}

	tx.lock.RLock()
	defer tx.lock.RUnlock()
	return tx.values[k], nil
}

func (tx *Transaction) OnEnd(k any, fn func(*Transaction, bool)) error {
	if tx.IsFinished() {
		return ErrFinishedTransaction
	}

	tx.lock.Lock()
	defer tx.lock.Unlock()

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
	if !atomic.CompareAndSwapUint32(&tx.finished, 0, 1) {
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
	if !atomic.CompareAndSwapUint32(&tx.finished, 0, 1) {
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
