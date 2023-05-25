//go:build unix

package internal

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
)

func handleSignalsInGoroutine(sh *shell, prevTermState *term.State) {
	signal.Reset()
	signalChan := make(chan os.Signal, 1)
	signal.Ignore(syscall.SIGTTOU, syscall.SIGTTIN, syscall.SIGTSTP)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGWINCH, syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGQUIT)

	go func() {
		for s := range signalChan {
			switch s {
			case os.Interrupt:
				continue
			case syscall.SIGTERM:
				term.Restore(sh.inFd, prevTermState)
				os.Exit(0)
			case syscall.SIGWINCH:
				continue
			}
		}
	}()
}
