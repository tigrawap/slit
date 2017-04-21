package main

import (
	"github.com/nsf/termbox-go"
	"github.com/tigrawap/slit/logging"
	"time"
)

type ALT_KEYMAP uint

const (
	NOT_ESCAPE ALT_KEYMAP = iota
	ESC
	ALT_LEFT_ARROW
	ALT_RIGHT_ARROW
	ALT_BACKSPACE
	ALT_D
)

// Returns escape key, sho
func getEscKey(ev termbox.Event) ALT_KEYMAP {
	if ev.Key != termbox.KeyEsc {
		return NOT_ESCAPE
	}
	closed := make(chan struct{})
	go func() {
		select {
		case <-time.After(10 * time.Nanosecond):
			termbox.Interrupt() // TODO: More reliable, sleepless way?
		case <-closed:
			return
		}
	}()
	switch ev := termbox.PollEvent(); ev.Type {
	case termbox.EventInterrupt:
		return ESC
	case termbox.EventKey:
		close(closed)
		switch ev.Key {
		case termbox.KeyBackspace, termbox.KeyBackspace2:
			return ALT_BACKSPACE
		}
		switch ev.Ch {
		case 'f':
			return ALT_RIGHT_ARROW
		case 'b':
			return ALT_LEFT_ARROW
		case 'd':
			return ALT_D
		}
		logging.Debug("unknown event", ev.Key, ev.Mod, ev.Ch)
	}
	return ESC
}
