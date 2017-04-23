package main

import (
	"context"
	"github.com/tigrawap/slit/ansi"
	"github.com/tigrawap/slit/logging"
	"github.com/tigrawap/slit/runes"
	"io"
	"sync"
)

type viewBuffer struct {
	fetcher     *Fetcher
	lock        sync.RWMutex // buffer updated in big chunks from Fetcher with rolling back-forward copies
	buffer      []Line
	pos         int // zero position in buffer
	resetPos    Pos // Zero Line to start displaying from if buffer is empty
	window      int // height of window, buffer size calculated in multiplies of window
	eofReached  bool
	originalPos Pos
}

func (b *viewBuffer) getLine(offset int) (ansi.Astring, error) {
	if b.pos+offset >= len(b.buffer) && !b.eofReached {
		b.fill()
	}
	if b.pos+offset >= len(b.buffer) || len(b.buffer) == 0 {
		b.eofReached = true
		return ansi.NewAstring([]byte{}), io.EOF
	}
	return b.buffer[b.pos+offset].Str, nil // TODO: What happens if we reached the end? panic!
}

func (b *viewBuffer) fill() {
	b.lock.Lock()
	defer b.lock.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fillFrom := b.resetPos
	if len(b.buffer) > 0 {
		fillFrom = b.buffer[len(b.buffer)-1].Pos //Will start getting from next Line, not from current
	}
	dataChan := b.fetcher.Get(ctx, fillFrom)
	for data := range dataChan {
		if len(b.buffer) > 0 {
			if b.buffer[len(b.buffer)-1].Offset == data.Offset {
				continue
			}
		}
		b.buffer = append(b.buffer, data) // will shrink it later(make async as well?), first let's fill the window
		if len(b.buffer[b.pos:]) >= b.window*3 {
			break
		}
	}
	tail := len(b.buffer[:b.pos])
	if tail > b.window*3 { // tail is too long, trim it
		b.eofReached = false
		cutFrom := tail - b.window*3
		b.buffer = b.buffer[cutFrom:]
		b.pos -= cutFrom
	}
}

func (b *viewBuffer) backFill() {
	b.lock.Lock()
	defer b.lock.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var prevLine Pos
	if len(b.buffer) != 0 {
		logging.Debug("Using previous Line using buffer", b.buffer[0].Pos)
		prevLine = b.buffer[0].Pos
	} else {
		logging.Debug("Going with resetPos set on reset", b.resetPos)
		prevLine = b.resetPos
	}
	logging.Debug("Trying to backfill from", prevLine)
	if prevLine.Offset < 0 {
		return // Nothing to backfill
	}
	dataChan := b.fetcher.GetBack(ctx, prevLine)
	newData := make([]Line, 0, b.window*3)
	for data := range dataChan {
		if len(b.buffer) != 0 && data.Offset == prevLine.Offset {
			continue // reached original line
		}
		newData = append(newData, data)
		if len(newData) >= b.window*3 {
			break
		}
	}
	if len(newData) == 0 {
		return //nothing to do here
	}
	//oldData := b.buffer[0:]
	if len(b.buffer) > b.window*2 {
		b.buffer = b.buffer[0 : b.window*2] // shrinking forward-buffer
	}
	oldDataLen := len(b.buffer)
	b.buffer = append(b.buffer, newData...)              // Ensuring that got enough space, expanding if needed
	copy(b.buffer[len(newData):], b.buffer[:oldDataLen]) // Shifting to make space on the left
	for i := len(newData) - 1; i >= 0; i-- {
		b.buffer[len(newData)-1-i] = newData[i] // Inserting values in reverse order
	}
	b.pos += len(newData)

}

func (b *viewBuffer) shift(direction int) {
	logging.Debug("direction", direction)
	defer func() {
		b.originalPos = b.currentLine().Pos
	}()
	if direction < 0 {
		logging.Debug("targeting ", b.pos+direction)
		if b.pos+direction < 0 {
			b.backFill()
		}
		b.pos = b.pos + direction
		if b.pos < 0 {
			b.pos = 0
		}
		return
	}
	//b.Line = b.Line + direction
	//if len(b.buffer)
	downShift := func() bool {
		if b.pos+direction < len(b.buffer)-1 {
			b.pos = b.pos + direction
			return true
		}
		return false
	}

	if downShift() {
		return
	}

	b.fill()
	if downShift() {
		return
	} else {
		if len(b.buffer) == 0 {
			return
		}
		b.pos = len(b.buffer) - 1
	}
}

func (b *viewBuffer) searchForward(searchFunc SearchFunc) int {
	for i, line := range b.buffer[b.pos:] {
		if i == 0 {
			// TODO: Maintain search index?( to navigate inside string)
			continue
		}
		if searchFunc(line.Str.Runes) != nil{
			return i
		}
	}
	return -1
}

func (b *viewBuffer) searchBack(searchFunc SearchFunc) int {
	prevLines := b.buffer[:b.pos]
	for i := 1; i <= len(prevLines); i++ {
		if searchFunc(prevLines[len(prevLines)-i].Str.Runes) != nil{
			return i
		}
	}
	return -1
}

func (b *viewBuffer) lastLine() Line {
	lastLine := len(b.buffer) - 1
	if lastLine != -1 {
		return b.buffer[lastLine]
	} else {
		return Line{}
	}
}

func (b *viewBuffer) currentLine() Line {
	if len(b.buffer) == 0 {
		return Line{}
	}
	return b.buffer[b.pos]
}

func (b *viewBuffer) reset(toLine Pos) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.buffer = b.buffer[:0]
	b.pos = 0
	b.resetPos = toLine
	b.originalPos = toLine
	b.eofReached = false
}

func (b *viewBuffer) refresh() {
	b.reset(b.currentLine().Pos)
}
