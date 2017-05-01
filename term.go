package main

import (
	"code.cloudfoundry.org/bytefmt"
	"context"
	"github.com/nsf/termbox-go"
	"github.com/tigrawap/slit/ansi"
	"github.com/tigrawap/slit/logging"
	"io"
	"runtime"
	"strconv"
	"sync"
	"time"
)

type viewer struct {
	pos           int
	hOffset       int
	width         int
	height        int
	wrap          bool
	fetcher       *Fetcher
	focus         Focusing
	info          infobar
	searchMode    infobarMode
	forwardSearch bool
	search        []rune
	buffer        viewBuffer
	keepChars     int
	ctx           context.Context
	following     bool
}

type action uint

const (
	NO_ACTION action = iota
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
	searchFunc, err := getSearchFunc(v.info.searchType, v.search)
	if err != nil {
		return
	}
	if distance := v.buffer.searchForward(searchFunc); distance != -1 {
		v.navigate(distance)
		return
	}
	if pos := v.fetcher.Search(context.TODO(), v.buffer.lastLine().Pos, searchFunc); pos != POS_NOT_FOUND {
		v.buffer.reset(pos)
		v.draw()
	}
}

func (v *viewer) searchBack() {
	searchFunc, err := getSearchFunc(v.info.searchType, v.search)
	if err != nil {
		return
	}
	if distance := v.buffer.searchBack(searchFunc); distance != -1 {
		v.navigate(-distance)
		return
	}
	fromPos := v.buffer.currentLine().Pos
	if fromPos.Line > 0 {
		fromPos.Line--
	}
	fromPos.Offset--
	if pos := v.fetcher.SearchBack(context.TODO(), fromPos, searchFunc); pos != POS_NOT_FOUND {
		v.buffer.reset(pos)
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

func (v *viewer) addFilter(sub []rune, action FilterAction) {
	filter, err := NewFilter(sub, action, v.info.searchType)
	if err != nil {
		logging.Debug(err)
		return
	}
	v.fetcher.lock.Lock()
	v.fetcher.filters = append(v.fetcher.filters, filter)
	v.fetcher.filtersEnabled = true
	v.buffer.reset(v.buffer.currentLine().Pos)
	v.fetcher.lock.Unlock()
}

func (v *viewer) switchFilters() {
	v.fetcher.filtersEnabled = !v.fetcher.filtersEnabled
	v.buffer.reset(v.buffer.currentLine().Pos)
	v.draw()
}

var stylesMap = map[uint8]termbox.Attribute{
	1: termbox.AttrBold,
	7: termbox.AttrReverse,
}

func (v *viewer) replaceWithKeptChars(chars []rune, attrs []ansi.RuneAttr, data ansi.Astring) ([]rune, []ansi.RuneAttr) {
	if v.keepChars > 0 && !v.wrap {
		shift := min(v.hOffset, v.keepChars)
		if v.keepChars > 0 {
			if shift < len(chars) {
				chars = chars[shift:]
				attrs = attrs[shift:]
			}
			keptChars := make([]rune, shift, shift+len(chars))
			keptAttrs := make([]ansi.RuneAttr, shift, shift+len(chars))
			copy(keptChars, data.Runes[:min(shift, len(data.Runes))])
			copy(keptAttrs, data.Attrs[:min(shift, len(data.Runes))])
			chars = append(keptChars, chars...)
			attrs = append(keptAttrs, attrs...)
			for i := 0; i < v.keepChars && i < len(chars); i++ {
				attr := &attrs[i]
				attr.Fg = ansi.FgColor(ansi.ColorBlue)
				//attr.Bg = ansi.BgColor(ansi.ColorBlue)
				//attr.Style = uint8(ansi.StyleBold)
			}
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
	var highlightStyle termbox.Attribute
	var hlIndices [][]int
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
		hlIndices = [][]int{}
		if len(v.search) != 0 {
			searchFunc, err := getSearchFunc(v.info.searchType, v.search)
			if err == nil {
				hlIndices = IndexAll(searchFunc, chars)
			}
		}
		for i, char := range chars {
			attr = attrs[i]
			bg = termbox.ColorDefault
			fg = termbox.ColorDefault
			highlightStyle = termbox.Attribute(0)
			if len(hlIndices) != 0 && hlChars == 0 {
				if hlIndices[0][0] == i {
					hlChars = hlIndices[0][1] - hlIndices[0][0]
					hlIndices = hlIndices[1:]
				}
			}
			if hlChars != 0 {
				highlightStyle = termbox.AttrReverse
				hlChars--
			}
			if attr.Fg != 0 {
				fg = termbox.Attribute(attr.Fg-30+1) | stylesMap[attr.Style]
			}
			if attr.Bg != 0 {
				bg = termbox.Attribute(attr.Bg - 40 + 1)
			}
			if highlightStyle != termbox.Attribute(0) {
				fg = fg | highlightStyle
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
	v.following = false
	if config.follow && !v.buffer.isFull() {
		v.following = true
	}
	v.draw()
}

func (v *viewer) navigateEnd() {
	v.buffer.reset(Pos{POS_UNKNOWN, v.fetcher.lastOffset()})
	v.navigate(-v.height) //not adding +1 since nothing on screen now
	if config.follow {
		v.following = true
	}
}

func (v *viewer) navigateStart() {
	v.following = false
	v.buffer.reset(Pos{0, 0})
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
		case 'U':
			if ok := v.fetcher.removeLastFilter(); ok {
				v.buffer.refresh()
				v.draw()
			}
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
			v.buffer.refresh()
			v.draw()
		case 'C':
			v.switchFilters()
		case 'K':
			v.focus = &v.info
			v.info.reset(ibModeKeepCharacters)
		case 'j':
			v.navigate(+1)
		case 'k':
			v.navigate(-1)

		}
	} else {
		switch ev.Key {
		case termbox.KeyArrowDown:
			v.navigate(+1)
		case termbox.KeyArrowUp:
			v.navigate(-1)
		case termbox.KeyArrowRight:
			v.navigateRight()
		case termbox.KeyArrowLeft:
			v.navigateLeft()
		case termbox.KeyPgup:
			v.navigate(-v.height)
		case termbox.KeyPgdn:
			v.navigate(+v.height)
		case termbox.KeySpace:
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
	v.height-- // Saving one Line for infobar
	v.info.resize(v.width, v.height)
	v.buffer.window = v.height
	v.draw()
}

type infobarRequest struct {
	str  []rune
	mode infobarMode
}

var requestSearch = make(chan infobarRequest)
var requestRefresh = make(chan struct{})
var requestRefill = make(chan struct{})
var requestStatusUpdate = make(chan LineNo)
var requestKeepCharsChange = make(chan int)

func (v *viewer) termGui() {
	err := termbox.Init()
	if err != nil {
		panic(err)
	}
	defer termbox.Close()
	wg := sync.WaitGroup{}
	ctx, cancel := context.WithCancel(v.ctx)
	defer wg.Wait()
	defer cancel()
	termbox.SetInputMode(termbox.InputEsc)
	termbox.SetOutputMode(termbox.Output256)
	v.info = infobar{
		y:              0,
		width:          0,
		currentLine:    &v.buffer.originalPos,
		totalLines:     0,
		filtersEnabled: &v.fetcher.filtersEnabled,
		keepChars:      &v.keepChars,
		flock:          &v.fetcher.lock,
		searchType:     CaseSensitive,
	}
	v.focus = v
	v.buffer = viewBuffer{
		fetcher: v.fetcher,
	}
	v.resize(termbox.Size())
	if config.follow {
		v.navigateEnd()
	}
	wg.Add(3)
	go func() { v.refreshIfEmpty(ctx); wg.Done() }()
	go func() { v.updateLastLine(ctx); wg.Done() }()
	go func() { v.follow(ctx); wg.Done() }()
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
			case <-requestRefresh:
				v.buffer.refresh()
				v.draw()
			case <-requestRefill: // It is not most efficient solution, it might cause huge amount of redraws
				v.refill()
			case line := <-requestStatusUpdate:
				v.info.totalLines = line + 1
				if v.focus == v {
					v.info.draw()
				}
			case <-time.After(10 * time.Millisecond):
				continue
			case charChange := <-requestKeepCharsChange:
				if v.keepChars+charChange >= 0 {
					v.keepChars = v.keepChars + charChange
				}
				v.draw()
			}
		}
	}

}
func (v *viewer) refill() {
	for {
		result := v.buffer.fill()
		if result.newLines != 0 {
			v.buffer.shift(result.newLines)
			if v.buffer.isFull() {
				v.buffer.shiftToEnd()
			}
			v.draw()
			continue
		}
		if result.lastChanged {
			v.draw()
			continue
		}
		return
	}
}

func (v *viewer) refreshIfEmpty(ctx context.Context) {
	refresh := func() {
		go termbox.Interrupt()
		requestRefresh <- struct{}{}
	}
	delay := 3 * time.Millisecond
	locked := false
loop:
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
			if config.follow {
				break loop
			}
			v.buffer.lock.RLock()
			locked = true
			if len(v.buffer.buffer) >= v.height {
				break loop
			}
			if v.buffer.pos != 0 || v.buffer.resetPos.Offset != 0 {
				break loop
			}
			if len(v.fetcher.filters) != 0 {
				break loop
			}
			v.buffer.lock.RUnlock()
			locked = false
			if config.stdin && config.isStdinRead() {
				refresh()
				break
			}
			delay = time.Duration(min64(int64(4000*time.Millisecond), int64(delay*2)))
			refresh()
		}
	}
	if locked {
		v.buffer.lock.RUnlock()
	}
}

func (f *viewer) updateLastLine(ctx context.Context) {
	delay := 10 * time.Millisecond
	lastLine := Pos{0, 0}
	var dataLine PosLine
loop:
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
			prevLine := lastLine
			dataLine = f.fetcher.advanceLines(lastLine)
			lastLine = dataLine.Pos
			if lastLine != prevLine {
				go termbox.Interrupt()
				select {
				case requestStatusUpdate <- lastLine.Line:
					f.fetcher.updateMap(dataLine)
				case <-ctx.Done():
					return

				}
				delay = 0
			} else if config.stdin && config.isStdinRead() {
				break loop
			} else {
				if delay == 0 {
					delay = 10 * time.Millisecond
				}
				delay = time.Duration(min64(int64(4000*time.Millisecond), int64(delay*2)))
			}
		}
	}
}

func (v *viewer) follow(ctx context.Context) {
	delay := 100 * time.Millisecond
	lastOffset := v.fetcher.lastOffset()
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
			if !config.follow {
				continue
			}
			if v.following {
				prevOffset := lastOffset
				lastOffset = v.fetcher.lastOffset()
				if lastOffset != prevOffset {
					go termbox.Interrupt()
					select {
					case requestRefill <- struct{}{}:
					case <-ctx.Done():
						return

					}

				}

			}
		}
	}

}

func (v *viewer) processInfobarRequest(search infobarRequest) {
	defer logging.Timeit("Got search request")()
	switch search.mode {
	case ibModeFilter:
		v.addFilter(search.str, FilterIntersect)
	case ibModeAppend:
		v.addFilter(search.str, FilterUnion)
	case ibModeExclude:
		v.addFilter(search.str, FilterExclude)
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
		if err != nil || keep < 0 {
			logging.Debug("Err: Keepchar: ", err)
			v.keepChars = 0
		} else {
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
