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

type TransactionIsolator struct {
	currentWriteTx       *Transaction
	readonlyTxs          []DoneChan  //Transactions are not stored to avoid having references.
	readonlyTxsWaitTimer *time.Timer //Timer used for timing out the current write tx waiting for readonly txs to finish.

	//This lock only protects the fields of TransactionIsolator.
	//It is not used for transaction isolation.
	lock sync.Mutex
}

func (isolator *TransactionIsolator) hasUnfinishedReadonlyTxs() bool {
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

func (isolator *TransactionIsolator) WaitIfOtherTransaction(ctx *Context, requireRunningTransaction bool) error {
	return isolator.waitIfOtherTransaction(ctx, requireRunningTransaction, 0)
}

func (isolator *TransactionIsolator) waitIfOtherTransaction(ctx *Context, requireRunningTransaction bool, depth int) error {
	if depth >= MAX_SUBSEQUENT_WAIT_WRITE_TX_COUNT {
		return ErrTooManyWriteTxsWaited
	}

	isolator.lock.Lock()
	needUnlock := true
	defer func() {
		if needUnlock {
			isolator.lock.Unlock()
		}
	}()

	tx := ctx.GetTx()

	if requireRunningTransaction && (tx == nil || tx.IsFinished()) {
		panic(ErrRunningTransactionExpected)
	}

	isolator.removeFinishedTxs()

	if isolator.currentWriteTx == nil {
		if tx == nil || tx.IsFinished() {
			return nil
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
			return nil
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
					return ErrWaitReadonlyTxsTimeout
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

		return nil
	}

	if tx == isolator.currentWriteTx {
		return nil
	}

	needUnlock = false
	isolator.lock.Unlock()

	currentWriteTx := isolator.currentWriteTx

	//Wait for currentWriteTx to finish.
	select {
	case <-currentWriteTx.Finished():
	case <-ctx.Done():
		return ctx.Err()
	}

	return isolator.waitIfOtherTransaction(ctx, requireRunningTransaction, depth+1)
}

func (isolator *TransactionIsolator) removeFinishedTxs() {
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
