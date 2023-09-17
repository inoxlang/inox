package inoxsh_ns

import (
	"io"
	"testing"
	"time"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
)

const buffSize = 10_000

func TestShell(t *testing.T) {

	//TODO: fix data race + rework cd
	//
	//
	t.Skip()

	t.Run("literal", func(t *testing.T) {
		ctx, config, in, out := setup()
		defer ctx.CancelGracefully()

		go func() {
			state := core.NewGlobalState(ctx)

			sh := newShell(config, state, in, out, out)
			sh.runLoop()
		}()

		_, err := in.Write([]byte("14580\n"))
		assert.NoError(t, err)

		time.Sleep(time.Second / 10)

		b := make([]byte, buffSize)
		n, err := out.Read(b)
		assert.NoError(t, err)

		assert.Contains(t, string(b[:n]), "14580")
	})

	t.Run("pipe", func(t *testing.T) {
		ctx, config, in, out := setup()
		defer ctx.CancelGracefully()

		go func() {
			state := core.NewGlobalState(ctx)

			sh := newShell(config, state, in, out, out)
			sh.runLoop()
		}()

		_, err := in.Write([]byte("idt [{title: \"hello\"}] | map ~$ .title\n"))
		assert.NoError(t, err)

		time.Sleep(time.Second / 10)

		b := make([]byte, buffSize)
		n, err := out.Read(b)
		assert.NoError(t, err)

		assert.Contains(t, string(b[:n]), "hello")
	})

	t.Run("ring buffers inputs & outputs", func(t *testing.T) {
		ctx, config, in, out := setup()
		defer ctx.CancelGracefully()

		go func() {
			state := core.NewGlobalState(ctx)

			sh := newShell(config, state, in, out, out)
			sh.runLoop()
		}()

		_, err := in.Write([]byte("print 49108\n"))
		assert.NoError(t, err)

		time.Sleep(time.Second / 10)

		b := make([]byte, buffSize)
		n, err := out.Read(b)
		assert.NoError(t, err)

		assert.Contains(t, string(b[:n]), "49108")
	})

}

func setup() (ctx *core.Context, config REPLConfiguration, in io.ReadWriter, out io.ReadWriter) {

	fgColor := core.ColorFromAnsiColor(termenv.ANSIWhite)
	bgColor := core.ColorFromAnsiColor(termenv.ANSIBlack)
	config = REPLConfiguration{
		PrintingConfig: PrintingConfig{
			defaultFgColor:                 fgColor,
			defaultFgColorSequence:         fgColor.GetAnsiEscapeSequence(false),
			backgroundColor:                bgColor,
			defaultBackgroundColorSequence: bgColor.GetAnsiEscapeSequence(true),
			prettyPrintConfig:              defaultPrettyPrintConfig,
		},
		prompt: core.NewWrappedValueList(),
	}

	ctx = core.NewContext(core.ContextConfig{})

	in = core.NewRingBuffer(ctx, core.ByteCount(buffSize))
	out = core.NewRingBuffer(ctx, core.ByteCount(buffSize))
	return
}
