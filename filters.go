package main

import "github.com/tigrawap/slit/runes"

type filterAction uint8

const (
	filterNoaction filterAction = iota
	filterInclude
	filterExclude
)

type filter interface {
	isOk([]rune) filterAction
}

type includeOnlyFilter struct{
	sub []rune
}

func (f *includeOnlyFilter) isOk(r []rune) filterAction{
	if runes.Index(r, f.sub) != -1{
		return filterInclude
	}
	return filterExclude
}
