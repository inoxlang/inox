package core

import (
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestLiteTransactionIsolator(t *testing.T) {

	t.Run("a readonly transaction should not have to wait for other read transactions to finish", func(t *testing.T) {
		readCtx1 := NewContextWithEmptyState(ContextConfig{}, nil)
		StartNewReadonlyTransaction(readCtx1)
		defer readCtx1.CancelGracefully()

		readCtx2 := NewContextWithEmptyState(ContextConfig{}, nil)
		StartNewReadonlyTransaction(readCtx2)
		defer readCtx2.CancelGracefully()

		isolator := &LiteTransactionIsolator{}

		assert.NoError(t, utils.Ret1(isolator.WaitForOtherReadWriteTxToTerminate(readCtx1, false)))
		assert.NoError(t, utils.Ret1(isolator.WaitForOtherReadWriteTxToTerminate(readCtx2, false)))
		assert.NoError(t, utils.Ret1(isolator.WaitForOtherReadWriteTxToTerminate(readCtx1, false)))
		assert.NoError(t, utils.Ret1(isolator.WaitForOtherReadWriteTxToTerminate(readCtx2, false)))
	})

	t.Run("a write transaction should wait for the current writr transaction to finish", func(t *testing.T) {
		writeCtx1 := NewContextWithEmptyState(ContextConfig{}, nil)
		writeTx1 := StartNewTransaction(writeCtx1)
		defer writeCtx1.CancelGracefully()

		writeCtx2 := NewContextWithEmptyState(ContextConfig{}, nil)
		StartNewTransaction(writeCtx2)
		defer writeCtx2.CancelGracefully()

		isolator := &LiteTransactionIsolator{}

		goRoutineStarted := make(chan struct{})
		afterCall := make(chan struct{})
		assert.NoError(t, utils.Ret1(isolator.WaitForOtherReadWriteTxToTerminate(writeCtx1, false)))

		go func() {
			goRoutineStarted <- struct{}{}
			assert.NoError(t, utils.Ret1(isolator.WaitForOtherReadWriteTxToTerminate(writeCtx2, false)))
			afterCall <- struct{}{}
		}()

		<-goRoutineStarted
		time.Sleep(10 * time.Millisecond)

		select {
		case <-afterCall:
			assert.Fail(t, "tx2 should be waiting for the current read-write tx (tx1) to finish")
		default:
		}

		assert.NoError(t, writeTx1.Commit(writeCtx1))
		time.Sleep(10 * time.Millisecond)

		select {
		case <-afterCall:
		case <-time.After(10 * time.Millisecond):
			assert.Fail(t, "tx2 should not be waiting")
		}
	})

	t.Run("a readonly transaction should not wait for the current write transaction to finish", func(t *testing.T) {
		writeCtx := NewContextWithEmptyState(ContextConfig{}, nil)
		writeTx := StartNewTransaction(writeCtx)
		defer writeCtx.CancelGracefully()

		readCtx := NewContextWithEmptyState(ContextConfig{}, nil)
		StartNewReadonlyTransaction(readCtx)
		defer readCtx.CancelGracefully()

		isolator := &LiteTransactionIsolator{}

		assert.NoError(t, utils.Ret1(isolator.WaitForOtherReadWriteTxToTerminate(writeCtx, false)))
		assert.NoError(t, utils.Ret1(isolator.WaitForOtherReadWriteTxToTerminate(readCtx, false)))

		assert.NoError(t, writeTx.Commit(writeCtx))
		assert.NoError(t, utils.Ret1(isolator.WaitForOtherReadWriteTxToTerminate(readCtx, false)))
	})

	t.Run("a write transaction should not wait for readonly transactions to finish", func(t *testing.T) {
		readCtx1 := NewContextWithEmptyState(ContextConfig{}, nil)
		readTx1 := StartNewReadonlyTransaction(readCtx1)
		defer readCtx1.CancelGracefully()

		readCtx2 := NewContextWithEmptyState(ContextConfig{}, nil)
		readTx2 := StartNewReadonlyTransaction(readCtx2)
		defer readCtx2.CancelGracefully()

		writeCtx := NewContextWithEmptyState(ContextConfig{}, nil)
		StartNewTransaction(writeCtx)
		defer writeCtx.CancelGracefully()

		isolator := &LiteTransactionIsolator{}

		assert.NoError(t, utils.Ret1(isolator.WaitForOtherReadWriteTxToTerminate(readCtx1, false)))
		assert.NoError(t, utils.Ret1(isolator.WaitForOtherReadWriteTxToTerminate(readCtx2, false)))

		assert.NoError(t, utils.Ret1(isolator.WaitForOtherReadWriteTxToTerminate(writeCtx, false)))

		assert.NoError(t, readTx1.Commit(readCtx1))
		assert.NoError(t, utils.Ret1(isolator.WaitForOtherReadWriteTxToTerminate(writeCtx, false)))

		assert.NoError(t, readTx2.Commit(readCtx2))
		assert.NoError(t, utils.Ret1(isolator.WaitForOtherReadWriteTxToTerminate(writeCtx, false)))
	})

}

func TestStrongTransactionIsolator(t *testing.T) {

	t.Run("a readonly transaction should not have to wait for other read transactions to finish", func(t *testing.T) {
		readCtx1 := NewContextWithEmptyState(ContextConfig{}, nil)
		StartNewReadonlyTransaction(readCtx1)
		defer readCtx1.CancelGracefully()

		readCtx2 := NewContextWithEmptyState(ContextConfig{}, nil)
		StartNewReadonlyTransaction(readCtx2)
		defer readCtx2.CancelGracefully()

		isolator := &StrongTransactionIsolator{}

		assert.NoError(t, utils.Ret1(isolator.WaitForOtherTxsToTerminate(readCtx1, false)))
		assert.NoError(t, utils.Ret1(isolator.WaitForOtherTxsToTerminate(readCtx2, false)))
		assert.NoError(t, utils.Ret1(isolator.WaitForOtherTxsToTerminate(readCtx1, false)))
		assert.NoError(t, utils.Ret1(isolator.WaitForOtherTxsToTerminate(readCtx2, false)))
	})

	t.Run("a write transaction should wait for the current write transaction to finish", func(t *testing.T) {
		writeCtx1 := NewContextWithEmptyState(ContextConfig{}, nil)
		writeTx1 := StartNewTransaction(writeCtx1)
		defer writeCtx1.CancelGracefully()

		writeCtx2 := NewContextWithEmptyState(ContextConfig{}, nil)
		StartNewTransaction(writeCtx2)
		defer writeCtx2.CancelGracefully()

		isolator := &StrongTransactionIsolator{}

		goRoutineStarted := make(chan struct{})
		afterCall := make(chan struct{})
		assert.NoError(t, utils.Ret1(isolator.WaitForOtherTxsToTerminate(writeCtx1, false)))

		go func() {
			goRoutineStarted <- struct{}{}
			assert.NoError(t, utils.Ret1(isolator.WaitForOtherTxsToTerminate(writeCtx2, false)))
			afterCall <- struct{}{}
		}()

		<-goRoutineStarted
		time.Sleep(10 * time.Millisecond)

		select {
		case <-afterCall:
			assert.Fail(t, "tx2 should be waiting for the current read-write tx (tx1) to finish")
		default:
		}

		assert.NoError(t, writeTx1.Commit(writeCtx1))
		time.Sleep(10 * time.Millisecond)

		select {
		case <-afterCall:
		case <-time.After(10 * time.Millisecond):
			assert.Fail(t, "tx2 should not be waiting")
		}
	})

	t.Run("a readonly transaction should wait for the current write transaction to finish", func(t *testing.T) {
		writeCtx := NewContextWithEmptyState(ContextConfig{}, nil)
		writeTx := StartNewTransaction(writeCtx)
		defer writeCtx.CancelGracefully()

		readCtx := NewContextWithEmptyState(ContextConfig{}, nil)
		StartNewReadonlyTransaction(readCtx)
		defer readCtx.CancelGracefully()

		isolator := &StrongTransactionIsolator{}

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
		readCtx1 := NewContextWithEmptyState(ContextConfig{}, nil)
		readTx1 := StartNewReadonlyTransaction(readCtx1)
		defer readCtx1.CancelGracefully()

		readCtx2 := NewContextWithEmptyState(ContextConfig{}, nil)
		readTx2 := StartNewReadonlyTransaction(readCtx2)
		defer readCtx2.CancelGracefully()

		writeCtx := NewContextWithEmptyState(ContextConfig{}, nil)
		StartNewTransaction(writeCtx)
		defer writeCtx.CancelGracefully()

		isolator := &StrongTransactionIsolator{}

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
		readCtx1 := NewContextWithEmptyState(ContextConfig{}, nil)
		StartNewReadonlyTransaction(readCtx1)
		defer readCtx1.CancelGracefully()

		readCtx2 := NewContextWithEmptyState(ContextConfig{}, nil)
		StartNewReadonlyTransaction(readCtx2)
		defer readCtx2.CancelGracefully()

		writeCtx := NewContextWithEmptyState(ContextConfig{}, nil)
		StartNewTransaction(writeCtx)
		defer writeCtx.CancelGracefully()

		isolator := &StrongTransactionIsolator{}

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
