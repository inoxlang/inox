package filekv

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
	"go.etcd.io/bbolt"
)

type KVTx struct {
	tx     *bbolt.Tx
	bucket *bbolt.Bucket
}

func NewDatabaseTxIL(tx *bbolt.Tx) *KVTx {
	return &KVTx{
		tx:     tx,
		bucket: tx.Bucket(BBOLT_DATA_BUCKET),
	}
}

func (tx *KVTx) Get(ctx *core.Context, key core.Path) (result core.Value, valueFound core.Bool, finalErr error) {
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

func (tx *KVTx) GetSerialized(ctx *core.Context, key core.Path) (result string, valueFound core.Bool, finalErr error) {
	item := tx.bucket.Get([]byte(key))
	if item == nil {
		valueFound = false
	} else {
		valueFound = true
		result = string(item)
		return
	}
	return
}

func (tx *KVTx) Set(ctx *core.Context, key core.Path, value core.Serializable) error {
	repr := core.GetRepresentation(value, ctx)
	return tx.SetSerialized(ctx, key, string(repr))
}

func (tx *KVTx) SetSerialized(ctx *core.Context, key core.Path, serialized string) error {
	return tx.bucket.Put([]byte(key), []byte(serialized))
}

func (tx *KVTx) Insert(ctx *core.Context, key core.Path, value core.Serializable) error {
	repr := core.GetRepresentation(value, ctx)
	return tx.InsertSerialized(ctx, key, string(repr))
}

func (tx *KVTx) InsertSerialized(ctx *core.Context, key core.Path, serialized string) error {
	if tx.bucket.Get([]byte(key)) != nil {
		return ErrKeyAlreadyPresent
	}

	return tx.bucket.Put([]byte(key), []byte(serialized))
}

func (tx *KVTx) Delete(ctx *core.Context, key core.Path) error {
	return tx.bucket.Delete([]byte(key))
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
