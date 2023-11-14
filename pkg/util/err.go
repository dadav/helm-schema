package util

type CircularError struct {
	msg string
}

func (e *CircularError) Error() string { return e.msg }
