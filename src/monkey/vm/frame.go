package vm

import (
	"monkey/code"
	"monkey/object"
)

type Frame struct {
	cl *object.Closure
	// ip is the instruction pointer in this frame, for this function
	ip          int
	basePointer int
}

func NewFrame(cl *object.Closure, basePointer int) *Frame {
	return &Frame{cl: cl, ip: -1, basePointer: basePointer}
}

func (f *Frame) Instructions() code.Instructions {
	return f.cl.Fn.Instructions
}
