package vm

import (
	"monkey/code"
	"monkey/object"
)

type Frame struct {
	fn *object.CompiledFunction
	// ip is the instruction pointer in this frame, for this function
	ip int
}

func NewFrame(fn *object.CompiledFunction) *Frame {
	return &Frame{fn: fn, ip: -1}
}

func (f *Frame) Instructions() code.Instructions {
	return f.fn.Instructions
}


