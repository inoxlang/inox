package account

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
)

func TestCreation(t *testing.T) {
	t.Skip("manual test")
	username := "<Github username>"
	hoster := Github

	fls := fs_ns.NewMemFilesystem(1_000_000)
	ctx := core.NewContextWithEmptyState(core.ContextConfig{
		Permissions: []core.Permission{
			core.HttpPermission{Kind_: permbase.Read, AnyEntity: true},
			core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")},
			core.FilesystemPermission{Kind_: permbase.Write, Entity: core.PathPattern("/...")},
			core.FilesystemPermission{Kind_: permbase.Delete, Entity: core.PathPattern("/...")},
		},
		Filesystem: fls,
	}, nil)
	defer ctx.CancelGracefully()

	db, err := OpenAnonymousAccountDatabase(ctx, "/accounts.kv", fls)
	if !assert.NoError(t, err) {
		return
	}

	printChan := make(chan string, 10)
	conn := &Connection{
		PrintFn:  func(text string) { printChan <- text },
		ReadChan: make(chan string),
	}

	var done atomic.Bool

	go func() {
		defer done.Store(true)
		defer func() {
			e := recover()
			if e != nil {
				err = utils.ConvertPanicValueToError(e)
			}
		}()
		err = CreateAnonymousAccountInteractively(ctx, hoster.String(), conn, db)
	}()

	select {
	case challengeExplanation := <-printChan:
		if !assert.Regexp(t, "explanation:.*(r|R)epository.*", challengeExplanation) {
			return
		}
		fmt.Println(challengeExplanation)
	case <-time.After(1 * time.Second):
		if done.Load() {
			assert.NoError(t, err)
		}
		assert.FailNow(t, "time out")
	}

	//give the tester enough time to create the repository.
	time.Sleep(25 * time.Second)

	select {
	case conn.ReadChan <- username:
	case <-time.After(time.Second):
		if done.Load() {
			assert.NoError(t, err)
		}
		assert.FailNow(t, "time out")
	}

	var token string
	select {
	case token = <-printChan:
		if !assert.Regexp(t, "token:.*", token) {
			return
		}
		fmt.Println(token)
	case <-time.After(1 * time.Second):
		if done.Load() {
			assert.NoError(t, err)
		}
		assert.FailNow(t, "time out")
	}

	//acknowledge token reception
	select {
	case conn.ReadChan <- "ack:token":
	case <-time.After(time.Second):
		if done.Load() {
			assert.NoError(t, err)
		}
		assert.FailNow(t, "time out")
	}

	if !utils.InefficientlyWaitUntilTrue(&done, 10*time.Second) {
		if done.Load() {
			assert.NoError(t, err)
		}
		assert.FailNow(t, "time out")
	}

	if !assert.NoError(t, err) {
		return
	}

	token = strings.TrimPrefix(token, "token:")

	account, err := db.GetAccount(ctx, token)
	if !assert.NoError(t, err) {
		return
	}

	assert.NotEmpty(t, account.ULID)
	assert.NotEmpty(t, account.InformationHash)
	assert.EqualValues(t, utils.Must(HashCleartextToken(token)), account.TokenHash)

	_, err = ulid.Parse(account.ULID)
	assert.NoError(t, err)
}
