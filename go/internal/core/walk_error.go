package core

type WalkError struct {
	CLIs []string
	Err  error
}

func (e WalkError) Error() string { return e.Err.Error() }

func (e WalkError) Unwrap() error { return e.Err }
