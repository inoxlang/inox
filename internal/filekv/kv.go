package filekv

import (
	"errors"
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/go-git/go-billy/v5"
	core "github.com/inoxlang/inox/internal/core"

	"github.com/inoxlang/inox/internal/utils"
)

const (
	KV_STORE_SRC_NAME = "/kv"
)

var (
	ErrInvalidPathKey    = errors.New("invalid path used as local database key")
	ErrKeyAlreadyPresent = errors.New("key already present")

	_ core.SerializedValueStorage = (*SerializedValueStorageAdapter)(nil)
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
	Filesystem billy.Basic
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
	serialized, found, err := kv.GetSerialized(ctx, key, db)

	if err != nil {
		return nil, found, err
	}

	if !found {
		return core.Nil, false, nil
	}

	val, err := core.ParseRepr(ctx, utils.StringAsBytes(serialized))
	return val, found, err
}

func (kv *SingleFileKV) GetSerialized(ctx *core.Context, key core.Path, db any) (string, core.Bool, error) {
	if kv.isClosed() {
		return "", false, errDatabaseClosed
	}

	if !key.IsAbsolute() {
		return "", false, ErrInvalidPathKey
	}

	var (
		valueFound = core.True
		serialized string
		dbtx       = kv.getCreateDatabaseTxn(db, ctx.GetTx())
	)

	if dbtx == nil {
		err := kv.db.View(func(txn *Tx) error {
			item, err := txn.Get(string(key))
			if err == errNotFound {
				valueFound = core.False
				return nil
			} else if err != nil {
				return err
			}
			serialized = item
			return nil
		})

		if err != nil {
			return "", false, err
		}

	} else {
		var err error
		serialized, valueFound, err = dbtx.GetSerialized(ctx, key)
		if err != nil {
			return "", false, err
		}
	}

	if valueFound {
		//TODO ....
	}

	return serialized, valueFound, nil
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

	dbTx := kv.getCreateDatabaseTxn(db, ctx.GetTx())

	if dbTx == nil {
		return kv.db.View(iterWithTx)
	} else {
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
		dbTx       = kv.getCreateDatabaseTxn(db, ctx.GetTx())
	)

	if dbTx == nil {
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
		var err error
		_, valueFound, err = dbTx.Get(ctx, key)
		if err != nil {
			panic(err)
		}
	}

	return valueFound
}

func (kv *SingleFileKV) Insert(ctx *core.Context, key core.Path, value core.Serializable, db any) {
	repr := core.GetRepresentation(value, ctx)

	kv.InsertSerialized(ctx, key, string(repr), db)
}

func (kv *SingleFileKV) InsertSerialized(ctx *core.Context, key core.Path, serialized string, db any) {
	//TODO: check valid representation

	if kv.db.isClosed() {
		panic(errDatabaseClosed)
	}

	if !key.IsAbsolute() {
		panic(ErrInvalidPathKey)
	}

	dbTx := kv.getCreateDatabaseTxn(db, ctx.GetTx())

	if dbTx == nil {
		err := kv.db.Update(func(txn *Tx) error {
			_, replaced, err := txn.Set(string(key), serialized, nil)
			if replaced {
				return fmt.Errorf("%w: %s", ErrKeyAlreadyPresent, key)
			}
			return err
		})

		if err != nil {
			panic(err)
		}

	} else {
		err := dbTx.InsertSerialized(ctx, key, serialized)

		if err != nil {
			panic(err)
		}
	}
}

func (kv *SingleFileKV) Set(ctx *core.Context, key core.Path, value core.Serializable, db any) {
	repr := core.GetRepresentation(value, ctx)
	kv.SetSerialized(ctx, key, string(repr), db)
}

func (kv *SingleFileKV) SetSerialized(ctx *core.Context, key core.Path, serialized string, db any) {
	//TODO: check valid representation

	if kv.db.isClosed() {
		panic(errDatabaseClosed)
	}

	if !key.IsAbsolute() {
		panic(ErrInvalidPathKey)
	}

	dbtx := kv.getCreateDatabaseTxn(db, ctx.GetTx())

	if dbtx == nil {
		err := kv.db.Update(func(txn *Tx) error {
			_, _, err := txn.Set(string(key), serialized, nil)
			return err
		})

		if err != nil {
			panic(err)
		}

	} else {
		err := dbtx.SetSerialized(ctx, key, serialized)

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

	dbTx := kv.getCreateDatabaseTxn(db, ctx.GetTx())

	if dbTx == nil {
		err := kv.db.Update(func(dbTx *Tx) error {
			_, err := dbTx.Delete(string(key))
			return err
		})

		if err != nil {
			panic(err)
		}

	} else {
		err := dbTx.Delete(ctx, key)
		if err != nil {
			panic(err)
		}
	}
}

// getCreateDatabaseTxn gets or creates a DatabaseTx associated with tx, if tx is nil or is already finished
// and no DatabaseTx is associated with it nil is returned.
func (kv *SingleFileKV) getCreateDatabaseTxn(db any, tx *core.Transaction) *DatabaseTx {
	if tx == nil {
		return nil
	}

	kv.transactionMapLock.Lock()
	defer kv.transactionMapLock.Unlock()
	dbTx, ok := kv.transactions[tx]

	if ok {
		return NewDatabaseTxIL(dbTx)
	}

	//begin a new database transaction & add it to the core.Transaction.
	dbTx, err := kv.db.Begin(true)
	if err != nil {
		panic(err)
	}

	if tx.IsFinished() {
		dbTx.Rollback()
		return nil
	}

	//add core.Transaction to KV.
	kv.transactions[tx] = dbTx

	if err = tx.OnEnd(kv, makeTxEndcallbackFn(dbTx, tx, kv)); err != nil && err != core.ErrFinishedTransaction {
		panic(err)
	}

	return NewDatabaseTxIL(dbTx)
}

func makeTxEndcallbackFn(dbtx *Tx, tx *core.Transaction, kv *SingleFileKV) func(t *core.Transaction, success bool) {
	return func(t *core.Transaction, success bool) {
		kv.transactionMapLock.Lock()
		if _, ok := kv.transactions[tx]; !ok {
			return
		}
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
	serialized, found, err := tx.GetSerialized(ctx, key)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, found, nil
	}

	result, err = core.ParseRepr(ctx, utils.StringAsBytes(serialized))

	if err != nil {
		return nil, true, err
	}

	return result, true, nil
}

func (tx *DatabaseTx) GetSerialized(ctx *core.Context, key core.Path) (result string, valueFound core.Bool, finalErr error) {
	item, err := tx.tx.Get(string(key))
	if err == errNotFound {
		valueFound = false
	} else if err != nil {
		panic(err)
	} else {
		valueFound = true
		result = item
		return
	}
	return
}

func (tx *DatabaseTx) Set(ctx *core.Context, key core.Path, value core.Serializable) error {
	repr := core.GetRepresentation(value, ctx)
	return tx.SetSerialized(ctx, key, string(repr))
}

func (tx *DatabaseTx) SetSerialized(ctx *core.Context, key core.Path, serialized string) error {
	_, _, err := tx.tx.Set(string(key), serialized, nil)

	return err
}

func (tx *DatabaseTx) Insert(ctx *core.Context, key core.Path, value core.Serializable) error {
	repr := core.GetRepresentation(value, ctx)
	return tx.InsertSerialized(ctx, key, string(repr))
}

func (tx *DatabaseTx) InsertSerialized(ctx *core.Context, key core.Path, serialized string) error {
	_, replaced, err := tx.tx.Set(string(key), serialized, nil)

	if replaced {
		return fmt.Errorf("%w: %s", ErrKeyAlreadyPresent, key)
	}

	return err
}

func (tx *DatabaseTx) Delete(ctx *core.Context, key core.Path) error {
	_, err := tx.tx.Delete(string(key))
	return err
}

type SerializedValueStorageAdapter struct {
	kv  *SingleFileKV
	url core.URL
}

func NewSerializedValueStorage(kv *SingleFileKV, url core.URL) *SerializedValueStorageAdapter {
	return &SerializedValueStorageAdapter{kv: kv, url: url}
}

func (a *SerializedValueStorageAdapter) BaseURL() core.URL {
	return a.url
}

func (a *SerializedValueStorageAdapter) GetSerialized(ctx *core.Context, key core.Path) (string, bool) {
	serialized, ok := utils.Must2(a.kv.GetSerialized(ctx, key, a))
	return serialized, bool(ok)
}

func (a SerializedValueStorageAdapter) Has(ctx *core.Context, key core.Path) bool {
	return bool(a.kv.Has(ctx, key, a))
}

func (a *SerializedValueStorageAdapter) InsertSerialized(ctx *core.Context, key core.Path, serialized string) {
	a.kv.InsertSerialized(ctx, key, serialized, a)
}

// SetSerialized implements core.SerializedValueStorage.
func (a *SerializedValueStorageAdapter) SetSerialized(ctx *core.Context, key core.Path, serialized string) {
	a.kv.SetSerialized(ctx, key, serialized, a)
}
