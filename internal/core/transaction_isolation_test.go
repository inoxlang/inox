package core

import (
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestTransactionIsolator(t *testing.T) {

	t.Run("a readonly transaction should not have to wait for other read transactions to finish", func(t *testing.T) {
		readCtx1 := NewContexWithEmptyState(ContextConfig{}, nil)
		StartNewReadonlyTransaction(readCtx1)
		defer readCtx1.CancelGracefully()

		readCtx2 := NewContexWithEmptyState(ContextConfig{}, nil)
		StartNewReadonlyTransaction(readCtx2)
		defer readCtx2.CancelGracefully()

		isolator := &TransactionIsolator{}

		assert.NoError(t, utils.Ret1(isolator.WaitForOtherTxsToTerminate(readCtx1, false)))
		assert.NoError(t, utils.Ret1(isolator.WaitForOtherTxsToTerminate(readCtx2, false)))
		assert.NoError(t, utils.Ret1(isolator.WaitForOtherTxsToTerminate(readCtx1, false)))
		assert.NoError(t, utils.Ret1(isolator.WaitForOtherTxsToTerminate(readCtx2, false)))
	})

	t.Run("a readonly transaction should wait for the current write transaction to finish", func(t *testing.T) {
		writeCtx := NewContexWithEmptyState(ContextConfig{}, nil)
		writeTx := StartNewTransaction(writeCtx)
		defer writeCtx.CancelGracefully()

		readCtx := NewContexWithEmptyState(ContextConfig{}, nil)
		StartNewReadonlyTransaction(readCtx)
		defer readCtx.CancelGracefully()

		isolator := &TransactionIsolator{}

		goRoutineStarted := make(chan struct{})
		afterCall := make(chan struct{})
		assert.NoError(t, utils.Ret1(isolator.WaitForOtherTxsToTerminate(writeCtx, false)))

		go func() {
			goRoutineStarted <- struct{}{}
			assert.NoError(t, utils.Ret1(isolator.WaitForOtherTxsToTerminate(readCtx, false)))
			afterCall <- struct{}{}
		}()

		<-goRoutineStarted

		time.Sleep(time.Millisecond)
		select {
		case <-afterCall:
			assert.Fail(t, "read tx should be waiting for the write tx to finish")
		default:
		}

		assert.NoError(t, utils.Ret1(isolator.WaitForOtherTxsToTerminate(writeCtx, false)))
		time.Sleep(time.Millisecond)

		select {
		case <-afterCall:
			assert.Fail(t, "read tx should be waiting for the write tx to finish")
		default:
		}

		assert.NoError(t, writeTx.Commit(writeCtx))

		select {
		case <-afterCall:
		case <-time.After(5 * time.Millisecond):
			assert.Fail(t, "read tx should not be waiting")
		}
	})

	t.Run("a write transaction should wait for readonly transactions to finish", func(t *testing.T) {
		readCtx1 := NewContexWithEmptyState(ContextConfig{}, nil)
		readTx1 := StartNewReadonlyTransaction(readCtx1)
		defer readCtx1.CancelGracefully()

		readCtx2 := NewContexWithEmptyState(ContextConfig{}, nil)
		readTx2 := StartNewReadonlyTransaction(readCtx2)
		defer readCtx2.CancelGracefully()

		writeCtx := NewContexWithEmptyState(ContextConfig{}, nil)
		StartNewTransaction(writeCtx)
		defer writeCtx.CancelGracefully()

		isolator := &TransactionIsolator{}

		goRoutineStarted := make(chan struct{})
		afterCall := make(chan struct{})
		assert.NoError(t, utils.Ret1(isolator.WaitForOtherTxsToTerminate(readCtx1, false)))
		assert.NoError(t, utils.Ret1(isolator.WaitForOtherTxsToTerminate(readCtx2, false)))

		go func() {
			goRoutineStarted <- struct{}{}
			assert.NoError(t, utils.Ret1(isolator.WaitForOtherTxsToTerminate(writeCtx, false)))
			afterCall <- struct{}{}
		}()

		<-goRoutineStarted
		time.Sleep(10 * time.Millisecond)

		select {
		case <-afterCall:
			assert.Fail(t, "write tx should be waiting for the read txs to finish")
		default:
		}

		assert.NoError(t, readTx1.Commit(readCtx1))
		time.Sleep(10 * time.Millisecond)

		select {
		case <-afterCall:
			assert.Fail(t, "write tx should be waiting for the remaining read tx to finish")
		default:
		}

		assert.NoError(t, readTx2.Commit(readCtx2))
		time.Sleep(10 * time.Millisecond)

		select {
		case <-afterCall:
		case <-time.After(10 * time.Millisecond):
			assert.Fail(t, "write tx should not be waiting")
		}
	})

	t.Run("a write transaction should time out waiting for readonly transactions to finish", func(t *testing.T) {
		readCtx1 := NewContexWithEmptyState(ContextConfig{}, nil)
		StartNewReadonlyTransaction(readCtx1)
		defer readCtx1.CancelGracefully()

		readCtx2 := NewContexWithEmptyState(ContextConfig{}, nil)
		StartNewReadonlyTransaction(readCtx2)
		defer readCtx2.CancelGracefully()

		writeCtx := NewContexWithEmptyState(ContextConfig{}, nil)
		StartNewTransaction(writeCtx)
		defer writeCtx.CancelGracefully()

		isolator := &TransactionIsolator{}

		goRoutineStarted := make(chan struct{})
		afterCalls := make(chan struct{})

		go func() {
			//readonly txs.
			goRoutineStarted <- struct{}{}
			assert.NoError(t, utils.Ret1(isolator.WaitForOtherTxsToTerminate(readCtx1, false)))
			assert.NoError(t, utils.Ret1(isolator.WaitForOtherTxsToTerminate(readCtx2, false)))
			afterCalls <- struct{}{}
		}()

		<-goRoutineStarted
		<-afterCalls
		assert.ErrorIs(t, utils.Ret1(isolator.WaitForOtherTxsToTerminate(writeCtx, false)), ErrWaitReadonlyTxsTimeout)
	})
}
