package account

import (
	"encoding/json"
	"errors"
	"sync/atomic"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/buntdb"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	UnknownAccountToken = errors.New("unknown account token")
)

func OpenAnonymousAccountDatabase(ctx *core.Context, path core.Path, fls afs.Filesystem) (*AnonymousAccountDatabase, error) {

	readPerm := core.FilesystemPermission{
		Kind_:  permkind.Read,
		Entity: path,
	}

	if err := ctx.CheckHasPermission(readPerm); err != nil {
		return nil, err
	}

	writePerm := core.FilesystemPermission{
		Kind_:  permkind.Write,
		Entity: path,
	}

	if err := ctx.CheckHasPermission(writePerm); err != nil {
		return nil, err
	}

	//TODO: find a unique location on disk

	kvStore, err := buntdb.OpenBuntDBNoPermCheck(path.UnderlyingString(), fls)

	if err != nil {
		return nil, err
	}

	db := &AnonymousAccountDatabase{
		kv: kvStore,
	}
	ctx.OnGracefulTearDown(func(ctx *core.Context) error {
		return db.Close(ctx)
	})

	return db, nil
}

type AnonymousAccountDatabase struct {
	kv     *buntdb.DB
	closed atomic.Bool
}

func (db *AnonymousAccountDatabase) Persist(ctx *core.Context, account *AnonymousAccount) error {
	marshalled, err := json.Marshal(account)
	if err != nil {
		return err
	}

	path := string("/" + account.TokenHash)

	return utils.Catch(func() {
		tx, err := db.kv.Begin(true)
		if err != nil {
			panic(err)
		}
		defer tx.Commit()
		tx.Set(path, string(marshalled), nil)
	})
}

func (db *AnonymousAccountDatabase) GetAccount(ctx *core.Context, cleartextToken string) (*AnonymousAccount, error) {
	tokenHash, err := HashCleartextToken(cleartextToken)
	if err != nil {
		return nil, err
	}

	path := string("/" + tokenHash)

	readonlyTx, err := db.kv.Begin(false)
	if err != nil {
		return nil, err
	}

	marshalled, err := readonlyTx.Get(path)
	notFound := errors.Is(err, buntdb.ErrNotFound)

	if err != nil && !notFound {
		return nil, err
	}

	if notFound {
		return nil, UnknownAccountToken
	}

	var account AnonymousAccount
	err = json.Unmarshal([]byte(marshalled), &account)
	if err != nil {
		return nil, errors.New("failed to unmarshal account information")
	}

	if account.TokenHash != tokenHash {
		return nil, UnknownAccountToken
	}

	return &account, nil
}

func (db *AnonymousAccountDatabase) Close(ctx *core.Context) error {
	if !db.closed.CompareAndSwap(false, true) {
		return nil
	}
	return db.kv.Close()
}
