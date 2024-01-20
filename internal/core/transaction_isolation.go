package core

import "sync"

type TransactionIsolator struct {
	currentTransaction     *Transaction
	currentTransactionLock sync.Mutex
}

func (isolator *TransactionIsolator) WaitIfOtherTransaction(ctx *Context, requireRunningTransaction bool) error {
	isolator.currentTransactionLock.Lock()
	unlock := true
	defer func() {
		if unlock {
			isolator.currentTransactionLock.Unlock()
		}
	}()

	tx := ctx.GetTx()

	if requireRunningTransaction && (tx == nil || tx.IsFinished()) {
		panic(ErrRunningTransactionExpected)
	}

	if isolator.currentTransaction != nil && isolator.currentTransaction.IsFinished() {
		isolator.currentTransaction = nil
	}

	if isolator.currentTransaction == nil {
		if tx != nil && !tx.IsFinished() {
			isolator.currentTransaction = tx
		}
		return nil
	}

	if tx != isolator.currentTransaction {
		select {
		case <-isolator.currentTransaction.WaitFinished():
		case <-ctx.Done():
			return ctx.Err()
		}

		isolator.currentTransaction = nil
		unlock = false
		isolator.currentTransactionLock.Unlock()
		return isolator.WaitIfOtherTransaction(ctx, requireRunningTransaction)
	}

	return nil
}
