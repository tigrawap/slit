package ansi

import (
	"bytes"
	"github.com/tigrawap/slit/runes"
	"strconv"
	"strings"
)

type RuneAttr struct {
	Fg    uint8
	Bg    uint8
	Style uint8
}

type Astring struct {
	Runes []rune
	Attrs []RuneAttr
}

type Style uint8

const (
	StyleNormal Style = iota
	StyleBold
)

type Color uint8

const (
	ColorBlack   Color = iota
	ColorRed
	ColorGreen
	ColorYellow
	ColorBlue
	ColorMagenta
	ColorCyan
	ColorGray
)

func FgColor(color Color) uint8 {
	return uint8(color) + 30
}

func BgColor(color Color) uint8 {
	return uint8(color) + 40
}

//Returns new Astring, struct containing bytes converted to runes and ansi attributes per rune
func NewAstring(src []byte) Astring {
	var distance int
	var attr RuneAttr
	var r rune
	//var b byte

	rr := bytes.Runes(src)
	max := len(rr)
	astring := Astring{
		make([]rune, max, max),
		make([]RuneAttr, max, max),
	}
	ri := 0
mainLoop:
	for i := 0; i < len(rr); i++ {
		r = rr[i]
		if r == 27 && i != max-1 { // [27 91] is control sequence
			if rr[i+1] == 91 {
				// TODO: For now ignoring all escape sequences other then styling
				distance = runes.IndexRune(rr[i+2:], 'm')
				if distance > 7 {
					continue // 000;000 is maximal distance, if m is further - something went wrong
				}
				if distance == -1 {
					i = i + 1
					continue
				}
				attr.Fg, attr.Bg, attr.Style = 0, 0, 0
				if distance != 0 {
					data := string(rr[i+2: i+2+distance])
					formats := strings.Split(data, ";")
					for _, format := range formats {
						// TODO: Can be optimized by using bytes directly
						f, err := strconv.Atoi(format)
						if err == nil {
							if f >= 30 && f <= 37 {
								attr.Fg = uint8(f)
							} else if f >= 40 && f <= 47 {
								attr.Bg = uint8(f)
							} else if f != 0 {
								attr.Style = uint8(f)
							}
						} else {
							// If could not parse as string - we are reading it wrong
							// Ignoring control sequence itself, but not ditching characters
							continue mainLoop
						}
					}
				}
				i = i + 2 + distance
				continue
				// Control sequence
			}
		} else if r == 8 { // CTRL+H/Backspace
			if i > 0 && len(rr) > i+1 {
				prevChar := rr[i-1]
				nextChar := rr[i+1]
				i += 1 // Will move 1 char forward to skip next char, we are using it now
				astring.Runes[ri-1] = nextChar
				if prevChar == nextChar {
					astring.Attrs[ri-1].Fg = FgColor(ColorRed)
					astring.Attrs[ri-1].Style = uint8(StyleBold)
				} else if prevChar == '_' {
					astring.Attrs[ri-1].Fg = FgColor(ColorGreen)
					astring.Attrs[ri-1].Style = uint8(StyleBold)
				}
				continue // No need to advance ri, used previous one

			}
		}
		astring.Runes[ri] = r
		astring.Attrs[ri] = attr
		ri++
	}
	astring.Runes = astring.Runes[:ri]
	astring.Attrs = astring.Attrs[:ri]
	return astring
}
