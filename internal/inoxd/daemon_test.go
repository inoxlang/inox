//go:build reqbin

package inoxd

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode"

	"github.com/gorilla/websocket"
	"github.com/inoxlang/inox/internal/inoxd/cloud/cloudproxy"
	"github.com/inoxlang/inox/internal/inoxd/crypto"
	"github.com/inoxlang/inox/internal/inoxd/systemd/unitenv"
	"github.com/inoxlang/inox/internal/projectserver"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/inoxlang/inox/internal/utils/processutils"
	"github.com/rs/zerolog"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/stretchr/testify/assert"
)

func TestDaemonSingleProjectServerMode(t *testing.T) {
	//this test suite require inox to be in /usr/local/go/bin.

	t.SkipNow()

	newLockedWriter := func(outputBuf *bytes.Buffer) io.Writer {
		var lock sync.Mutex
		return utils.FnWriter{
			WriteFn: func(p []byte) (n int, err error) {
				lock.Lock()
				defer lock.Unlock()
				return outputBuf.Write(p)
			},
		}
	}

	dialer := *websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	t.Run("base case", func(t *testing.T) {
		//setup
		tmpDir := t.TempDir()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer func() {
			cancel()

			//wait for teardown to finish.
			time.Sleep(time.Second)
		}()

		keyset := crypto.GenerateRandomInoxdMasterKeyset()
		save, ok := os.LookupEnv(unitenv.INOXD_MASTER_KEYSET_ENV_VARNAME)
		if ok {
			defer os.Setenv(unitenv.INOXD_MASTER_KEYSET_ENV_VARNAME, save)
		}
		os.Setenv(unitenv.INOXD_MASTER_KEYSET_ENV_VARNAME, string(keyset))

		outputBuf := bytes.NewBuffer(nil)
		writer := newLockedWriter(outputBuf)
		//////

		var sessionCreatedOnServer atomic.Bool

		hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
			//fmt.Println(message)
			if strings.Contains(message, "new session at 127.0.0.1") {
				sessionCreatedOnServer.Store(true)
			}
		})
		logger := zerolog.New(writer).Hook(hook)

		go Inoxd(InoxdArgs{
			Config: DaemonConfig{
				InoxCloud:      false,
				InoxBinaryPath: "inox",
				Server: projectserver.IndividualServerConfig{
					ProjectsDir: tmpDir,
					Port:        6000,
				},
			},
			Logger: logger,
			GoCtx:  ctx,

			DoNotUseCgroups: true,
		})

		//wait for the project server to start.
		time.Sleep(time.Second)

		c, _, err := dialer.Dial("wss://localhost:6000", nil)
		if !errors.Is(err, io.EOF) && !assert.NoError(t, err, "failed to connect") {
			return
		}
		c.Close()

		//wait for the logs.
		time.Sleep(100 * time.Millisecond)

		if !assert.True(t, sessionCreatedOnServer.Load()) {
			return
		}
	})

	t.Run("killing the project-server process once should not cause any issue", func(t *testing.T) {

		tmpDir := t.TempDir()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer func() {
			cancel()

			//wait for teardown to finish.
			time.Sleep(time.Second)
		}()

		keyset := crypto.GenerateRandomInoxdMasterKeyset()
		save, ok := os.LookupEnv(unitenv.INOXD_MASTER_KEYSET_ENV_VARNAME)
		if ok {
			defer os.Setenv(unitenv.INOXD_MASTER_KEYSET_ENV_VARNAME, save)
		}
		os.Setenv(unitenv.INOXD_MASTER_KEYSET_ENV_VARNAME, string(keyset))

		outputBuf := bytes.NewBuffer(nil)
		writer := newLockedWriter(outputBuf)
		//////

		var sessionCreatedOnServer atomic.Int32
		var projectServerPid atomic.Int32

		//logger hook used to check that the hello ack has been received and to get the PID.
		hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
			if strings.Contains(message, "new session at 127.0.0.1") {
				sessionCreatedOnServer.Add(1)
			}

			//detect logging of the PID.
			p := reflect.ValueOf(e).Elem().FieldByName("buf").Bytes()
			jsonFieldName := []byte(processutils.NEW_PROCESS_PID_LOG_FIELD_NAME + `":`)
			fieldIndex := bytes.Index(p, jsonFieldName)

			if fieldIndex >= 0 {
				pidIndex := fieldIndex + len(jsonFieldName)
				i := pidIndex

				for i < len(p) {
					r := rune(p[i])
					if !unicode.IsDigit(r) {
						break
					}

					i++
				}

				if i == len(p)-1 {
					i++
				}

				pid, err := strconv.Atoi(string(p[pidIndex:i]))
				if err != nil {
					assert.Fail(t, err.Error())
				}
				projectServerPid.Store(int32(pid))
			}
		})
		logger := zerolog.New(writer).Hook(hook)

		go Inoxd(InoxdArgs{
			Config: DaemonConfig{
				InoxCloud:      false,
				InoxBinaryPath: "inox",
				Server: projectserver.IndividualServerConfig{
					ProjectsDir: tmpDir,
					Port:        6000,
				},
			},
			Logger: logger,
			GoCtx:  ctx,

			DoNotUseCgroups: true,
		})

		//wait for the project server to start.
		time.Sleep(time.Second)

		c, _, err := dialer.Dial("wss://localhost:6000", nil)
		if !assert.NoError(t, err, "failed to connect") {
			return
		}
		c.Close()

		//wait for the logs.
		time.Sleep(100 * time.Millisecond)
		assert.EqualValues(t, 1, sessionCreatedOnServer.Load())

		pid := projectServerPid.Load()
		if !assert.NotZero(t, pid) {
			return
		}

		//kill the process and wait for a new connection to be established.
		process := utils.Must(process.NewProcess(pid))
		process.Kill()
		//wait for the project server to start.
		time.Sleep(time.Second)

		c, _, err = dialer.Dial("wss://localhost:6000", nil)
		if !errors.Is(err, io.EOF) && !assert.NoError(t, err, "failed to connect") {
			return
		}
		c.Close()

		//wait for the logs.
		time.Sleep(10 * time.Millisecond)
		if !assert.EqualValues(t, 2, sessionCreatedOnServer.Load()) {
			return
		}
	})
}

func TestDaemonCloudMode(t *testing.T) {
	//this test suite require inox to be in /usr/local/go/bin.

	newLockedWriter := func(outputBuf *bytes.Buffer) io.Writer {
		var lock sync.Mutex
		return utils.FnWriter{
			WriteFn: func(p []byte) (n int, err error) {
				lock.Lock()
				defer lock.Unlock()
				return outputBuf.Write(p)
			},
		}
	}

	t.Run("base case", func(t *testing.T) {
		//setup
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
		writer := newLockedWriter(outputBuf)
		//////

		var helloAckReceived atomic.Bool

		//logger hook used to check that the hello ack has been received.
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
	})

	t.Run("killing the cloud proxy process once should not cause any issue", func(t *testing.T) {
		//setup
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
		writer := newLockedWriter(outputBuf)
		//////

		var helloAckCount atomic.Int32
		var proxyPid atomic.Int32

		//logger hook used to check that the hello ack has been received and to get the PID.
		hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
			if strings.Contains(message, "ack received on connection to cloud-proxy") {
				helloAckCount.Add(1)
			}

			//detect logging of the PID.
			p := reflect.ValueOf(e).Elem().FieldByName("buf").Bytes()
			jsonFieldName := []byte(processutils.NEW_PROCESS_PID_LOG_FIELD_NAME + `":`)
			fieldIndex := bytes.Index(p, jsonFieldName)

			if fieldIndex >= 0 {
				pidIndex := fieldIndex + len(jsonFieldName)
				i := pidIndex

				for i < len(p) {
					r := rune(p[i])
					if !unicode.IsDigit(r) {
						break
					}

					i++
				}

				if i == len(p)-1 {
					i++
				}

				pid, err := strconv.Atoi(string(p[pidIndex:i]))
				if err != nil {
					assert.Fail(t, err.Error())
				}
				proxyPid.Store(int32(pid))
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
		time.Sleep(time.Second)

		assert.EqualValues(t, 1, helloAckCount.Load())

		pid := proxyPid.Load()
		if !assert.NotZero(t, pid) {
			return
		}

		//kill the process and wait for a new connection to be established.
		process := utils.Must(process.NewProcess(pid))
		process.Kill()
		time.Sleep(time.Second)

		assert.EqualValues(t, 2, helloAckCount.Load())
	})
}
