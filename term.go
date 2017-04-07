package main

import (
	"github.com/nsf/termbox-go"
	"github.com/tigrawap/slit/ansi"
	"github.com/tigrawap/slit/logging"
)

type viewer struct {
	pos           int
	hOffset       int
	width         int
	height        int
	wrap          bool
	rf            RuneFile
	focus         Focusing
	info          infobar
	forwardSearch bool
	search        []rune
}

type action uint

const (
	NO_ACTION          action = iota
	ACTION_QUIT
	ACTION_RESET_FOCUS
)

type View interface {
}

type Focusing interface {
	View
	processKey(ev termbox.Event) action
}

type Navigator interface {
	Focusing
	navigate(direction int)
}

func (v *viewer) searchForward() {
	for i, line := range v.rf[v.pos:] {
		if i == 0 {
			// TODO: Check search index
			continue
		}
		if ansi.Index(line, v.search) != -1 {
			v.navigate(+i)
			break
		}
	}
}

func (v *viewer) searchBack() {
	var line ansi.Arunes
	prevLines := v.rf[:v.pos]
	for i := len(prevLines) - 1; i >= 0; i-- {
		line = prevLines[i]
		if ansi.Index(line, v.search) != -1 {
			v.navigate(i - len(prevLines))
			break
		}
	}
}

func (v *viewer) nextSearch(reverse bool) {
	if len(v.search) == 0 {
		return
	}
	if v.forwardSearch != reverse { // Basically XOR
		v.searchForward()
	} else {
		v.searchBack()
	}
}

var stylesMap = map[uint16]termbox.Attribute{
	1: termbox.AttrBold,
}

func (v *viewer) draw() {
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	for ty, dataLine := 0, v.pos; ty < v.height; ty++ {
		var tx int
		chars := v.rf[dataLine]
		if v.hOffset > len(chars)-1 {
			chars = chars[:0]
		} else {
			chars = chars[v.hOffset:]
		}
		var hlIndices []int
		hlChars := 0
		if len(v.search) != 0 {
			hlIndices = []int{}
			hlIndices = ansi.IndexAll(chars, v.search) //TODO: Optimize for longer string? (mind wrap)
		} else {
			hlIndices = []int{}
		}
		for i, char := range chars {
			bg := termbox.ColorBlack
			fg := termbox.ColorWhite
			if len(hlIndices) != 0 && hlChars == 0 {
				if hlIndices[0] == i {
					hlIndices = hlIndices[1:]
					hlChars = len(v.search)
				}
			}
			if hlChars != 0 {
				bg = termbox.ColorYellow
				hlChars--
			}
			if char.Attr.Fg != 0 {
				fg = termbox.Attribute(char.Attr.Fg-30+1) | stylesMap[char.Attr.Style]
			}
			if bg == termbox.ColorBlack { // If already highlighted by search - dont use original color
				if char.Attr.Bg != 0 {
					bg = termbox.Attribute(char.Attr.Bg - 40 + 1)
				}
			}
			termbox.SetCell(tx, ty, char.Rune, fg, bg)
			if !v.wrap {
			}
			tx++
			if tx >= v.width {
				if v.wrap {
					tx = 0
					ty++
				} else {
					break
				}
			}
		}
		if ty >= v.height {
			break
		}
		dataLine++
	}
	v.info.draw()
	termbox.Flush()
}

func (v *viewer) navigate(direction int) {
	newPos := v.pos + direction
	if newPos < 0 {
		newPos = 0
	}
	if newPos >= len(v.rf)-v.height {
		newPos = len(v.rf) - v.height
	}
	v.pos = newPos
	v.draw()
}

func (v *viewer) navigateEnd() {
	v.pos = len(v.rf) - v.height
	v.draw()
}

func (v *viewer) navigateStart() {
	v.pos = 0
	v.draw()
}

func (v *viewer) navigateRight() {
	v.wrap = false
	v.hOffset += v.width / 2
	v.draw()
}

func (v *viewer) navigateLeft() {
	v.wrap = false
	v.hOffset -= v.width / 2
	if v.hOffset < 0 {
		v.hOffset = 0
	}
	v.draw()
}

func (v *viewer) resetFocus() {
	v.focus = v
	termbox.HideCursor()
	termbox.Flush()
}

func (v *viewer) processKey(ev termbox.Event) (a action) {
	if ev.Ch != 0 {
		switch ev.Ch {
		case 'W':
			logging.Debug("switching wrapping")
			v.wrap = !v.wrap
			if v.wrap {
				v.hOffset = 0
			}
			v.draw()
		case 'q':
			logging.Debug("got key quit")
			return ACTION_QUIT
		case 'n':
			v.nextSearch(false)
		case 'N':
			v.nextSearch(true)
		case 'g':
			v.navigateStart()
		case 'G':
			v.navigateEnd()
		case '/':
			v.focus = &v.info
			v.forwardSearch = true
			v.info.reset(ibModeSearch)
		case '?':
			v.focus = &v.info
			v.forwardSearch = false
			v.info.reset(ibModeBackSearch)
		}
	} else {
		switch ev.Key {
		case termbox.KeyArrowDown:
			v.navigate(+1)
		case termbox.KeyArrowRight:
			v.navigateRight()
		case termbox.KeyArrowLeft:
			v.navigateLeft()
		case termbox.KeyArrowUp:
			v.navigate(-1)
		case termbox.KeyPgup:
			v.navigate(-v.height)
		case termbox.KeyPgdn:
			v.navigate(+v.height)
		case termbox.KeyHome:
			v.navigateStart()
		case termbox.KeyEnd:
			v.navigateEnd()
		}
	}
	return
}

func (v *viewer) resize(width, height int) {
	v.width, v.height = width, height
	v.height-- // Saving one line for infobar
	if v.pos+height >= len(v.rf) {
		v.pos = len(v.rf) - height + 1 // resize can lead to crash when expanding in the end of file
	}
	v.info.resize(v.width, v.height)
	v.draw()
}

var requestSearch = make(chan []rune)

func (v *viewer) termGui() {
	err := termbox.Init()
	if err != nil {
		panic(err)
	}
	defer termbox.Close()
	termbox.SetInputMode(termbox.InputEsc)
	termbox.SetOutputMode(termbox.Output256)
	v.info = infobar{y: 0, width: 0}
	v.focus = v
	v.resize(termbox.Size())
loop:
	for {
		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventKey:
			// Globals
			//logging.Debug(ev.Key, ev.Mod, ev.Ch)
			action := v.focus.processKey(ev)
			switch action {
			case ACTION_QUIT:
				break loop
			case ACTION_RESET_FOCUS:
				v.resetFocus()
			}
		case termbox.EventResize:
			logging.Debug("Resize event", ev.Width, ev.Height)
			v.resize(ev.Width, ev.Height)
		case termbox.EventError:
			panic(ev.Err)
		case termbox.EventInterrupt:
			select {
			case search := <-requestSearch:
				v.search = search
				v.nextSearch(false)
				v.draw()
			default:
			}
		}
	}

}
