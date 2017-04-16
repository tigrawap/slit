package main

import "github.com/tigrawap/slit/runes"

type filterAction uint8

const (
	filterNoaction filterAction = iota
	filterInclude
	filterExclude
)

type filter interface {
	takeAction(str []rune, currentAction filterAction) filterAction
}

type includeFilter struct{
	sub []rune
	append bool
}


type excludeFilter struct{
	sub []rune
}

func (f *includeFilter) takeAction(r []rune, currentAction filterAction) filterAction{
	if currentAction == filterInclude && f.append{
		return filterInclude
	}
	if currentAction == filterExclude && !f.append{
		return filterExclude
	}
	if runes.Index(r, f.sub) != -1{
		return filterInclude
	}
	return filterExclude
}

func (f *excludeFilter) takeAction(r []rune, currentAction filterAction) filterAction{
	if currentAction == filterExclude{
		return filterExclude
	}
	if runes.Index(r, f.sub) != -1{
		return filterExclude
	}
	return filterInclude
}
