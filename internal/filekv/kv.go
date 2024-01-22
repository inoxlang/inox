package filekv

import (
	"errors"
	"fmt"
	"maps"
	"runtime/debug"
	"sync"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"go.etcd.io/bbolt"

	"github.com/inoxlang/inox/internal/utils"
)

const (
	KV_STORE_LOG_SRC  = "kv"
	BBOLT_FILE_FPERMS = 0700
)

var (
	ErrInvalidPathKey    = errors.New("invalid path used as local database key")
	ErrKeyAlreadyPresent = errors.New("key already present")
	ErrClosedKvStore     = errors.New("closed KV store")
	ErrEntryNotFound     = errors.New("entry not found")
	ErrOpenKvStore       = errors.New("KV store is already open")

	BBOLT_DATA_BUCKET = []byte("data")

	JSON_SERIALIZATION_CONFIG = core.JSONSerializationConfig{ReprConfig: core.ALL_VISIBLE_REPR_CONFIG}

	_ core.DataStore = (*SerializedValueStorageAdapter)(nil)

	bboltOptions = &bbolt.Options{
		Timeout:      time.Second,
		NoGrowSync:   false,
		FreelistType: bbolt.FreelistArrayType,
	}
)

// A SingleFileKV is a key-value store that uses a Bbolt database under the hood,
// therefore data is stored on a single on-disk file.
type SingleFileKV struct {
	db *bbolt.DB

	path core.Path
	host core.Host

	transactionMapLock sync.Mutex
	transactions       map[*core.Transaction]*bbolt.Tx
}

type KvStoreConfig struct {
	Path core.Path
}

func OpenSingleFileKV(config KvStoreConfig) (_ *SingleFileKV, finalErr error) {
	path := string(config.Path)

	kv := &SingleFileKV{
		path: config.Path,

		transactions: map[*core.Transaction]*bbolt.Tx{},
	}

	db, err := bbolt.Open(path, BBOLT_FILE_FPERMS, bboltOptions)

	if errors.Is(err, bbolt.ErrTimeout) {
		finalErr = ErrOpenKvStore
		return
	}

	if err != nil {
		finalErr = err
		return
	}
	kv.db = db

	//create data bucket

	err = kv.db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(BBOLT_DATA_BUCKET)
		return err
	})
	if err != nil {
		finalErr = err
		return
	}

	return kv, nil
}

func (kv *SingleFileKV) Close(ctx *core.Context) (bboltError error) {
	defer func() {
		bboltError = kv.db.Close()
	}()

	logger := ctx.NewChildLoggerForInternalSource(KV_STORE_LOG_SRC)

	//before closing the BBolt database all the transactions are closed

	logger.Print("close KV store")

	kv.transactionMapLock.Lock()
	transactions := maps.Clone(kv.transactions)
	kv.transactionMapLock.Unlock()

	logger.Print("number of transactions to close: ", len(transactions))

	for tx, bboltTx := range transactions {
		func() {
			defer utils.Recover()
			//will be ignored if the transaction already finished.
			tx.Rollback(ctx)
		}()

		func() {
			defer utils.Recover()
			bboltTx.Rollback()
		}()
	}

	logger.Print("KV stored is now closed")
	//see the deferred call at the top
	return
}

func (kv *SingleFileKV) isClosed() bool {
	err := kv.db.View(func(tx *bbolt.Tx) error { return nil })
	return errors.Is(err, bbolt.ErrDatabaseNotOpen)
}

func (kv *SingleFileKV) Get(ctx *core.Context, key core.Path, db any) (core.Value, core.Bool, error) {
	serialized, found, err := kv.GetSerialized(ctx, key, db)

	if err != nil {
		return nil, found, err
	}

	if !found {
		return core.Nil, false, nil
	}

	val, err := core.ParseJSONRepresentation(ctx, serialized, nil)
	return val, true, err
}

func (kv *SingleFileKV) GetSerialized(ctx *core.Context, key core.Path, db any) (string, core.Bool, error) {
	if kv.isClosed() {
		return "", false, ErrClosedKvStore
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
		err := kv.db.View(func(txn *bbolt.Tx) error {
			bucket := txn.Bucket(BBOLT_DATA_BUCKET)

			item := bucket.Get([]byte(key))
			if item == nil {
				valueFound = core.False
				return nil
			}
			serialized = string(item)
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
		return ErrClosedKvStore
	}

	if fn == nil {
		return errors.New("iteration function is nil")
	}

	handleItem := func(key, serialized string) (cont bool) {
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
			return utils.Must(core.ParseJSONRepresentation(ctx, serialized, nil))
		}

		err := fn(path, getVal)
		return err == nil
	}

	iterWithTx := func(txn *bbolt.Tx) error {
		bucket := txn.Bucket(BBOLT_DATA_BUCKET)
		stopIteration := errors.New("")

		err := bucket.ForEach(func(k, v []byte) error {
			cont := handleItem(string(k), string(v))
			if !cont {
				return stopIteration
			}
			return nil
		})

		if errors.Is(err, stopIteration) {
			return nil
		}
		return err
	}

	kvTx := kv.getCreateDatabaseTxn(db, ctx.GetTx())

	if kvTx == nil {
		return kv.db.View(iterWithTx)
	} else {
		return iterWithTx(kvTx.tx)
	}
}

func (kv *SingleFileKV) UpdateNoCtx(fn func(dbTx *KVTx) error) error {
	if kv.isClosed() {
		return ErrClosedKvStore
	}

	if fn == nil {
		return errors.New("iteration function is nil")
	}

	return kv.db.Update(func(dbTx *bbolt.Tx) (finalErr error) {
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
		panic(ErrClosedKvStore)
	}

	if !key.IsAbsolute() {
		panic(ErrInvalidPathKey)
	}

	var (
		valueFound = core.True
		dbTx       = kv.getCreateDatabaseTxn(db, ctx.GetTx())
	)

	if dbTx == nil {
		err := kv.db.View(func(txn *bbolt.Tx) error {
			bucket := txn.Bucket(BBOLT_DATA_BUCKET)

			item := bucket.Get([]byte(key))
			if item == nil {
				valueFound = core.False
				return nil
			}
			return nil
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
	repr := core.MustGetJSONRepresentationWithConfig(value, ctx, JSON_SERIALIZATION_CONFIG)

	kv.InsertSerialized(ctx, key, string(repr), db)
}

func (kv *SingleFileKV) InsertSerialized(ctx *core.Context, key core.Path, serialized string, db any) {
	//TODO: check valid representation

	if kv.isClosed() {
		panic(ErrClosedKvStore)
	}

	if !key.IsAbsolute() {
		panic(ErrInvalidPathKey)
	}

	dbTx := kv.getCreateDatabaseTxn(db, ctx.GetTx())

	if dbTx == nil {
		err := kv.db.Update(func(txn *bbolt.Tx) error {
			bucket := txn.Bucket(BBOLT_DATA_BUCKET)
			k := []byte(key)

			//return an error if the entry already exists.
			if bucket.Get(k) != nil {
				return fmt.Errorf("%w: %s", ErrKeyAlreadyPresent, key)
			}

			return bucket.Put([]byte(key), []byte(serialized))
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
	repr := core.MustGetJSONRepresentationWithConfig(value, ctx, JSON_SERIALIZATION_CONFIG)
	kv.SetSerialized(ctx, key, string(repr), db)
}

func (kv *SingleFileKV) SetSerialized(ctx *core.Context, key core.Path, serialized string, db any) {
	//TODO: check valid representation

	if kv.isClosed() {
		panic(ErrClosedKvStore)
	}

	if !key.IsAbsolute() {
		panic(ErrInvalidPathKey)
	}

	dbtx := kv.getCreateDatabaseTxn(db, ctx.GetTx())

	if dbtx == nil {
		err := kv.db.Update(func(txn *bbolt.Tx) error {
			bucket := txn.Bucket(BBOLT_DATA_BUCKET)

			return bucket.Put([]byte(key), []byte(serialized))
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
	if kv.isClosed() {
		panic(ErrClosedKvStore)
	}

	if !key.IsAbsolute() {
		panic(ErrInvalidPathKey)
	}

	dbTx := kv.getCreateDatabaseTxn(db, ctx.GetTx())

	if dbTx == nil {
		err := kv.db.Update(func(txn *bbolt.Tx) error {
			bucket := txn.Bucket(BBOLT_DATA_BUCKET)

			return bucket.Delete([]byte(key))
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

// getCreateDatabaseTxn gets or creates a DatabaseTx associated with tx at certain conditions. If a DatabaseTx is already
// associated with $tx, it is returned. If $tx is terminated or terminating, nil is returned. Otherwise a DatabaseTx is
// associated with $tx and a termination callback is registered on $tx.
func (kv *SingleFileKV) getCreateDatabaseTxn(db any, tx *core.Transaction) *KVTx {
	if tx == nil {
		return nil
	}

	kv.transactionMapLock.Lock()
	defer kv.transactionMapLock.Unlock()
	dbTx, ok := kv.transactions[tx]

	if ok {
		return NewDatabaseTxIL(dbTx)
	}

	if tx.IsFinished() {
		dbTx.Rollback()
		return nil
	}

	if tx.IsFinishing() {
		//If the tx is terminating registering a termination callback will not work.
		return nil
	}

	//begin a new database transaction & add it to the core.Transaction.
	dbTx, err := kv.db.Begin(true)
	if err != nil {
		panic(err)
	}

	//add core.Transaction to KV.
	kv.transactions[tx] = dbTx

	err = tx.OnEnd(kv, makeTxEndcallbackFn(dbTx, tx, kv))
	if err != nil && !errors.Is(err, core.ErrFinishedTransaction) && !errors.Is(err, core.ErrFinishingTransaction) {
		panic(err)
	}

	return NewDatabaseTxIL(dbTx)
}

func makeTxEndcallbackFn(dbtx *bbolt.Tx, tx *core.Transaction, kv *SingleFileKV) func(t *core.Transaction, success bool) {
	return func(t *core.Transaction, success bool) {
		kv.transactionMapLock.Lock()
		if _, ok := kv.transactions[tx]; !ok {
			return
		}
		delete(kv.transactions, tx)
		kv.transactionMapLock.Unlock()

		if success {
			if !dbtx.Writable() {
				//Bbolt read-only transactions must be rolled back and not committed.
				dbtx.Rollback()
			} else if err := dbtx.Commit(); err != nil {
				panic(err)
			}
		} else if err := dbtx.Rollback(); err != nil {
			panic(err)
		}
	}
}
