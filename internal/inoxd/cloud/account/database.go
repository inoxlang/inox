package account

import (
	"encoding/json"
	"errors"
	"sync/atomic"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/filekv"
	"github.com/inoxlang/inox/internal/permkind"
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

	store, err := filekv.OpenSingleFileKV(filekv.KvStoreConfig{
		Path:       path,
		Filesystem: fls,
	})

	if err != nil {
		return nil, err
	}

	db := &AnonymousAccountDatabase{
		kv: store,
	}
	ctx.OnGracefulTearDown(func(ctx *core.Context) error {
		return db.Close(ctx)
	})

	return db, nil
}

type AnonymousAccountDatabase struct {
	kv     *filekv.SingleFileKV
	closed atomic.Bool
}

func (db *AnonymousAccountDatabase) Persist(ctx *core.Context, account *AnonymousAccount) error {
	marshalled, err := json.Marshal(account)
	if err != nil {
		return err
	}

	path := core.Path("/" + account.TokenHash)

	return utils.Catch(func() {
		db.kv.SetSerialized(ctx, path, string(marshalled), db)
	})
}

func (db *AnonymousAccountDatabase) GetAccount(ctx *core.Context, cleartextToken string) (*AnonymousAccount, error) {
	tokenHash, err := HashCleartextToken(cleartextToken)
	if err != nil {
		return nil, err
	}

	path := core.Path("/" + tokenHash)

	marshalled, found, err := db.kv.GetSerialized(ctx, path, db)
	if err != nil {
		return nil, err
	}

	if !found {
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
	return db.kv.Close(ctx)
}
