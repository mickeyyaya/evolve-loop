package b

import "example.com/fixture/a"

func Digest() int { return 2 }

func Drive(r a.Runner) int { return r.Run() }

func Apply(f func() int) int { return f() }

func Describe(e error) string { return e.Error() }

func Shut(c interface{ Close() int }) int { return c.Close() }
