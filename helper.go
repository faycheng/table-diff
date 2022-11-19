package main

import (
	"errors"
	"os"
)

type Step struct {
	Head int64
	Tail int64
}

func StepsFrom(from, to, step int64) (steps []Step) {
	steps = make([]Step, 0)
	for i := from; i < to; i++ {
		if (i-from)%step == 0 {
			head := i
			tail := head + step
			if tail > to {
				tail = to
			}
			steps = append(steps, Step{Head: head, Tail: tail})
		}
	}
	return steps
}

func fileExist(name string) (bool, error) {
	_, err := os.Stat(name)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}
