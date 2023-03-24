package internal

import (
	"encoding/binary"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	_wal "github.com/thomasjungblut/go-sstables/wal"
)

func TestCustomDB(t *testing.T) {
	t.SkipNow()
	pth := _wal.BasePath("/tmp/waldir")
	opts, err := _wal.NewWriteAheadLogOptions(pth)
	if !assert.NoError(t, err) {
		return
	}

	// wal, err := _wal.NewWriteAheadLog(opts)
	// assert.NoError(t, err)

	// record := make([]byte, 8)
	// binary.BigEndian.PutUint64(record, 42)
	// err = wal.AppendSync(record)
	// assert.NoError(t, err)

	// wal.Close()
	// wal.Rotate()

	replayer, err := _wal.NewReplayer(opts)
	assert.NoError(t, err)

	err = replayer.Replay(func(record []byte) error {
		n := binary.BigEndian.Uint64(record)
		log.Println(n)
		return nil
	})
	assert.NoError(t, err)
}
