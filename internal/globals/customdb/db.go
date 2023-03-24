package internal

import (
	"errors"
	"log"

	"github.com/thomasjungblut/go-sstables/wal"

	core "github.com/inox-project/inox/internal/core"
)

type Database struct {
	dirpath    string
	blockCache map[int64]*DataBlock

	wal        wal.WriteAheadLogI
	walOptions *wal.Options
}

func _open(dirpath core.Path) (*Database, error) {

	if dirpath.IsDirPath() {
		return nil, errors.New("path argument should be a directory path")
	}

	walOpts, err := wal.NewWriteAheadLogOptions(wal.BasePath(string(dirpath)))
	if err != nil {
		return nil, err
	}

	db := &Database{
		dirpath:    string(dirpath),
		walOptions: walOpts,
	}

	//replay WAL

	replayer, err := wal.NewReplayer(walOpts)
	if err != nil {
		return nil, err
	}

	err = replayer.Replay(func(record []byte) error {

		return nil
	})

	if err != nil {
		return nil, err
	}

	//remove WAL files

	cleaner := wal.NewCleaner(walOpts)
	if err := cleaner.Clean(); err != nil {
		log.Println(err)
	}

	//create new WAL

	db.wal, err = wal.NewWriteAheadLog(walOpts)
	if err != nil {
		return nil, err
	}

	return db, nil
}

type DataBlock struct {
}
