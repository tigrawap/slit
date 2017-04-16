package ansi

import (
	"github.com/tigrawap/slit/runes"
	"strings"
	"strconv"
	"bytes"
)

type RuneAttr struct {
	Fg    uint8
	Bg    uint8
	Style uint8
}

type Astring struct {
	Runes []rune
	Attrs []RuneAttr
	//Fg    []uint8
	//Bg    []uint8
	//Style []uint8
}

//Returns new Astring, struct containing bytes converted to runes and ansi attributes per rune
func NewAstring(src []byte) Astring {
	var end int
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
	for i := 0; i < len(rr); i++ {
		r = rr[i]
		if r == 27 && i != max-1 { // [27 91] is control sequence
			if rr[i+1] == 91 {
				// TODO: Should be sequence(regex?-based, m can be something else)
				end = runes.IndexRune(rr[i+2:], 'm')
				if end == -1 {
					i = i + 1
					continue
				}
				attr.Fg, attr.Bg, attr.Style = 0, 0, 0
				if end != 0 {
					data := string(rr[i+2:i+2+end])
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
						}
					}
				}
				i = i + 2 + end
				continue
				// Control sequence
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
