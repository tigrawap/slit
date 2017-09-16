package filters

import "fmt"

type UnknownFilterTypeError struct {
	FilterTypeStr string
	Filename      string
}

func (e *UnknownFilterTypeError) Error() string {
	out_msg := fmt.Sprintf("Unknown filter type \"%s\"", e.FilterTypeStr)
	if e.Filename != "" {
		out_msg += fmt.Sprintf(" in \"%s\"", e.Filename)
	}
	return out_msg
}

type FilterTooShortError struct {
	FilterStr string
	Filename  string
}

func (e *FilterTooShortError) Error() string {
	out_msg := fmt.Sprintf("Filter \"%s\" is too short", e.FilterStr)
	if e.Filename != "" {
		out_msg += fmt.Sprintf(" in \"%s\"", e.Filename)
	}
	return out_msg
}
