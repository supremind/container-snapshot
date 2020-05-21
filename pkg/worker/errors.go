package worker

import (
	"errors"
	"fmt"
)

type Error struct {
	msg    string
	reason error
}

func (e *Error) Error() string {
	return fmt.Sprint(e.msg, e.reason.Error())
}

func (e *Error) Unwrap() error {
	return e.reason
}

var (
	_errInvalidImage = errors.New("invlid image name")
	_errCommit       = errors.New("container commit failed")
	_errPush         = errors.New("image push failed")
)

func errInvalidImage(msg string) *Error {
	return &Error{
		msg:    msg,
		reason: _errInvalidImage,
	}
}

func errCommit(msg string) *Error {
	return &Error{
		msg:    msg,
		reason: _errCommit,
	}
}

func errPush(msg string) *Error {
	return &Error{
		msg:    msg,
		reason: _errPush,
	}
}
