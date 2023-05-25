//go:build unix

package internal

import (
	badger "github.com/dgraph-io/badger/v3"
	core "github.com/inoxlang/inox/internal/core"
	internal "github.com/inoxlang/inox/internal/core"
)

type underlying struct {
	db   *badger.DB
	path core.Path
	host Host
}

func openUnderlying(config LocalDatabaseConfig) (_ underlying, finalErr error) {
	opts := badger.DefaultOptions(string(config.Path))

	if config.InMemory {
		opts = opts.WithInMemory(true)
	}

	db, err := badger.Open(opts)
	if err != nil {
		finalErr = err
		return
	}

	return underlying{
		db:   db,
		path: config.Path,
		host: config.Host,
	}, nil
}

func (u underlying) close() {
	u.db.Close()
	if u.db.IsClosed() {
		dbRegistry.lock.Lock()
		defer dbRegistry.lock.Unlock()

		delete(dbRegistry.openDatabases, u.path)
	}
}

func (u underlying) isClosed() bool {
	return u.db.IsClosed()
}

func (u underlying) get(ctx *Context, key Path, db any) (Value, Bool) {
	if u.isClosed() {
		panic(ErrDatabaseClosed)
	}

	if !key.IsAbsolute() {
		panic(ErrInvalidPathKey)
	}

	var (
		r          = getFullResourceName(u.host, key)
		valueFound = core.True
		val        Value
		b          []byte
		tx         = ctx.GetTx()
	)

	if tx == nil {
		err := u.db.View(func(txn *badger.Txn) error {
			item, err := txn.Get([]byte(key))
			if err == badger.ErrKeyNotFound {
				valueFound = core.False
				return nil
			} else if err != nil {
				return err
			}

			_ = item.Value(func(val []byte) error {
				b = val
				return nil
			})

			val, err = internal.ParseRepr(ctx, b)
			return err
		})

		if err != nil {
			panic(err)
		}

	} else {
		dbtx := u.getCreateDatabaseTxn(db, tx)

		item, err := dbtx.Get([]byte(key))
		if err == badger.ErrKeyNotFound {
			valueFound = core.False
		} else if err != nil {
			panic(err)
		} else {
			_ = item.Value(func(val []byte) error {
				b = val
				return nil
			})

			val, err = internal.ParseRepr(ctx, b)

			if err != nil {
				panic(err)
			}
		}
	}

	if valueFound {
		if err := ctx.AcquireResource(r); err != nil {
			panic(err)
		}
	}

	if val == nil {
		val = core.Nil
	}

	return val, valueFound
}

func (u underlying) getCreateDatabaseTxn(db any, tx *core.Transaction) *badger.Txn {
	v, err := tx.GetValue(db)
	if err != nil {
		panic(err)
	}
	dbtx, ok := v.(*badger.Txn)

	if !ok {
		dbtx = u.db.NewTransaction(true)
		if err = tx.SetValue(db, dbtx); err != nil {
			panic(err)
		}

		if err = tx.OnEnd(db, makeTxEndcallbackFn(dbtx)); err != nil {
			panic(err)
		}
	}
	return dbtx
}

func (u underlying) has(ctx *Context, key Path, db any) Bool {
	if u.isClosed() {
		panic(ErrDatabaseClosed)
	}

	if !key.IsAbsolute() {
		panic(ErrInvalidPathKey)
	}

	var (
		valueFound = core.True
		tx         = ctx.GetTx()
	)

	if tx == nil {
		err := u.db.View(func(txn *badger.Txn) error {
			_, err := txn.Get([]byte(key))
			if err == badger.ErrKeyNotFound {
				valueFound = core.False
				return nil
			}
			return err
		})

		if err != nil {
			panic(err)
		}

	} else {
		dbtx := u.getCreateDatabaseTxn(db, tx)

		_, err := dbtx.Get([]byte(key))
		if err == badger.ErrKeyNotFound {
			valueFound = core.False
		} else if err != nil {
			panic(err)
		}

	}

	return valueFound
}

func (u underlying) set(ctx *Context, key Path, value Value, db any) {

	if u.db.IsClosed() {
		panic(ErrDatabaseClosed)
	}

	if !key.IsAbsolute() {
		panic(ErrInvalidPathKey)
	}

	tx := ctx.GetTx()
	r := getFullResourceName(u.host, key)

	if tx == nil {
		err := u.db.Update(func(txn *badger.Txn) error {
			repr := core.GetRepresentation(value, ctx)
			return txn.Set([]byte(key), []byte(repr))
		})

		if err != nil {
			panic(err)
		}

	} else {
		dbtx := u.getCreateDatabaseTxn(db, tx)

		repr := core.GetRepresentation(value, ctx)
		if err := dbtx.Set([]byte(key), []byte(repr)); err != nil {
			panic(err)
		}

	}

	if err := ctx.AcquireResource(r); err != nil {
		panic(err)
	}
}

func makeTxEndcallbackFn(dbtx *badger.Txn) func(t *internal.Transaction, success bool) {
	return func(t *internal.Transaction, success bool) {
		defer dbtx.Discard()
		if success {
			dbtx.Commit()
		}
	}
}
