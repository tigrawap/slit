package main

import (
	"github.com/tigrawap/slit/runes"
	"errors"
	"regexp"
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

type SearchFunc func(sub []rune) int
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
		ff = func(str []rune) int {
			return runes.Index(str, sub)
		}
	case RegEx:
		re, err := regexp.Compile(string(sub))
		if err != nil {
			return nil, BadFilterDefinition
		}
		ff = func(str []rune) int {
			ret := re.FindStringIndex(string(str))
			if ret == nil {
				return -1
			} else {
				return ret[0]
			}
		}
	default:
		return nil, BadFilterDefinition
	}

	var af ActionFunc
	switch action {
	case FilterIntersect:
		af = buildIntersectionFunc(ff)
	case FilterUnion:
		af = buildUnionFunc(ff)
	case FilterExclude:
		af = buildExcludeFunc(ff)
	default:
		return nil, BadFilterDefinition
	}

	return &Filter{
		sub:        sub,
		st:         searchType,
		takeAction: af,
	}, nil
}

func buildUnionFunc(searchFunc SearchFunc) ActionFunc {
	return func(str []rune, currentAction filterResult) filterResult {
		if currentAction == filterIncluded {
			return filterIncluded
		}
		if searchFunc(str) != -1 {
			return filterIncluded
		}
		return filterExcluded
	}

}

func buildIntersectionFunc(searchFunc SearchFunc) ActionFunc {
	return func(str []rune, currentAction filterResult) filterResult {
		if currentAction == filterExcluded {
			return filterExcluded
		}
		if searchFunc(str) != -1 {
			return filterIncluded
		}
		return filterExcluded
	}
}

func buildExcludeFunc(searchFunc SearchFunc) ActionFunc {
	return func(str []rune, currentAction filterResult) filterResult {
		if currentAction == filterExcluded {
			return filterExcluded
		}
		if searchFunc(str) != -1 {
			return filterExcluded
		}
		return filterIncluded
	}
}
