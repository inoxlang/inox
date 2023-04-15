package internal

import (
	"testing"
	"time"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
)

func TestShell(t *testing.T) {
	//TODO: fix data race + rework cd
	//
	//
	t.Skip()

	t.Run("ring buffers inputs & outputs", func(t *testing.T) {

		fgColor := core.ColorFromAnsiColor(termenv.ANSIWhite)
		bgColor := core.ColorFromAnsiColor(termenv.ANSIBlack)
		config := REPLConfiguration{
			PrintingConfig: PrintingConfig{
				defaultFgColor:                 fgColor,
				defaultFgColorSequence:         fgColor.GetAnsiEscapeSequence(false),
				backgroundColor:                bgColor,
				defaultBackgroundColorSequence: bgColor.GetAnsiEscapeSequence(true),
			},
			prompt: core.NewWrappedValueList(),
		}

		ctx := core.NewContext(core.ContextConfig{})
		defer ctx.Cancel()

		buffSize := 1000

		in := core.NewRingBuffer(ctx, core.ByteCount(buffSize))
		out := core.NewRingBuffer(ctx, core.ByteCount(buffSize))

		go func() {
			state := core.NewGlobalState(ctx)

			sh := newShell(config, state, in, out)
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
