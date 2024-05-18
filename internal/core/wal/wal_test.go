//https://github.com/tidwall/wal by Josh Baker, MIT licensed.
//This file includes https://github.com/tidwall/wal/pull/22 and https://github.com/tidwall/wal/pull/25.

package wal

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestLog(t *testing.T) {
	t.Run("nil-opts", func(t *testing.T) {
		testLog(t, nil, 100)
	})
	t.Run("no-sync", func(t *testing.T) {
		t.Run("binary", func(t *testing.T) {
			testLog(t, makeOpts(512, true), 100)
		})
	})
	t.Run("sync", func(t *testing.T) {
		t.Run("binary", func(t *testing.T) {
			testLog(t, makeOpts(512, false), 100)
		})
	})
}

func TestOutliers(t *testing.T) {

	// Create some scenarios where the log has been corrupted, operations
	// fail, or various weirdnesses.
	t.Run("fail-in-memory", func(t *testing.T) {
		fls := newFS()

		if l, err := OpenWAL(":memory:", fls, nil); err == nil {
			l.Close()
			t.Fatal("expected error")
		}
	})
	t.Run("fail-not-a-directory", func(t *testing.T) {
		fls := newFS()

		if err := fls.MkdirAll("/testlog", 0777); err != nil {
			t.Fatal(err)
		} else if f, err := fls.Create("/testlog/file"); err != nil {
			t.Fatal(err)
		} else if err := f.Close(); err != nil {
			t.Fatal(err)
		} else if l, err := OpenWAL("/testlog/file", fls, nil); err == nil {
			l.Close()
			t.Fatal("expected error")
		}
	})
	t.Run("load-with-junk-files", func(t *testing.T) {
		fls := newFS()

		// junk should be ignored
		if err := fls.MkdirAll("/testlog/junk/other1", 0777); err != nil {
			t.Fatal(err)
		}
		f, err := fls.Create("/testlog/junk/other2")
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
		f, err = fls.Create("/testlog/junk/" + strings.Repeat("A", 20))
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
		l, err := OpenWAL("/testlog/junk", fls, nil)
		if err != nil {
			t.Fatal(err)
		}
		l.Close()
	})

	t.Run("fail-corrupted-tail-json", func(t *testing.T) {
		fls := newFS()

		opts := makeOpts(512, true)
		optsRecoverTail := *opts
		optsRecoverTail.RecoverCorruptedTail = true
		fls.MkdirAll("/testlog/corrupt-tail", 0777)
		util.WriteFile(fls,
			"/testlog/corrupt-tail/00000000000000000001",
			[]byte("\n"), 0666)
		if l, err := OpenWAL("/testlog/corrupt-tail", fls, opts); err != ErrCorruptLog {
			l.Close()
			t.Fatalf("expected %v, got %v", ErrCorruptLog, err)
		}
		if l, err := OpenWAL("/testlog/corrupt-tail", fls, &optsRecoverTail); err != nil {
			l.Close()
			t.Fatalf("expected %v, got %v", nil, err)
		}
		util.WriteFile(fls,
			"/testlog/corrupt-tail/00000000000000000001",
			[]byte(`{}`+"\n"), 0666)
		if l, err := OpenWAL("/testlog/corrupt-tail", fls, opts); err != ErrCorruptLog {
			l.Close()
			t.Fatalf("expected %v, got %v", ErrCorruptLog, err)
		}
		if l, err := OpenWAL("/testlog/corrupt-tail", fls, &optsRecoverTail); err != nil {
			l.Close()
			t.Fatalf("expected %v, got %v", nil, err)
		}
		util.WriteFile(fls,
			"/testlog/corrupt-tail/00000000000000000001",
			[]byte(`{"index":"1"}`+"\n"), 0666)
		if l, err := OpenWAL("/testlog/corrupt-tail", fls, opts); err != ErrCorruptLog {
			l.Close()
			t.Fatalf("expected %v, got %v", ErrCorruptLog, err)
		}
		if l, err := OpenWAL("/testlog/corrupt-tail", fls, &optsRecoverTail); err != nil {
			l.Close()
			t.Fatalf("expected %v, got %v", nil, err)
		}
		util.WriteFile(fls,
			"/testlog/corrupt-tail/00000000000000000001",
			[]byte(`{"index":"1","data":"?"}`), 0666)
		if l, err := OpenWAL("/testlog/corrupt-tail", fls, opts); err != ErrCorruptLog {
			l.Close()
			t.Fatalf("expected %v, got %v", ErrCorruptLog, err)
		}
		if l, err := OpenWAL("/testlog/corrupt-tail", fls, &optsRecoverTail); err != nil {
			l.Close()
			t.Fatalf("expected %v, got %v", nil, err)
		}
	})

	t.Run("start-marker-file", func(t *testing.T) {
		fls := newFS()

		lpath := "/testlog/start-marker"
		opts := makeOpts(512, true)
		l := must(OpenWAL(lpath, fls, opts)).(*Log)
		defer l.Close()
		for i := uint64(1); i <= 100; i++ {
			must(nil, l.Write(i, []byte(dataStr(i))))
		}
		path := l.segments[l.findSegment(35)].path
		firstIndex := l.segments[l.findSegment(35)].index
		must(nil, l.Close())
		data := must(util.ReadFile(fls, path)).([]byte)
		must(nil, util.WriteFile(fls, path+".START", data, 0666))
		l = must(OpenWAL(lpath, fls, opts)).(*Log)
		defer l.Close()
		testFirstLast(t, l, firstIndex, 100, nil)

	})
	// t.Run("end-marker-file", func(t *testing.T) {
	// 	lpath := "/testlog/end-marker"
	// 	opts := makeOpts(512, true, JSON)
	// 	l := must(Open(lpath, opts)).(*Log)
	// 	defer l.Close()
	// 	for i := uint64(1); i <= 100; i++ {
	// 		must(nil, l.Write(i, []byte(dataStr(i))))
	// 	}
	// 	path := l.segments[l.findSegment(35)].path
	// 	data := must(ioutil.ReadFile(path)).([]byte)
	// 	var lastIndex uint64
	// 	for {

	// 		n, _, err := readEntry(rd, JSON, true)
	// 		if err == io.EOF {
	// 			break
	// 		}
	// 		if err != nil {
	// 			t.Fatal(err)
	// 		}
	// 		lastIndex = n
	// 	}

	// 	must(nil, l.Close())
	// 	data = must(ioutil.ReadFile(path)).([]byte)
	// 	must(nil, ioutil.WriteFile(path+".END", data, 0666))
	// 	l = must(Open(lpath, opts)).(*Log)
	// 	defer l.Close()
	// 	testFirstLast(t, l, 1, lastIndex)
	// })

}

// https://github.com/tidwall/wal/issues/1
func TestIssue1(t *testing.T) {
	fls := newFS()

	in := []byte{0, 0, 0, 0, 0, 0, 0, 1, 37, 108, 131, 178, 151, 17, 77, 32,
		27, 48, 23, 159, 63, 14, 240, 202, 206, 151, 131, 98, 45, 165, 151, 67,
		38, 180, 54, 23, 138, 238, 246, 16, 0, 0, 0, 0}
	opts := *defaultLogOptions
	l, err := OpenWAL("/testlog", fls, &opts)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err := l.Write(1, in); err != nil {
		t.Fatal(err)
	}
	out, err := l.Read(1)
	if err != nil {
		t.Fatal(err)
	}
	if string(in) != string(out) {
		t.Fatal("data mismatch")
	}
}

func TestSimpleTruncateFront(t *testing.T) {
	fls := newFS()

	opts := &LogOptions{
		NoSync:      true,
		SegmentSize: 100,
	}

	l, err := OpenWAL("/testlog", fls, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		l.Close()
	}()

	makeData := func(index uint64) []byte {
		return []byte(fmt.Sprintf("data-%d", index))
	}

	valid := func(t *testing.T, first, last uint64) {
		t.Helper()
		index, err := l.FirstIndex()
		if err != nil {
			t.Fatal(err)
		}
		if index != first {
			t.Fatalf("expected %v, got %v", first, index)
		}
		index, err = l.LastIndex()
		if err != nil {
			t.Fatal(err)
		}
		if index != last {
			t.Fatalf("expected %v, got %v", last, index)
		}
		for i := first; i <= last; i++ {
			data, err := l.Read(i)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != string(makeData(i)) {
				t.Fatalf("expcted '%s', got '%s'", makeData(i), data)
			}
		}
	}
	validReopen := func(t *testing.T, first, last uint64) {
		t.Helper()
		valid(t, first, last)
		if err := l.Close(); err != nil {
			t.Fatal(err)
		}
		l, err = OpenWAL("/testlog", fls, opts)
		if err != nil {
			t.Fatal(err)
		}
		valid(t, first, last)
	}
	for i := 1; i <= 100; i++ {
		err := l.Write(uint64(i), makeData(uint64(i)))
		if err != nil {
			t.Fatal(err)
		}
	}
	validReopen(t, 1, 100)

	if err := l.TruncateFront(1); err != nil {
		t.Fatal(err)
	}
	validReopen(t, 1, 100)

	if err := l.TruncateFront(2); err != nil {
		t.Fatal(err)
	}
	validReopen(t, 2, 100)

	if err := l.TruncateFront(4); err != nil {
		t.Fatal(err)
	}
	validReopen(t, 4, 100)

	if err := l.TruncateFront(5); err != nil {
		t.Fatal(err)
	}
	validReopen(t, 5, 100)

	if err := l.TruncateFront(99); err != nil {
		t.Fatal(err)
	}
	validReopen(t, 99, 100)

	if err := l.TruncateFront(100); err != nil {
		t.Fatal(err)
	}
	validReopen(t, 100, 100)

}

func TestSimpleTruncateBack(t *testing.T) {
	fls := newFS()

	opts := &LogOptions{
		NoSync:      true,
		SegmentSize: 100,
	}

	l, err := OpenWAL("/testlog", fls, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		l.Close()
	}()

	makeData := func(index uint64) []byte {
		return []byte(fmt.Sprintf("data-%d", index))
	}

	valid := func(t *testing.T, first, last uint64) {
		t.Helper()
		index, err := l.FirstIndex()
		if err != nil {
			t.Fatal(err)
		}
		if index != first {
			t.Fatalf("expected %v, got %v", first, index)
		}
		index, err = l.LastIndex()
		if err != nil {
			t.Fatal(err)
		}
		if index != last {
			t.Fatalf("expected %v, got %v", last, index)
		}
		for i := first; i <= last; i++ {
			data, err := l.Read(i)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != string(makeData(i)) {
				t.Fatalf("expcted '%s', got '%s'", makeData(i), data)
			}
		}
	}
	validReopen := func(t *testing.T, first, last uint64) {
		t.Helper()
		valid(t, first, last)
		if err := l.Close(); err != nil {
			t.Fatal(err)
		}
		l, err = OpenWAL("/testlog", fls, opts)
		if err != nil {
			t.Fatal(err)
		}
		valid(t, first, last)
	}
	for i := 1; i <= 100; i++ {
		err := l.Write(uint64(i), makeData(uint64(i)))
		if err != nil {
			t.Fatal(err)
		}
	}
	validReopen(t, 1, 100)

	/////////////////////////////////////////////////////////////
	if err := l.TruncateBack(100); err != nil {
		t.Fatal(err)
	}
	validReopen(t, 1, 100)
	if err := l.Write(101, makeData(101)); err != nil {
		t.Fatal(err)
	}
	validReopen(t, 1, 101)

	/////////////////////////////////////////////////////////////
	if err := l.TruncateBack(99); err != nil {
		t.Fatal(err)
	}
	validReopen(t, 1, 99)
	if err := l.Write(100, makeData(100)); err != nil {
		t.Fatal(err)
	}
	validReopen(t, 1, 100)

	if err := l.TruncateBack(94); err != nil {
		t.Fatal(err)
	}
	validReopen(t, 1, 94)

	if err := l.TruncateBack(93); err != nil {
		t.Fatal(err)
	}
	validReopen(t, 1, 93)

	if err := l.TruncateBack(92); err != nil {
		t.Fatal(err)
	}
	validReopen(t, 1, 92)

}

func TestConcurrency(t *testing.T) {
	fls := newFS()

	l, err := OpenWAL("/testlog", fls, &LogOptions{
		NoSync: true,
		NoCopy: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// Write 1000 entries
	for i := 1; i <= 1000; i++ {
		err := l.Write(uint64(i), []byte(dataStr(uint64(i))))
		if err != nil {
			t.Fatal(err)
		}
	}

	// Perform 100,000 reads (over 100 threads)
	finished := int32(0)
	maxIndex := int32(1000)
	numReads := int32(0)
	for i := 0; i < 100; i++ {
		go func() {
			defer atomic.AddInt32(&finished, 1)

			for i := 0; i < 1_000; i++ {
				index := rand.Int31n(atomic.LoadInt32(&maxIndex)) + 1
				if _, err := l.Read(uint64(index)); err != nil {
					panic(err)
				}
				atomic.AddInt32(&numReads, 1)
			}
		}()
	}

	// continue writing
	for index := maxIndex + 1; atomic.LoadInt32(&finished) < 100; index++ {
		err := l.Write(uint64(index), []byte(dataStr(uint64(index))))
		if err != nil {
			t.Fatal(err)
		}
		atomic.StoreInt32(&maxIndex, index)
	}

	// confirm total reads
	if exp := int32(100_000); numReads != exp {
		t.Fatalf("expected %d reads, but god %d", exp, numReads)
	}
}

func dataStr(index uint64) string {
	if index%2 == 0 {
		return fmt.Sprintf("data-\"%d\"", index)
	}
	return fmt.Sprintf("data-'%d'", index)
}

func testLog(t *testing.T, opts *LogOptions, N int) {
	logPath := "/testlog"
	fls := newFS()

	l, err := OpenWAL(logPath, fls, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// FirstIndex - should be zero
	n, err := l.FirstIndex()
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("expected %d, got %d", 0, n)
	}

	// LastIndex - should be zero
	n, err = l.LastIndex()
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("expected %d, got %d", 0, n)
	}

	for i := 1; i <= N; i++ {
		// Write - try to append previous index, should fail
		err = l.Write(uint64(i-1), nil)
		if err != ErrLogIndexOutOfOrder {
			t.Fatalf("expected %v, got %v", ErrLogIndexOutOfOrder, err)
		}
		// Write - append next item
		err = l.Write(uint64(i), []byte(dataStr(uint64(i))))
		if err != nil {
			t.Fatalf("expected %v, got %v", nil, err)
		}
		// Write - get next item
		data, err := l.Read(uint64(i))
		if err != nil {
			t.Fatalf("expected %v, got %v", nil, err)
		}
		if string(data) != dataStr(uint64(i)) {
			t.Fatalf("expected %s, got %s", dataStr(uint64(i)), data)
		}
	}

	// Read -- should fail, not found
	_, err = l.Read(0)
	if err != ErrLogEntryNotFound {
		t.Fatalf("expected %v, got %v", ErrLogEntryNotFound, err)
	}
	// Read -- read back all entries
	for i := 1; i <= N; i++ {
		data, err := l.Read(uint64(i))
		if err != nil {
			t.Fatalf("error while getting %d", i)
		}
		if string(data) != dataStr(uint64(i)) {
			t.Fatalf("expected %s, got %s", dataStr(uint64(i)), data)
		}
	}
	// Read -- read back first half entries
	for i := 1; i <= N/2; i++ {
		data, err := l.Read(uint64(i))
		if err != nil {
			t.Fatalf("error while getting %d", i)
		}
		if string(data) != dataStr(uint64(i)) {
			t.Fatalf("expected %s, got %s", dataStr(uint64(i)), data)
		}
	}
	// Read -- read second third entries
	for i := N / 3; i <= N/3+N/3; i++ {
		data, err := l.Read(uint64(i))
		if err != nil {
			t.Fatalf("error while getting %d", i)
		}
		if string(data) != dataStr(uint64(i)) {
			t.Fatalf("expected %s, got %s", dataStr(uint64(i)), data)
		}
	}

	// Read -- random access
	for _, v := range rand.Perm(N) {
		index := uint64(v + 1)
		data, err := l.Read(index)
		if err != nil {
			t.Fatal(err)
		}
		if dataStr(index) != string(data) {
			t.Fatalf("expected %v, got %v", dataStr(index), string(data))
		}
	}

	// FirstIndex/LastIndex -- check valid first and last indexes
	n, err = l.FirstIndex()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected %d, got %d", 1, n)
	}
	n, err = l.LastIndex()
	if err != nil {
		t.Fatal(err)
	}
	if n != uint64(N) {
		t.Fatalf("expected %d, got %d", N, n)
	}

	// Close -- close the log
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	// Write - try while closed
	err = l.Write(1, nil)
	if err != ErrLogClosed {
		t.Fatalf("expected %v, got %v", ErrLogClosed, err)
	}
	// WriteBatch - try while closed
	err = l.WriteBatch(nil)
	if err != ErrLogClosed {
		t.Fatalf("expected %v, got %v", ErrLogClosed, err)
	}
	// FirstIndex - try while closed
	_, err = l.FirstIndex()
	if err != ErrLogClosed {
		t.Fatalf("expected %v, got %v", ErrLogClosed, err)
	}
	// LastIndex - try while closed
	_, err = l.LastIndex()
	if err != ErrLogClosed {
		t.Fatalf("expected %v, got %v", ErrLogClosed, err)
	}
	// Get - try while closed
	_, err = l.Read(0)
	if err != ErrLogClosed {
		t.Fatalf("expected %v, got %v", ErrLogClosed, err)
	}
	// TruncateFront - try while closed
	err = l.TruncateFront(0)
	if err != ErrLogClosed {
		t.Fatalf("expected %v, got %v", ErrLogClosed, err)
	}
	// TruncateBack - try while closed
	err = l.TruncateBack(0)
	if err != ErrLogClosed {
		t.Fatalf("expected %v, got %v", ErrLogClosed, err)
	}

	// Open -- reopen log
	l, err = OpenWAL(logPath, fls, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// Read -- read back all entries
	for i := 1; i <= N; i++ {
		data, err := l.Read(uint64(i))
		if err != nil {
			t.Fatalf("error while getting %d, err=%s", i, err)
		}
		if string(data) != dataStr(uint64(i)) {
			t.Fatalf("expected %s, got %s", dataStr(uint64(i)), data)
		}
	}
	// FirstIndex/LastIndex -- check valid first and last indexes
	n, err = l.FirstIndex()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected %d, got %d", 1, n)
	}
	n, err = l.LastIndex()
	if err != nil {
		t.Fatal(err)
	}
	if n != uint64(N) {
		t.Fatalf("expected %d, got %d", N, n)
	}
	// Write -- add 50 more items
	for i := N + 1; i <= N+50; i++ {
		index := uint64(i)
		if err := l.Write(index, []byte(dataStr(index))); err != nil {
			t.Fatal(err)
		}
		data, err := l.Read(index)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != dataStr(index) {
			t.Fatalf("expected %v, got %v", dataStr(index), string(data))
		}
	}
	N += 50
	// FirstIndex/LastIndex -- check valid first and last indexes
	n, err = l.FirstIndex()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected %d, got %d", 1, n)
	}
	n, err = l.LastIndex()
	if err != nil {
		t.Fatal(err)
	}
	if n != uint64(N) {
		t.Fatalf("expected %d, got %d", N, n)
	}
	// Batch -- test batch writes
	b := new(Batch)
	b.Write(1, nil)
	b.Write(2, nil)
	b.Write(3, nil)

	i := utils.Must(l.LastIndex()) + 1
	b.FinalizeEntry = func(entry []byte, absoluteIndex uint64) {
		assert.Equal(t, absoluteIndex, i)
		i++
	}
	finalizeBatchCalled := false
	afterWriteAttemptCalled := false
	b.FinalizeBatch = func(lastEntryIndex uint64) {
		finalizeBatchCalled = true
	}
	b.AfterWriteAttempt = func(err error) {
		afterWriteAttemptCalled = true
		assert.ErrorIs(t, err, ErrLogIndexOutOfOrder)
	}

	// WriteBatch -- should fail out of order
	err = l.WriteBatch(b)
	if err != ErrLogIndexOutOfOrder {
		t.Fatalf("expected %v, got %v", ErrLogIndexOutOfOrder, nil)
	}

	assert.True(t, finalizeBatchCalled)
	assert.True(t, afterWriteAttemptCalled)

	// Clear -- clear the batch
	b.Clear()
	b.FinalizeEntry = func(entry []byte, absoluteIndex uint64) {
		assert.Fail(t, "no entry")
	}
	finalizeBatchCalled = false
	afterWriteAttemptCalled = false
	b.FinalizeBatch = func(lastEntryIndex uint64) {
		finalizeBatchCalled = true
	}
	b.AfterWriteAttempt = func(err error) {
		afterWriteAttemptCalled = true
		assert.NoError(t, err)
	}

	// WriteBatch -- should succeed
	err = l.WriteBatch(b)
	if err != nil {
		t.Fatal(err)
	}
	b.FinalizeBatch = nil
	b.FinalizeEntry = nil
	b.AfterWriteAttempt = nil

	assert.True(t, finalizeBatchCalled)
	assert.True(t, afterWriteAttemptCalled)

	// Write 100 entries in batches of 10
	for i := 0; i < 10; i++ {
		for i := N + 1; i <= N+10; i++ {
			index := uint64(i)
			b.Write(index, []byte(dataStr(index)))
		}

		finalizeBatchCalled := false
		afterWriteAttemptCalled := false
		{
			i := utils.Must(l.LastIndex()) + 1
			b.FinalizeEntry = func(entry []byte, absoluteIndex uint64) {
				assert.Equal(t, absoluteIndex, i)
				i++
			}
			b.FinalizeBatch = func(lastEntryIndex uint64) {
				finalizeBatchCalled = true
			}
			b.AfterWriteAttempt = func(err error) {
				afterWriteAttemptCalled = true
				assert.NoError(t, err)
			}
		}

		err = l.WriteBatch(b)
		if err != nil {
			t.Fatal(err)
		}
		N += 10

		assert.True(t, finalizeBatchCalled)
		assert.True(t, afterWriteAttemptCalled)
		b.FinalizeBatch = nil
		b.FinalizeEntry = nil
		b.AfterWriteAttempt = nil
	}
	// Read -- read back all entries
	for i := 1; i <= N; i++ {
		data, err := l.Read(uint64(i))
		if err != nil {
			t.Fatalf("error while getting %d", i)
		}
		if string(data) != dataStr(uint64(i)) {
			t.Fatalf("expected %s, got %s", dataStr(uint64(i)), data)
		}
	}

	// Write -- one entry, so the buffer might be activated
	err = l.Write(uint64(N+1), []byte(dataStr(uint64(N+1))))
	if err != nil {
		t.Fatal(err)
	}
	N++
	// Read -- one random read, so there is an opened reader
	data, err := l.Read(uint64(N / 2))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != dataStr(uint64(N/2)) {
		t.Fatalf("expected %v, got %v", dataStr(uint64(N/2)), string(data))
	}

	// TruncateFront -- should fail, out of range
	for _, i := range []int{0, N + 1} {
		index := uint64(i)
		if err = l.TruncateFront(index); err != ErrLogIndexOutOfRange {
			t.Fatalf("expected %v, got %v", ErrLogIndexOutOfRange, err)
		}
		testFirstLast(t, l, uint64(1), uint64(N), nil)
	}

	// TruncateBack -- should fail, out of range
	err = l.TruncateFront(0)
	if err != ErrLogIndexOutOfRange {
		t.Fatalf("expected %v, got %v", ErrLogIndexOutOfRange, err)
	}
	testFirstLast(t, l, uint64(1), uint64(N), nil)

	// TruncateFront -- Remove no entries
	if err = l.TruncateFront(1); err != nil {
		t.Fatal(err)
	}
	testFirstLast(t, l, uint64(1), uint64(N), nil)

	// TruncateFront -- Remove first 80 entries
	if err = l.TruncateFront(81); err != nil {
		t.Fatal(err)
	}
	testFirstLast(t, l, uint64(81), uint64(N), nil)

	// Write -- one entry, so the buffer might be activated
	err = l.Write(uint64(N+1), []byte(dataStr(uint64(N+1))))
	if err != nil {
		t.Fatal(err)
	}
	N++
	testFirstLast(t, l, uint64(81), uint64(N), nil)

	// Read -- one random read, so there is an opened reader
	data, err = l.Read(uint64(N / 2))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != dataStr(uint64(N/2)) {
		t.Fatalf("expected %v, got %v", dataStr(uint64(N/2)), string(data))
	}

	// TruncateBack -- should fail, out of range
	for _, i := range []int{0, 80} {
		index := uint64(i)
		if err = l.TruncateBack(index); err != ErrLogIndexOutOfRange {
			t.Fatalf("expected %v, got %v", ErrLogIndexOutOfRange, err)
		}
		testFirstLast(t, l, uint64(81), uint64(N), nil)
	}

	// TruncateBack -- Remove no entries
	if err = l.TruncateBack(uint64(N)); err != nil {
		t.Fatal(err)
	}
	testFirstLast(t, l, uint64(81), uint64(N), nil)
	// TruncateBack -- Remove last 80 entries
	if err = l.TruncateBack(uint64(N - 80)); err != nil {
		t.Fatal(err)
	}
	N -= 80
	testFirstLast(t, l, uint64(81), uint64(N), nil)

	// Read -- read back all entries
	for i := 81; i <= N; i++ {
		data, err := l.Read(uint64(i))
		if err != nil {
			t.Fatalf("error while getting %d", i)
		}
		if string(data) != dataStr(uint64(i)) {
			t.Fatalf("expected %s, got %s", dataStr(uint64(i)), data)
		}
	}

	// Close -- close log after truncating
	if err = l.Close(); err != nil {
		t.Fatal(err)
	}

	// Open -- open log after truncating
	l, err = OpenWAL(logPath, fls, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	testFirstLast(t, l, uint64(81), uint64(N), nil)

	// Read -- read back all entries
	for i := 81; i <= N; i++ {
		data, err := l.Read(uint64(i))
		if err != nil {
			t.Fatalf("error while getting %d", i)
		}
		if string(data) != dataStr(uint64(i)) {
			t.Fatalf("expected %s, got %s", dataStr(uint64(i)), data)
		}
	}

	// TruncateFront -- truncate all entries but one
	if err = l.TruncateFront(uint64(N)); err != nil {
		t.Fatal(err)
	}
	testFirstLast(t, l, uint64(N), uint64(N), nil)

	// Write -- write on entry
	err = l.Write(uint64(N+1), []byte(dataStr(uint64(N+1))))
	if err != nil {
		t.Fatal(err)
	}
	N++
	testFirstLast(t, l, uint64(N-1), uint64(N), nil)

	// TruncateBack -- truncate all entries but one
	if err = l.TruncateBack(uint64(N - 1)); err != nil {
		t.Fatal(err)
	}
	N--
	testFirstLast(t, l, uint64(N), uint64(N), nil)

	if err = l.Write(uint64(N+1), []byte(dataStr(uint64(N+1)))); err != nil {
		t.Fatal(err)
	}
	N++

	l.Sync()
	testFirstLast(t, l, uint64(N-1), uint64(N), nil)
}

func testFirstLast(t *testing.T, l *Log, expectFirst, expectLast uint64, data func(index uint64) []byte) {
	t.Helper()
	fi, err := l.FirstIndex()
	if err != nil {
		t.Fatal(err)
	}
	li, err := l.LastIndex()
	if err != nil {
		t.Fatal(err)
	}
	if fi != expectFirst || li != expectLast {
		t.Fatalf("expected %v/%v, got %v/%v", expectFirst, expectLast, fi, li)
	}
	for i := fi; i <= li; i++ {
		dt1, err := l.Read(i)
		if err != nil {
			t.Fatal(err)
		}
		if data != nil {
			dt2 := data(i)
			if string(dt1) != string(dt2) {
				t.Fatalf("mismatch '%s' != '%s'", dt2, dt1)
			}
		}
	}

}

func makeOpts(segSize int, noSync bool) *LogOptions {
	opts := *defaultLogOptions
	opts.SegmentSize = segSize
	opts.NoSync = noSync
	return &opts
}

func must(v interface{}, err error) interface{} {
	if err != nil {
		panic(err)
	}
	return v
}

type fs struct {
	billy.Filesystem
}

func newFS() *fs {
	return &fs{
		Filesystem: memfs.New(),
	}
}

type file struct {
	billy.File
}

func (f *file) Sync() error {
	return nil
}

func (fs fs) Absolute(path string) (string, error) {
	if path[0] != '/' {
		return "", errors.New("not implemented")
	}
	return path, nil
}

func (fs fs) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	f, err := fs.Filesystem.OpenFile(filename, flag, perm)
	if err != nil {
		return nil, err
	}

	return &file{f}, nil
}

func (fs fs) Create(filename string) (billy.File, error) {
	f, err := fs.Filesystem.Create(filename)
	if err != nil {
		return nil, err
	}

	return &file{f}, nil
}
