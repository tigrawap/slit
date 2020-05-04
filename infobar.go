package slit

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/nsf/termbox-go"
	"github.com/tigrawap/slit/filters"
	"github.com/tigrawap/slit/logging"
	"github.com/tigrawap/slit/runes"
	"github.com/tigrawap/slit/utils"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

const promtLength = 1

type infobarMode uint

const (
	ibModeStatus infobarMode = iota
	ibModeSearch
	ibModeBackSearch
	ibModeFilter
	ibModeAppend
	ibModeExclude
	ibModeSave
	ibModeMessage
	ibModeKeepCharacters
	ibModeHighlight
)

type infobar struct {
	y              int
	width          int
	cx             int //cursor position
	editBuffer     []rune
	mode           infobarMode
	flock          *sync.RWMutex
	totalLines     LineNo
	currentLine    *Pos
	filtersEnabled *bool
	keepChars      *int
	history        ibHistory
	searchType     filters.SearchType
	message        ibMessage
}

type ibMessage struct {
	str   string
	color termbox.Attribute
}

const ibHistorySize = 1000

type ibHistory struct {
	buffer       [][]rune
	wlock        sync.RWMutex
	pos          int    // position from the end of file. New records appended, so 0 is always "before" last record with ==1 being last record
	currentInput []rune // when navigating from zero position will hold input use entered and displayed once back to zero Line
	loaded       bool
}

func (v *infobar) moveCursor(direction int) error {
	target := v.cx + direction
	if target < 0 {
		return errors.New("Reached beginning of the Line")
	}
	if target > len(v.editBuffer) {
		return errors.New("Reached end of the Line")
	}
	v.moveCursorToPosition(target)
	return nil
}

func (v *infobar) reset(mode infobarMode) {
	v.cx = 0
	v.editBuffer = v.editBuffer[:0]
	v.mode = mode
	v.draw()
}

func (v *infobar) clear() {
	for i := 0; i < v.width; i++ {
		termbox.SetCell(i, v.y, ' ', termbox.ColorDefault, termbox.ColorDefault)
	}
}

func (v *infobar) statusBar() {
	v.clear()
	v.message = ibMessage{}
	v.flock.Lock()
	defer v.flock.Unlock()
	str := []rune(fmt.Sprintf("%s/%d", *v.currentLine, v.totalLines))
	for i := 0; i < len(str); i++ {
		termbox.SetCell(v.width-len(str)+i, v.y, str[i], termbox.ColorYellow, termbox.ColorDefault)
	}
	if !*v.filtersEnabled {
		str := []rune("[-FILTERS]")
		for i := 0; i < len(str) && i+1 < v.width; i++ {
			termbox.SetCell(i+1, v.y, str[i], termbox.ColorMagenta, termbox.ColorDefault)
		}
	}
	termbox.Flush()
}

func (v *infobar) showSearch() {
	v.moveCursorToPosition(v.cx)
	v.syncSearchString()
}

func (v *infobar) draw() {
	switch v.mode {
	case ibModeBackSearch:
		termbox.SetCell(0, v.y, '?', termbox.ColorGreen, termbox.ColorDefault)
		v.showSearch()
	case ibModeSearch:
		termbox.SetCell(0, v.y, '/', termbox.ColorGreen, termbox.ColorDefault)
		v.showSearch()
	case ibModeFilter:
		termbox.SetCell(0, v.y, '&', termbox.ColorGreen, termbox.ColorDefault)
		v.showSearch()
	case ibModeExclude:
		termbox.SetCell(0, v.y, '-', termbox.ColorGreen, termbox.ColorDefault)
		v.showSearch()
	case ibModeHighlight:
		termbox.SetCell(0, v.y, '~', termbox.ColorGreen, termbox.ColorDefault)
		v.showSearch()
	case ibModeSave:
		termbox.SetCell(0, v.y, '>', termbox.ColorMagenta, termbox.ColorDefault)
		v.showSearch()
	case ibModeAppend:
		termbox.SetCell(0, v.y, '+', termbox.ColorGreen, termbox.ColorDefault)
		v.showSearch()
	case ibModeKeepCharacters:
		termbox.SetCell(0, v.y, 'K', termbox.ColorGreen, termbox.ColorDefault)
		v.editBuffer = []rune(strconv.Itoa(*v.keepChars))
		v.showSearch()
		v.moveCursorToPosition(len(v.editBuffer))
	case ibModeStatus:
		v.statusBar()
	case ibModeMessage:
		v.showMessage()
	default:
		panic("Not implemented")
	}
}

func (v *infobar) setInput(str string) {
	v.editBuffer = []rune(str)
	v.showSearch()
	v.moveCursorToPosition(len(v.editBuffer))
}

func (v *infobar) setMessage(message ibMessage) {
	logging.Debug("Setting message", message)
	v.message = message
	v.reset(ibModeMessage)
}

func (v *infobar) showMessage() {
	v.clear()
	logging.Debug("Showing message", v.message)
	str := []rune(v.message.str)
	for i := 0; i < len(str) && i+1 < v.width; i++ {
		logging.Debug("Adding char", str[i])
		termbox.SetCell(i+1, v.y, str[i], v.message.color, termbox.ColorDefault)
	}
	termbox.Flush()
}

func (v *infobar) navigateWord(forward bool) {
	v.moveCursorToPosition(v.findWord(forward))
}

func (v *infobar) findWord(forward bool) (pos int) {
	var addittor int
	var starter int
	if forward {
		addittor = +1
		pos = len(v.editBuffer)
	} else {
		starter = -1
		addittor = -1
		pos = 0
	}
	for i := v.cx + addittor + starter; 0 < i && i < len(v.editBuffer); i += addittor {
		if v.editBuffer[i] == ' ' {
			pos = i
			if !forward {
				pos++
			}
			break
		}
	}
	return
}

func (v *infobar) deleteWord(forward bool) {
	pos := v.findWord(forward)
	var newPos int
	if forward {
		newPos = v.cx
		if pos >= len(v.editBuffer) {
			pos--
		}
		v.editBuffer = append(v.editBuffer[:v.cx], v.editBuffer[pos+1:]...)
	} else {
		newPos = pos
		v.editBuffer = append(v.editBuffer[:pos], v.editBuffer[v.cx:]...)
	}
	v.moveCursorToPosition(newPos)
	v.syncSearchString()
}

func (v *infobar) moveCursorToPosition(pos int) {
	v.cx = pos
	termbox.SetCursor(pos+promtLength, v.y)
	termbox.Flush()
}

func (v *infobar) moveCursorToEnd() {
	v.moveCursorToPosition(len(v.editBuffer))
}

func (v *infobar) requestSearch() {
	searchString := append([]rune(nil), v.editBuffer...) // Buffer may be modified by concurrent reset
	searchMode := v.mode
	go func() {
		go func() {
			requestSearch <- infobarRequest{searchString, searchMode}
		}()
		termbox.Interrupt()
	}()
}

func (v *infobar) resize(width, height int) {
	v.width = width
	v.y = height
}

func (v *infobar) processKey(ev termbox.Event) (a action) {
	if ev.Ch != 0 || ev.Key == termbox.KeySpace {
		ch := ev.Ch
		if ev.Key == termbox.KeySpace {
			ch = ' '
		}
		v.editBuffer = runes.InsertRune(v.editBuffer, ch, v.cx)
		v.moveCursor(+1)
		v.syncSearchString()
	} else {
		switch ev.Key {
		case termbox.KeyEsc:
			logging.Debug("processing esc key")
			switch getEscKey(ev) {
			case ALT_LEFT_ARROW:
				v.navigateWord(false)
			case ALT_RIGHT_ARROW:
				v.navigateWord(true)
			case ALT_BACKSPACE:
				v.deleteWord(false)
			case ALT_D:
				v.deleteWord(true)
			case ESC:
				v.reset(ibModeStatus)
				return ACTION_RESET_FOCUS
			}
		case termbox.KeyEnter:
			v.addToHistory()
			v.requestSearch()
			v.reset(ibModeStatus)
			return ACTION_RESET_FOCUS
		case termbox.KeyArrowLeft:
			v.moveCursor(-1)
		case termbox.KeyArrowRight:
			v.moveCursor(+1)
		case termbox.KeyArrowUp:
			v.onKeyUp()
		case termbox.KeyArrowDown:
			v.onKeyDown()
		case termbox.KeyCtrlSlash:
			v.switchSearchType()
		case termbox.KeyBackspace, termbox.KeyBackspace2:
			err := v.moveCursor(-1)
			if err == nil {
				v.editBuffer = runes.DeleteRune(v.editBuffer, v.cx)
				v.syncSearchString()
			}
		}
	}
	return
}
func (v *infobar) switchSearchType() {
	switch v.mode {
	case ibModeExclude,
		ibModeAppend,
		ibModeSearch,
		ibModeBackSearch,
		ibModeHighlight,
		ibModeFilter:
		st := v.searchType
		nextID := st.ID + 1
		if _, ok := filters.SearchTypeMap[nextID]; !ok {
			nextID = 0
		}
		nextSt := filters.SearchTypeMap[nextID]
		v.searchType = nextSt
		v.draw()
	}
}

func (history *ibHistory) load() {
	if history.loaded {
		return
	}
	history.loaded = true
	f, err := os.Open(config.historyPath)
	if os.IsNotExist(err) {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		history.buffer = append(history.buffer, []rune(scanner.Text()))
	}
}

func (v *infobar) addToHistory() {
	switch v.mode {
	case ibModeKeepCharacters, ibModeSave:
		return
	default:
		v.history.add(v.editBuffer)
	}
}

func (history *ibHistory) add(str []rune) {
	if len(str) == 0 {
		return // no need to save empty strings
	}
	history.load()
	data := make([]rune, len(str))
	copy(data, str)
	history.wlock.Lock()
	history.buffer = append(history.buffer, data)
	history.pos = 0
	history.wlock.Unlock()
	go history.save(str)
}

func (history *ibHistory) save(str []rune) {
	history.wlock.Lock()
	defer history.wlock.Unlock()
	os.MkdirAll(filepath.Dir(config.historyPath), os.ModePerm)
	f, err := os.OpenFile(config.historyPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		logging.Debug(fmt.Sprintf("Could not open history file: %s", err))
		return
	}
	defer f.Close()
	f.Write([]byte(string(str) + "\n"))
	logging.Debug("len, size", len(history.buffer), ibHistorySize)
	if len(history.buffer) >= ibHistorySize {
		history.trim()
	}
}

func (history *ibHistory) trim() {
	tmpPath := config.historyPath + "_tmp"
	tmpFile := utils.OpenRewrite(tmpPath)
	writer := bufio.NewWriter(tmpFile)
	keptHistory := history.buffer[len(history.buffer)-ibHistorySize/100*80:]
	for _, str := range keptHistory {
		writer.WriteString(string(str) + "\n")
	}

	if err := writer.Flush(); err != nil {
		logging.Debug("Could not write temporary history file")
		return
	}
	if err := tmpFile.Close(); err != nil {
		logging.Debug("Could not close temporary history file")
		return
	}
	history.buffer = keptHistory
	os.Rename(tmpPath, config.historyPath)
}

func (v *infobar) onKeyUp() {
	switch v.mode {
	case ibModeKeepCharacters:
		v.changeKeepChars(+1)
	default:
		v.navigateHistory(+1)
	}
}

func (v *infobar) onKeyDown() {
	switch v.mode {
	case ibModeKeepCharacters:
		v.changeKeepChars(-1)
	default:
		v.navigateHistory(-1)
	}
}

func (v *infobar) navigateHistory(i int) {
	v.history.load()
	target := v.history.pos + i
	if len(v.history.buffer) == 0 {
		target = 0
	}
	if target > len(v.history.buffer) {
		target = len(v.history.buffer)
	}
	if target < 0 {
		target = 0
	}
	onPosChange := func() {
		v.moveCursorToEnd()
		v.syncSearchString()
	}
	if target == 0 {
		if v.history.pos != 0 {
			v.history.pos = target
			v.editBuffer = v.history.currentInput
			onPosChange()
		}
		return // Does not matter where we are going, but nothing to do here.
	}
	if v.history.pos == 0 { // Moved out from zero-search to existing search string
		v.history.currentInput = v.editBuffer
	}
	v.history.pos = target
	targetString := v.history.buffer[len(v.history.buffer)-target]
	v.editBuffer = make([]rune, len(targetString))
	copy(v.editBuffer, targetString)
	onPosChange()
}

func (v *infobar) setPromptCell(x, y int, ch rune, fg, bg termbox.Attribute) {
	termbox.SetCell(x+promtLength, v.y, ch, fg, bg)
}

func (v *infobar) syncSearchString() {
	// TODO: Does not handle well very narrow screen
	// TODO: All setCelling here need to be moved to some nicer wrapper funcs
	var color termbox.Attribute
	switch v.mode {
	case ibModeKeepCharacters:
		color = termbox.ColorYellow
	default:
		color = v.searchType.Color
	}
	for i := 0; i < v.width-promtLength; i++ {
		ch := ' '
		if i < len(v.editBuffer) {
			ch = v.editBuffer[i]
		}
		v.setPromptCell(i, v.y, ch, color, termbox.ColorDefault)
	}
	runeName := []rune(v.searchType.Name)
	for i := v.width - len(runeName); i < v.width && i > promtLength; i++ {
		c := i + len(runeName) - v.width
		termbox.SetCell(i, v.y, runeName[c], v.searchType.Color, termbox.ColorDefault)
	}
	termbox.Flush()
}

func (v *infobar) changeKeepChars(direction int) {
	go func() {
		go termbox.Interrupt()
		requestKeepCharsChange <- direction
	}()
}
