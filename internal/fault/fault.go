package fault

import "fmt"

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Exit    int    `json:"-"`
}

func (e *Error) Error() string { return e.Message }
func New(exit int, code, format string, args ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...), Exit: exit}
}
