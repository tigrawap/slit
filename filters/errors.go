package filters

import "fmt"

type UnknownFilterTypeError struct {
	FilterTypeStr string
	Filename      string
}

func (e *UnknownFilterTypeError) Error() string {
	outMsg := fmt.Sprintf("Unknown filter type \"%s\"", e.FilterTypeStr)
	if e.Filename != "" {
		outMsg += fmt.Sprintf(" in \"%s\"", e.Filename)
	}
	return outMsg
}

type FilterTooShortError struct {
	FilterStr string
	Filename  string
}

func (e *FilterTooShortError) Error() string {
	outMsg := fmt.Sprintf("Filter \"%s\" is too short", e.FilterStr)
	if e.Filename != "" {
		outMsg += fmt.Sprintf(" in \"%s\"", e.Filename)
	}
	return outMsg
}
