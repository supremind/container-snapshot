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
	ErrInvalidImage = errors.New("invlid image name")
	ErrCommit       = errors.New("container commit failed")
	ErrPush         = errors.New("image push failed")
)

func errInvalidImage(msg string) *Error {
	return &Error{
		msg:    msg,
		reason: ErrInvalidImage,
	}
}

func errCommit(msg string) *Error {
	return &Error{
		msg:    msg,
		reason: ErrCommit,
	}
}

func errPush(msg string) *Error {
	return &Error{
		msg:    msg,
		reason: ErrPush,
	}
}
