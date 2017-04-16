package main

import (
	"github.com/nsf/termbox-go"
	"errors"
	"github.com/tigrawap/slit/runes"
	"fmt"
)

const promtLength = 1

type infobarMode uint

const (
	ibModeStatus     infobarMode = iota
	ibModeSearch
	ibModeBackSearch
	ibModeFilter
	ibModeAppend
	ibModeExclude
)

type infobar struct {
	y           int
	width       int
	cx          int //cursor position
	editBuffer  []rune
	mode        infobarMode
	totalLines  *int
	currentLine *int
}

func (v *infobar) moveCursor(direction int) error {
	target := v.cx + direction
	if target < 0 {
		return errors.New("Reached beginning of the line")
	}
	if target > len(v.editBuffer) {
		return errors.New("Reached end of the line")
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

func (v *infobar) statusBar() {
	for i := 0; i < v.width; i++ {
		termbox.SetCell(i, v.y, ' ', termbox.ColorBlack, termbox.ColorBlack)
	}
	str := []rune(fmt.Sprintf("%d/%d", *v.currentLine+1, *v.totalLines))
	for i := 0; i < len(str); i++ {
		termbox.SetCell(v.width-len(str)+i, v.y, str[i], termbox.ColorYellow, termbox.ColorBlack)
	}
}

func (v *infobar) showSearch() {
	v.moveCursorToPosition(v.cx)
	v.syncSearchString()
}

func (v *infobar) draw() {
	switch v.mode {
	case ibModeBackSearch:
		termbox.SetCell(0, v.y, '?', termbox.ColorGreen, termbox.ColorBlack)
		v.showSearch()
	case ibModeSearch:
		termbox.SetCell(0, v.y, '/', termbox.ColorGreen, termbox.ColorBlack)
		v.showSearch()
	case ibModeFilter:
		termbox.SetCell(0, v.y, '&', termbox.ColorGreen, termbox.ColorBlack)
		v.showSearch()
	case ibModeExclude:
		termbox.SetCell(0, v.y, '-', termbox.ColorGreen, termbox.ColorBlack)
		v.showSearch()
	case ibModeAppend:
		termbox.SetCell(0, v.y, '+', termbox.ColorGreen, termbox.ColorBlack)
		v.showSearch()
	case ibModeStatus:
		v.statusBar()
	default:
		panic("Not implemented")
	}
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
				pos += 1
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

func (v *infobar) requestSearch() {
	searchString := append([]rune(nil), v.editBuffer...) // Buffer may be modified by concurrent reset
	searchMode := v.mode
	go func() {
		go func() {
			requestSearch <- searchRequest{searchString, searchMode}
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
			v.requestSearch()
			v.reset(ibModeStatus)
			return ACTION_RESET_FOCUS
		case termbox.KeyArrowLeft:
			v.moveCursor(-1)
		case termbox.KeyArrowRight:
			v.moveCursor(+1)
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

func (v *infobar) setCell(x, y int, ch rune, fg, bg termbox.Attribute) {
	termbox.SetCell(x+promtLength, v.y, ch, fg, bg)
}

func (v *infobar) syncSearchString() {
	for i := 0; i < v.width-promtLength; i++ {
		ch := ' '
		if i < len(v.editBuffer) {
			ch = v.editBuffer[i]
		}
		v.setCell(i, v.y, ch, termbox.ColorYellow, termbox.ColorBlack)
	}
	termbox.Flush()
}
