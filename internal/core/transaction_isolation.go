package core

import (
	"errors"
	"slices"
	"sync"
	"time"
)

const (
	MAX_SUBSEQUENT_WAIT_WRITE_TX_COUNT = 100
	WAIT_FOR_READ_TXS_TIMEOUT          = 2 * time.Second
)

var (
	ErrTooManyWriteTxsWaited  = errors.New("transaction has waited for too many write transactions to finish")
	ErrWaitReadonlyTxsTimeout = errors.New("waiting for readonly txs timed out")
)

type LiteTransactionIsolator struct {
	currentWriteTx *Transaction

	//This lock only protects the fields of LiteTransactionIsolator.
	//It is not used for transaction isolation.
	lock sync.Mutex
}

// WaitForOtherReadWriteTxToTerminate waits for the currently tracked read-write transaction to terminate if the context's transaction
// is read-write. The function does not wait if no read-write transaction is tracked, or if the context's transaction is readonly.
// When the currently tracked read-write transaction terminates, a random transaction among all waiting transactions resumes.
// In other words the first transaction to start waiting is not necessarily the one to resume first.
func (isolator *LiteTransactionIsolator) WaitForOtherReadWriteTxToTerminate(ctx *Context, requireRunningTx bool) (currentTx *Transaction, _ error) {
	return isolator.waitForOtherReadWriteTxToTerminate(ctx, requireRunningTx, 0)
}

func (isolator *LiteTransactionIsolator) waitForOtherReadWriteTxToTerminate(ctx *Context, requireRunningTx bool, depth int) (currentTx *Transaction, _ error) {
	// TODO: error for waiting too long for the current read-write tx to terminate.
	if depth >= MAX_SUBSEQUENT_WAIT_WRITE_TX_COUNT {
		return nil, ErrTooManyWriteTxsWaited
	}

	isolator.lock.Lock()
	needUnlock := true
	defer func() {
		if needUnlock {
			isolator.lock.Unlock()
		}
	}()

	tx := ctx.GetTx()

	if requireRunningTx && (tx == nil || tx.IsFinished()) {
		panic(ErrRunningTransactionExpected)
	}

	if isolator.currentWriteTx != nil && isolator.currentWriteTx.IsFinished() {
		isolator.currentWriteTx = nil
	}

	if isolator.currentWriteTx == nil {
		if tx == nil || tx.IsFinished() {
			return nil, nil
		}
		if !tx.IsReadonly() {
			isolator.currentWriteTx = tx
		}
		return tx, nil
	}

	if tx == isolator.currentWriteTx || tx.IsReadonly() {
		return tx, nil
	}

	currentWriteTx := isolator.currentWriteTx

	needUnlock = false
	isolator.lock.Unlock()

	//Wait for currentWriteTx to finish.
	select {
	case <-currentWriteTx.Finished():
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return isolator.waitForOtherReadWriteTxToTerminate(ctx, requireRunningTx, depth+1)
}

type StrongTransactionIsolator struct {
	currentWriteTx       *Transaction
	readonlyTxs          []DoneChan  //Transactions are not stored to avoid having references.
	readonlyTxsWaitTimer *time.Timer //Timer used for timing out the current write tx waiting for readonly txs to finish.

	//This lock only protects the fields of StrongTransactionIsolator.
	//It is not used for transaction isolation.
	lock sync.Mutex
}

func (isolator *StrongTransactionIsolator) hasUnfinishedReadonlyTxs() bool {
	isolator.lock.Lock()
	defer isolator.lock.Unlock()

	for _, doneChan := range isolator.readonlyTxs {
		select {
		case <-doneChan:
		default:
			return true
		}
	}

	return false
}

// WaitForOtherTxsToTerminate waits for specific transactions tracked by the isolator to terminate, it returns $ctx's transaction (can be nil).
// If $ctx has no transaction the call will only wait if there is a read-write transaction.
// Readonly transactions do not have to wait if only readonly transactions are tracked by the isolator.
// Read-write transactions have to wait for all readonly transactions, or the currently tracked read-write transaction, to terminate.
// ErrWaitReadonlyTxsTimeout is returned if too much time is spent waiting for readonly transaction to terminate.
// TODO: add similar error for waiting too long for the current read-write tx to terminate.
// When the currently tracked read-write transaction terminates, a random transaction among all waiting transactions resumes.
// In other words the first transaction to start waiting is not necessarily the one to resume first.
// ErrRunningTransactionExpected is returned if requireRunningTx is true and $ctx has no tx.
func (isolator *StrongTransactionIsolator) WaitForOtherTxsToTerminate(ctx *Context, requireRunningTx bool) (currentTx *Transaction, _ error) {
	return isolator.waitForReadWriteTxToTerminate(ctx, requireRunningTx, 0)
}

func (isolator *StrongTransactionIsolator) waitForReadWriteTxToTerminate(ctx *Context, requireRunningTx bool, depth int) (currentTx *Transaction, _ error) {
	if depth >= MAX_SUBSEQUENT_WAIT_WRITE_TX_COUNT {
		return nil, ErrTooManyWriteTxsWaited
	}

	isolator.lock.Lock()
	needUnlock := true
	defer func() {
		if needUnlock {
			isolator.lock.Unlock()
		}
	}()

	tx := ctx.GetTx()

	if requireRunningTx && (tx == nil || tx.IsFinished()) {
		panic(ErrRunningTransactionExpected)
	}

	isolator.removeFinishedTxs()

	if isolator.currentWriteTx == nil {
		if tx == nil || tx.IsFinished() {
			return nil, nil
		}

		if tx.IsReadonly() {
			//Add the Finished() chan of the tx to the list of readonly transactions that future
			//write txs will have to wait.
			finishedChan := tx.Finished()
			if !slices.Contains(isolator.readonlyTxs, finishedChan) {
				isolator.readonlyTxs = append(isolator.readonlyTxs, finishedChan)
			}

			needUnlock = false
			isolator.lock.Unlock()
			return tx, nil
		}
		//read-write transaction.

		isolator.currentWriteTx = tx

		//Wait for all readonly txs to finish.
		if slices.ContainsFunc(isolator.readonlyTxs, DoneChan.IsNotDone) {

			readTxs := isolator.readonlyTxs
			if isolator.readonlyTxsWaitTimer == nil {
				isolator.readonlyTxsWaitTimer = time.NewTimer(WAIT_FOR_READ_TXS_TIMEOUT)
			} else {
				isolator.readonlyTxsWaitTimer.Reset(WAIT_FOR_READ_TXS_TIMEOUT)
			}

			readonlyTxsWaitTimer := isolator.readonlyTxsWaitTimer

			needUnlock = false
			isolator.lock.Unlock()

			for _, doneChan := range readTxs {
				select {
				case <-doneChan:
				case <-readonlyTxsWaitTimer.C:
					return nil, ErrWaitReadonlyTxsTimeout
				}
			}

			isolator.lock.Lock()
			defer isolator.lock.Unlock()
			//? locking is not necessary because other read-write txs should never
			//execute code in the branch `isolator.currentWriteTx == nil`.

			//Stop and drain the timer.
			readonlyTxsWaitTimer.Stop()
			select {
			case <-readonlyTxsWaitTimer.C:
			default:
			}
		} else {
			needUnlock = false
			isolator.lock.Unlock()
		}

		return tx, nil
	}

	if tx == isolator.currentWriteTx {
		return tx, nil
	}

	currentWriteTx := isolator.currentWriteTx
	needUnlock = false
	isolator.lock.Unlock()

	//Wait for currentWriteTx to finish.
	select {
	case <-currentWriteTx.Finished():
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return isolator.waitForReadWriteTxToTerminate(ctx, requireRunningTx, depth+1)
}

func (isolator *StrongTransactionIsolator) removeFinishedTxs() {
	if isolator.currentWriteTx != nil && isolator.currentWriteTx.IsFinished() {
		isolator.currentWriteTx = nil
	}

	i := 0

	for i < len(isolator.readonlyTxs) {
		if isolator.readonlyTxs[i].IsDone() {
			//Remove the position.
			copy(isolator.readonlyTxs[i:], isolator.readonlyTxs[i+1:])
			isolator.readonlyTxs = isolator.readonlyTxs[:len(isolator.readonlyTxs)-1]
			continue
		}
		i++
	}
}
