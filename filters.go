package main

import (
	"github.com/tigrawap/slit/runes"
	"errors"
)

type filterResult uint8

const (
	filterNoaction filterResult = iota
	filterIncluded
	filterExcluded
)

type filter interface {
	takeAction(str []rune, currentAction filterResult) filterResult
}

type searchType uint8

const (
	CaseSensitive searchType = iota
	RegEx
)

type FilterAction uint8

const (
	FilterIntersect FilterAction = iota
	FilterUnion
	FilterExclude
)

type SearchFunc func(str []rune, sub []rune) int
type ActionFunc func(str []rune, currentAction filterResult) filterResult

type Filter struct {
	sub        []rune
	st         searchType
	action     FilterAction
	takeAction ActionFunc
}

var BadFilterDefinition = errors.New("Bad filter definition")

func NewFilter(sub []rune, action FilterAction, searchType searchType) (*Filter, error) {
	var ff SearchFunc
	switch searchType {
	case CaseSensitive:
		ff = runes.Index
	default:
		return nil, BadFilterDefinition
	}

	var af ActionFunc
	switch action {
	case FilterIntersect:
		af = buildIntersectionFunc(sub, ff)
	case FilterUnion:
		af = buildUnionFunc(sub, ff)
	case FilterExclude:
		af = buildExcludeFunc(sub, ff)
	default:
		return nil, BadFilterDefinition
	}

	return &Filter{
		sub:sub,
		st:searchType,
		takeAction:af,
	}, nil
}

func buildUnionFunc(sub []rune, searchFunc SearchFunc) ActionFunc {
	return func(str []rune, currentAction filterResult) filterResult {
		if currentAction == filterIncluded {
			return filterIncluded
		}
		if searchFunc(str, sub) != -1 {
			return filterIncluded
		}
		return filterExcluded
	}

}

func buildIntersectionFunc(sub []rune, searchFunc SearchFunc) ActionFunc {
	return func(str []rune, currentAction filterResult) filterResult {
		if currentAction == filterExcluded {
			return filterExcluded
		}
		if searchFunc(str, sub) != -1 {
			return filterIncluded
		}
		return filterExcluded
	}
}

func buildExcludeFunc(sub []rune, searchFunc SearchFunc) ActionFunc {
	return func(str []rune, currentAction filterResult) filterResult {
		if currentAction == filterExcluded {
			return filterExcluded
		}
		if searchFunc(str, sub) != -1 {
			return filterExcluded
		}
		return filterIncluded
	}
}
