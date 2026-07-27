package server

import "errors"

// Status codes sent from server to client on close
const (
	CodeGenericError       = 4000
	CodeUsernameError      = 4002
	CodeOverloaded         = 4003
	CodeProjectUnavailable = 4004
	CodeSecurity           = 4005
	CodeIdentifyYourself   = 4006
)

// connError carries a close code alongside an error message.
type connError struct {
	code int
	msg  string
}

func (e *connError) Error() string { return e.msg }

func newConnError(code int, msg string) *connError {
	return &connError{code: code, msg: msg}
}

var (
	errRoomFull         = errors.New("room is full")
	errTooManyVariables = errors.New("too many variables in room")
	errTooManyRooms     = errors.New("too many rooms")
	errVariableNotFound = errors.New("variable does not exist")
)
