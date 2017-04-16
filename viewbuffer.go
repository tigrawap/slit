package main

import (
	"sync"
	"context"
	"github.com/tigrawap/slit/logging"
	"github.com/tigrawap/slit/ansi"
	"github.com/tigrawap/slit/runes"
	"io"
)

type viewBuffer struct {
	fetcher *fetcher
	lock    sync.RWMutex // buffer updated in big chunks from fetcher with rolling back-forward copies
	buffer  []line
	pos     int // zero position in buffer
	zeroLine int // Zero line to start displaying from if buffer is empty
	window  int // height of window, buffer size calculated in multiplies of window
	eofReached bool
	originalPos int
}

func (b *viewBuffer) getLine(offset int) (ansi.Astring, error) {
	if b.pos+offset >= len(b.buffer)  {
		if !b.eofReached{
			b.fill()
		}
	}
	if b.pos+offset >= len(b.buffer)  || len(b.buffer) == 0{

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
	line := b.zeroLine
	if len(b.buffer) > 0 {
		line = b.buffer[len(b.buffer)-1].Pos + 1 //Will start getting from next line, not from current
	}
	dataChan := b.fetcher.Get(ctx, line)
	for data := range dataChan {
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
	logging.Debug("Trying to backfill")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	prevLine := b.zeroLine
	if b.zeroLine == 0 {
		return // Nothing to backfill?
	}
	if len(b.buffer) != 0{
		prevLine = b.buffer[0].Pos - 1
	}
	if prevLine <= 0 {
		return // still nothing to backfill
	}
	dataChan := b.fetcher.GetBack(ctx, prevLine)
	newData := make([]line, 0, b.window*3)
	for data := range dataChan {
		//logging.Debug("Backfill to buffer", string(data.Str.Runes), data.Pos, prevLine)
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
		b.buffer = b.buffer[0:b.window*2] // shrinking forward-buffer
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
	//b.pos = b.pos + direction
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
		if len(b.buffer) == 0{
			return
		}
		b.pos = len(b.buffer) - 1
	}
}

func (b *viewBuffer) searchForward(sub []rune) int {
	for i, line := range b.buffer[b.pos:] {
		if i == 0 {
			// TODO: Maintain search index?( to navigate inside string)
			continue
		}
		if runes.Index(line.Str.Runes, sub) != -1 {
			return i
		}
	}
	return -1
}

func (b *viewBuffer) searchBack(sub []rune) int {
	prevLines := b.buffer[:b.pos]
	for i := 1; i <= len(prevLines); i ++ {
		if runes.Index(prevLines[len(prevLines)-i].Str.Runes, sub) != -1 {
			return i
		}
	}
	return -1
}

func (b *viewBuffer) lastLine() line{
	lastLine := len(b.buffer)-1
	if lastLine != -1{
		return b.buffer[lastLine]
	}else{
		logging.Debug("Fetching last line when no line available")
		return line{}
	}
}

func (b *viewBuffer) currentLine() line{
	if len(b.buffer) == 0{
		return line{}
	}
	return b.buffer[b.pos]
}

func (b *viewBuffer) reset(toLine int){
	b.lock.Lock()
	defer b.lock.Unlock()
	b.eofReached = false
	b.buffer = b.buffer[:0]
	b.pos = 0
	b.zeroLine = toLine
	b.originalPos = toLine
}