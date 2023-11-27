package inoxd

import (
	"bytes"
	"context"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/inoxd/cloud/cloudproxy"
	"github.com/inoxlang/inox/internal/inoxd/crypto"
	"github.com/inoxlang/inox/internal/inoxd/systemd/unitenv"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestDaemonCloudMode(t *testing.T) {
	//this tests required inox to be in /usr/local/go/bin.

	tmpDir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	keyset := crypto.GenerateRandomInoxdMasterKeyset()
	save, ok := os.LookupEnv(unitenv.INOXD_MASTER_KEYSET_ENV_VARNAME)
	if ok {
		defer os.Setenv(unitenv.INOXD_MASTER_KEYSET_ENV_VARNAME, save)
	}
	os.Setenv(unitenv.INOXD_MASTER_KEYSET_ENV_VARNAME, string(keyset))

	outputBuf := bytes.NewBuffer(nil)
	var lock sync.Mutex
	writer := utils.FnWriter{
		WriteFn: func(p []byte) (n int, err error) {
			lock.Lock()
			defer lock.Unlock()
			return outputBuf.Write(p)
		},
	}

	var helloAckReceived atomic.Bool

	hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		//fmt.Println(message)
		if strings.Contains(message, "ack received on connection to cloud-proxy") {
			helloAckReceived.Store(true)
		}
	})
	logger := zerolog.New(writer).Hook(hook)

	go Inoxd(InoxdArgs{
		Config: DaemonConfig{
			InoxCloud:      true,
			InoxBinaryPath: "inox",
		},
		Logger: logger,
		GoCtx:  ctx,

		DoNotUseCgroups: true,
		TestOnlyProxyConfig: &cloudproxy.CloudProxyConfig{
			CloudDataDir: tmpDir,
			Port:         6000,
		},
	})

	//wait for the connection between inoxd and the cloud-proxy to be established.
	time.Sleep(1 * time.Second)

	assert.True(t, helloAckReceived.Load())
}
