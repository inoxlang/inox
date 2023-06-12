package filekv

import (
	"errors"
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/inoxlang/inox/internal/afs"
	core "github.com/inoxlang/inox/internal/core"

	"github.com/inoxlang/inox/internal/utils"
)

const (
	KV_STORE_SRC_NAME = "/kv"
)

var (
	ErrInvalidPathKey = errors.New("invalid path used as local database key")
)

// thin wrapper around a buntdb database.
type SingleFileKV struct {
	db   *buntDB
	path core.Path
	host core.Host

	transactionMapLock sync.Mutex
	transactions       map[*core.Transaction]*Tx
}

type KvStoreConfig struct {
	Path       core.Path
	InMemory   bool
	Filesystem afs.Filesystem
}

func OpenSingleFileKV(config KvStoreConfig) (_ *SingleFileKV, finalErr error) {
	path := string(config.Path)
	if config.InMemory {
		path = ":memory:"
	}

	fls := config.Filesystem
	buntDBconfig := defaultBuntdbConfig

	db, err := openBuntDBNoPermCheck(path, fls, buntDBconfig)
	if err != nil {
		finalErr = err
		return
	}

	return &SingleFileKV{
		db:   db,
		path: config.Path,

		transactions: map[*core.Transaction]*Tx{},
	}, nil
}

func (kv *SingleFileKV) Close(ctx *core.Context) {
	logger := ctx.Logger().With().Str(core.SOURCE_LOG_FIELD_NAME, KV_STORE_SRC_NAME).Logger()
	//before closing the buntdb database all the transactions must be closed or a deadlock will occur.

	logger.Print("close KV store")

	kv.transactionMapLock.Lock()
	transactions := utils.CopyMap(kv.transactions)
	kv.transactionMapLock.Unlock()

	logger.Print("number of transactions to close: ", len(transactions))

	for tx, dbTx := range transactions {
		func() {
			defer recover()
			//will be ignored if the transaction already finished.
			tx.Rollback(ctx)
		}()

		func() {
			defer recover()
			if dbTx.db != nil { //still not finished.
				dbTx.unlock()
			}
		}()
	}

	logger.Print("close bluntDB")
	kv.db.Close()
}

func (kv *SingleFileKV) isClosed() bool {
	return kv.db.isClosed()
}

func (kv *SingleFileKV) Get(ctx *core.Context, key core.Path, db any) (core.Value, core.Bool, error) {
	if kv.isClosed() {
		return nil, false, errDatabaseClosed
	}

	if !key.IsAbsolute() {
		return nil, false, ErrInvalidPathKey
	}

	var (
		valueFound = core.True
		val        core.Value
		tx         = ctx.GetTx()
	)

	if tx == nil {
		err := kv.db.View(func(txn *Tx) error {
			item, err := txn.Get(string(key))
			if err == errNotFound {
				valueFound = core.False
				return nil
			} else if err != nil {
				return err
			}

			val, err = core.ParseRepr(ctx, utils.StringAsBytes(item))
			return err
		})

		if err != nil {
			return nil, false, err
		}

	} else {
		dbtx := kv.getCreateDatabaseTxn(db, tx)

		var err error
		val, valueFound, err = dbtx.Get(ctx, key)
		if err != nil {
			return nil, false, err
		}
	}

	if valueFound {
		//TODO ....
	}

	if val == nil {
		val = core.Nil
	}

	return val, valueFound, nil
}

// ForEach calls a function for each item in the database, the provided getVal function should not be stored as it only
// returns the value at the time of the iteration.
func (kv *SingleFileKV) ForEach(ctx *core.Context, fn func(key core.Path, getVal func() core.Value) error, db any) error {
	if kv.isClosed() {
		return errDatabaseClosed
	}

	if fn == nil {
		return errors.New("iteration function is nil")
	}

	tx := ctx.GetTx()

	handleItem := func(key, value string) (cont bool) {
		if key == "" || key[0] != '/' {
			return true
		}

		defer func() {
			if recover() != nil {
				cont = false
			}
		}()

		path := core.PathFrom(key)
		getVal := func() core.Value {
			return utils.Must(core.ParseRepr(ctx, utils.StringAsBytes(value)))
		}

		err := fn(path, getVal)
		return err == nil
	}

	iterWithTx := func(dbTx *Tx) error {
		return dbTx.Ascend("", func(key, value string) bool {
			return handleItem(key, value)
		})
	}

	if tx == nil {
		return kv.db.View(iterWithTx)
	} else {
		dbTx := kv.getCreateDatabaseTxn(db, tx)
		return iterWithTx(dbTx.tx)
	}
}

func (kv *SingleFileKV) UpdateNoCtx(fn func(dbTx *DatabaseTx) error) error {
	if kv.isClosed() {
		return errDatabaseClosed
	}

	if fn == nil {
		return errors.New("iteration function is nil")
	}

	return kv.db.Update(func(dbTx *Tx) (finalErr error) {
		defer func() {
			e := recover()
			switch v := e.(type) {
			case error:
				finalErr = fmt.Errorf("%w %s", v, string(debug.Stack()))
			default:
				finalErr = fmt.Errorf("panic: %#v %s", e, string(debug.Stack()))
			case nil:
			}
		}()
		return fn(NewDatabaseTxIL(dbTx))
	})
}

func (kv *SingleFileKV) Has(ctx *core.Context, key core.Path, db any) core.Bool {
	if kv.isClosed() {
		panic(errDatabaseClosed)
	}

	if !key.IsAbsolute() {
		panic(ErrInvalidPathKey)
	}

	var (
		valueFound = core.True
		tx         = ctx.GetTx()
	)

	if tx == nil {
		err := kv.db.View(func(txn *Tx) error {
			_, err := txn.Get(string(key))
			if err == errNotFound {
				valueFound = core.False
				return nil
			}
			return err
		})

		if err != nil {
			panic(err)
		}

	} else {
		dbtx := kv.getCreateDatabaseTxn(db, tx)

		var err error
		_, valueFound, err = dbtx.Get(ctx, key)
		if err != nil {
			panic(err)
		}
	}

	return valueFound
}

func (kv *SingleFileKV) Set(ctx *core.Context, key core.Path, value core.Value, db any) {

	if kv.db.isClosed() {
		panic(errDatabaseClosed)
	}

	if !key.IsAbsolute() {
		panic(ErrInvalidPathKey)
	}

	tx := ctx.GetTx()

	if tx == nil {
		err := kv.db.Update(func(txn *Tx) error {
			repr := core.GetRepresentation(value, ctx)
			_, _, err := txn.Set(string(key), string(repr), nil)
			return err
		})

		if err != nil {
			panic(err)
		}

	} else {
		dbtx := kv.getCreateDatabaseTxn(db, tx)
		err := dbtx.Set(ctx, key, value)

		if err != nil {
			panic(err)
		}
	}
}

func (kv *SingleFileKV) Delete(ctx *core.Context, key core.Path, db any) {

	if kv.db.isClosed() {
		panic(errDatabaseClosed)
	}

	if !key.IsAbsolute() {
		panic(ErrInvalidPathKey)
	}

	tx := ctx.GetTx()

	if tx == nil {
		err := kv.db.Update(func(dbTx *Tx) error {
			_, err := dbTx.Delete(string(key))
			return err
		})

		if err != nil {
			panic(err)
		}

	} else {
		dbTx := kv.getCreateDatabaseTxn(db, tx)

		err := dbTx.Delete(ctx, key)
		if err != nil {
			panic(err)
		}
	}
}

func (kv *SingleFileKV) getCreateDatabaseTxn(db any, tx *core.Transaction) *DatabaseTx {
	//if there is already a database transaction in the core.Transaction we return it.
	v, err := tx.GetValue(db)
	if err != nil {
		panic(err)
	}
	dbTx, ok := v.(*Tx)

	if ok {
		return NewDatabaseTxIL(dbTx)
	}

	//begin a new database transaction & add it to the core.Transaction.
	dbTx, err = kv.db.Begin(true)
	if err != nil {
		panic(err)
	}
	if err = tx.SetValue(db, dbTx); err != nil {
		panic(err)
	}

	//add core.Transaction to KV.
	kv.transactionMapLock.Lock()
	kv.transactions[tx] = dbTx
	kv.transactionMapLock.Unlock()

	if err = tx.OnEnd(db, makeTxEndcallbackFn(dbTx, tx, kv)); err != nil {
		panic(err)
	}

	return NewDatabaseTxIL(dbTx)
}

func makeTxEndcallbackFn(dbtx *Tx, tx *core.Transaction, kv *SingleFileKV) func(t *core.Transaction, success bool) {
	return func(t *core.Transaction, success bool) {
		kv.transactionMapLock.Lock()
		delete(kv.transactions, tx)
		kv.transactionMapLock.Unlock()

		if success {
			if err := dbtx.Commit(); err != nil {
				panic(err)
			}
		} else if err := dbtx.Rollback(); err != nil {
			panic(err)
		}
	}
}

type DatabaseTx struct {
	tx *Tx
}

func NewDatabaseTxIL(tx *Tx) *DatabaseTx {
	return &DatabaseTx{
		tx: tx,
	}
}

func (tx *DatabaseTx) Get(ctx *core.Context, key core.Path) (result core.Value, valueFound core.Bool, finalErr error) {
	item, err := tx.tx.Get(string(key))
	if err == errNotFound {
		valueFound = false
	} else if err != nil {
		panic(err)
	} else {
		valueFound = true
		result, err = core.ParseRepr(ctx, utils.StringAsBytes(item))

		if err != nil {
			return nil, false, err
		}
		return
	}
	return
}

func (tx *DatabaseTx) Set(ctx *core.Context, key core.Path, value core.Value) error {
	repr := core.GetRepresentation(value, ctx)
	_, _, err := tx.tx.Set(string(key), string(repr), nil)

	return err
}

func (tx *DatabaseTx) Delete(ctx *core.Context, key core.Path) error {
	_, err := tx.tx.Delete(string(key))
	return err
}
