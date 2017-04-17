package main

import (
	"github.com/nsf/termbox-go"
	"github.com/tigrawap/slit/ansi"
	"github.com/tigrawap/slit/logging"
	"context"
	"github.com/tigrawap/slit/runes"
	"runtime"
	"time"
	"io"
	"code.cloudfoundry.org/bytefmt"
	"strconv"
)

type viewer struct {
	pos           int
	hOffset       int
	width         int
	height        int
	wrap          bool
	fetcher       *fetcher
	focus         Focusing
	info          infobar
	searchMode    infobarMode
	forwardSearch bool
	search        []rune
	buffer        viewBuffer
	keepChars     int
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
	index := -1
	if index = v.buffer.searchForward(v.search); index != -1 {
		v.navigate(index)
		return
	}
	if index = v.fetcher.Search(context.TODO(), v.buffer.lastLine().Pos, v.search); index != -1 {
		v.buffer.reset(index)
		v.draw()
	}
}

func (v *viewer) searchBack() {
	index := -1
	if index = v.buffer.searchBack(v.search); index != -1 {
		v.navigate(-index)
		return
	}
	if index = v.fetcher.SearchBack(context.TODO(), v.buffer.currentLine().Pos, v.search); index != -1 {
		v.buffer.reset(index)
		v.draw()
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

func (v *viewer) addFilter(filter filter) {
	v.fetcher.filters = append(v.fetcher.filters, filter)
	v.fetcher.filtersEnabled = true
	v.buffer.reset(v.buffer.currentLine().Pos)
}

func (v *viewer) switchFilters() {
	v.fetcher.filtersEnabled = !v.fetcher.filtersEnabled
	v.buffer.reset(v.buffer.currentLine().Pos)
	v.draw()
}

var stylesMap = map[uint8]termbox.Attribute{
	1: termbox.AttrBold,
}

func (v *viewer) replaceWithKeptChars(chars []rune, attrs []ansi.RuneAttr, data ansi.Astring) ([]rune, []ansi.RuneAttr) {
	if v.keepChars != 0 && !v.wrap {
		shift := min(v.hOffset, v.keepChars)
		if shift != 0 {
			if shift < len(chars) {
				chars = chars[shift:]
				attrs = attrs[shift:]
			}
			keptChars := make([]rune, shift, shift+len(chars))
			keptAttrs := make([]ansi.RuneAttr, shift, shift+len(chars))
			copy(keptChars, data.Runes[:min(shift, len(data.Runes))])
			copy(keptAttrs, data.Attrs[:min(shift, len(data.Runes))])
			for i, _ := range keptAttrs {
				attr := &keptAttrs[i]
				attr.Fg = ansi.FgColor(ansi.ColorBlue)
				//attr.Bg = ansi.BgColor(ansi.ColorBlue)
				//attr.Style = uint8(ansi.StyleBold)
			}
			chars = append(keptChars, chars...)
			attrs = append(keptAttrs, attrs...)
		}
	}
	return chars, attrs
}

func (v *viewer) draw() {
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	var chars []rune
	var attrs []ansi.RuneAttr
	var attr ansi.RuneAttr
	var bg termbox.Attribute
	var fg termbox.Attribute
	var hlIndices []int
	var hlChars int
	var tx int
	for ty, dataLine := 0, 0; ty < v.height; ty++ {
		tx = 0
		hlChars = 0
		data, err := v.buffer.getLine(dataLine)
		if err == io.EOF {
			break
		}
		if v.hOffset > len(data.Runes)-1 {
			chars = data.Runes[:0]
			attrs = data.Attrs[:0]
		} else {
			chars = data.Runes[v.hOffset:]
			attrs = data.Attrs[v.hOffset:]
		}
		chars, attrs = v.replaceWithKeptChars(chars, attrs, data)
		if len(v.search) != 0 {
			hlIndices = runes.IndexAll(chars, v.search)
		} else {
			hlIndices = []int{}
		}
		for i, char := range chars {
			attr = attrs[i]
			bg = termbox.ColorDefault
			fg = termbox.ColorDefault
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
			if attr.Fg != 0 {
				fg = termbox.Attribute(attr.Fg-30+1) | stylesMap[attr.Style]
			}
			if bg == termbox.ColorDefault { // If already highlighted by search - dont use original color
				if attr.Bg != 0 {
					bg = termbox.Attribute(attr.Bg - 40 + 1)
				}
			}
			termbox.SetCell(tx, ty, char, fg, bg)
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
	v.buffer.shift(direction)
	v.draw()
}

func (v *viewer) navigateEnd() {
	v.buffer.reset(v.fetcher.lastLine())
	v.navigate(- v.height + 1)
	v.draw()
}

func (v *viewer) navigateStart() {
	v.buffer.reset(0)
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
		case 'f':
			v.navigate(+v.height)
		case 'b':
			v.navigate(-v.height)
		case '/':
			v.focus = &v.info
			v.info.reset(ibModeSearch)
		case '&':
			v.focus = &v.info
			v.info.reset(ibModeFilter)
		case '?':
			v.focus = &v.info
			v.info.reset(ibModeBackSearch)
		case '+':
			v.focus = &v.info
			v.info.reset(ibModeAppend)
		case '-':
			v.focus = &v.info
			v.info.reset(ibModeExclude)
		case 'M':
			reportSystemUsage()
		case '=':
			v.fetcher.filters = v.fetcher.filters[:0]
			v.buffer.reset(v.buffer.currentLine().Pos)
			v.draw()
		case 'C':
			v.switchFilters()
		case 'K':
			v.focus = &v.info
			v.info.reset(ibModeKeepCharacters)

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
	v.info.resize(v.width, v.height)
	v.buffer.window = v.height
	v.draw()
}

type infobarRequest struct {
	str  []rune
	mode infobarMode
}

var requestSearch = make(chan infobarRequest)

func (v *viewer) termGui() {
	err := termbox.Init()
	if err != nil {
		panic(err)
	}
	defer termbox.Close()
	termbox.SetInputMode(termbox.InputEsc)
	termbox.SetOutputMode(termbox.Output256)
	v.info = infobar{
		y:              0,
		width:          0,
		currentLine:    &v.buffer.originalPos,
		totalLines:     &v.fetcher.totalLines,
		filtersEnabled: &v.fetcher.filtersEnabled,
	}
	v.focus = v
	v.buffer = viewBuffer{
		fetcher: v.fetcher,
	}
	v.resize(termbox.Size())
loop:
	for {
		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventKey:
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
				v.processInfobarRequest(search)
			case <-time.After(10 * time.Millisecond):
				continue
			}
		}
	}

}
func (v *viewer) processInfobarRequest(search infobarRequest) {
	defer logging.Timeit("Got search request")()
	switch search.mode {
	case ibModeFilter:
		v.addFilter(&includeFilter{search.str, false})
	case ibModeAppend:
		v.addFilter(&includeFilter{search.str, true})
	case ibModeExclude:
		v.addFilter(&excludeFilter{search.str})
	case ibModeSearch:
		v.search = search.str
		v.forwardSearch = true
		v.nextSearch(false)
	case ibModeBackSearch:
		v.search = search.str
		v.forwardSearch = false
		v.nextSearch(false)
	case ibModeKeepCharacters:
		keep, err := strconv.Atoi(string(search.str))
		if err != nil{
			logging.Debug("Err: Keepchar: ", err)
			v.keepChars = 0
		}else{
			v.keepChars = keep
		}
	}
	v.draw()
}

func reportSystemUsage() {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	logging.Debug(mem.Alloc)
	logging.Debug("Total alloc", bytefmt.ByteSize(mem.TotalAlloc))
	logging.Debug("Sys", bytefmt.ByteSize(mem.Sys))
	logging.Debug("Heap alloc", bytefmt.ByteSize(mem.HeapAlloc))
	logging.Debug("Heap sys", bytefmt.ByteSize(mem.HeapSys))
	logging.Debug("Goroutines num", runtime.NumGoroutine())
	runtime.GC()
}
