package ansi

import (
	"github.com/tigrawap/slit/runes"
	"strings"
	"strconv"
	"unicode/utf8"
	"bytes"
)

type RuneAttr struct {
	Fg    uint16
	Bg    uint16
	Style uint16
}

// ANSI/Attribute rune, every rune holds attribute information, which may be nil
// Using Arune instead of simple rune allows search/slicing by indices preserving all original information
type Arune struct {
	Rune rune
	Attr RuneAttr
}

type Arunes []Arune

// Returns new slice of Arune, length might be less then original slice since some runes were control sequences,
// that now stored in RuneAttr struct
func NewArune(src []byte) Arunes {
	//defer logging.Timeit("arunes")()
	max := len(src)
	ar := make(Arunes, 0, max)
	//return ar
	var end int
	var attr RuneAttr
	var r rune
	var utfl int
	var b byte

	for i := 0; i < len(src); i++ {
		b = src[i]
		if b == 27 && i != max-1 { // [27 91] is control sequence
			if src[i+1] == 91 {
				// TODO: Should be sequence-based, m can be something else
				end = bytes.IndexByte(src[i+2:], 'm')
				if end == -1 {
					i = i + 1
					continue
				}
				attr.Fg, attr.Bg, attr.Style = 0, 0, 0
				if end != 0 {
					data := string(src[i+2:i+2+end])
					formats := strings.Split(data, ";")
					for _, format := range formats {
						// TODO: Can be optimized by using bytes directly
						f, err := strconv.Atoi(format)
						if err == nil {
							if f >= 30 && f <= 37 {
								attr.Fg = uint16(f)
							} else if f >= 40 && f <= 47 {
								attr.Bg = uint16(f)
							} else if f != 0 {
								attr.Style = uint16(f)
							}
						}
					}
				}
				i = i + 2 + end
				continue
				// Control sequence
			}
		}
		r, utfl = utf8.DecodeRune(src[i:])
		i += utfl - 1
		ar = append(ar, Arune{r, attr})
	}
	return ar
}

// returns slice of Arunes, each element representing slice
// Control sequences from one line might be carried to another line
func NewLines(r []rune) []Arunes {
	return nil // TODO
}

func Uncolorize(r []rune) []rune {
	runes.Index(r, []rune{27, 91})
	return r
}

func Index(arunes Arunes, sub []rune) int {
	for i := 0; i < len(arunes); i++ {
		found := true
		if len(arunes[i:]) < len(sub) {
			return -1
		}
		for j := 0; j < len(sub); j++ {
			if arunes[i+j].Rune != sub[j] {
				found = false
				break
			}
		}
		if found {
			return i
		}
	}
	return -1
}

func IndexAll(arunes Arunes, sub []rune) (indices []int) {
	if len(sub) == 0 {
		return
	}
	var i, ret int
	f := 0
	indices = make([]int, 0, 1)
	for {
		ret = Index(arunes[i:], sub)
		f++
		if f > 100 {
			panic("Too many occurences")
		}
		if ret == -1 {
			break
		} else {
			indices = append(indices, i+ret)
			i = i + ret + len(sub)
		}
	}
	return
}
