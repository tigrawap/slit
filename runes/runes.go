package runes

import (
	"github.com/tigrawap/slit/logging"
)

func InsertRune(runes []rune, r rune, pos int) []rune {
	runes = append(runes, 0)
	copy(runes[pos+1:], runes[pos:])
	runes[pos] = r
	return runes
}

func DeleteRune(runes []rune, pos int) []rune {
	runes = append(runes[:pos], runes[pos+1:]...)
	return runes
}

func Index(runestack, sub []rune) int {
	for i := 0; i < len(runestack); i++ {
		found := true
		if len(runestack[i:]) < len(sub) {
			return -1
		}
		for j := 0; j < len(sub); j++ {
			if runestack[i+j] != sub[j] {
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

func IndexRune(runestack []rune, sub rune) int {
	for i := 0; i < len(runestack); i++ {
		if runestack[i] == sub {
			return i
		}
	}
	return -1
}

func IndexAll(runestack, sub []rune) (indices []int) {
	if len(sub) == 0 {
		return
	}
	var i, ret int
	f := 0
	indices = make([]int, 0, 1)
	for {
		ret = Index(runestack[i:], sub)
		f++
		if f > 100 {
			panic("Too many occurences")
		}
		if ret == -1 {
			break
		} else {
			indices = append(indices, i+ret)
			i = i + ret + len(sub)
			logging.Debug("starting from", i)
		}
	}
	return
}
