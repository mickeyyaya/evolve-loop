package a

const Version = 1

var Default = Digest

type Runner interface {
	Run() int
}

type Engine struct{}

func (Engine) Run() int { return Digest() }

func Digest() int { return 1 }

func (e *Engine) Reset() {}
