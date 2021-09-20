package vm

import (
	"fmt"
	"monkey/code"
	"monkey/compiler"
	"monkey/object"
)

const GlobalsSize = 65536
const MaxFrames = 1024
const StackSize = 2048

type VM struct {
	constants   []object.Object
	globals     []object.Object
	stack       []object.Object
	sp          int //Always points to the next value. Top of stack is stack[sp-1]
	frames      []*Frame
	framesIndex int
}

var True = &object.Boolean{Value: true}
var False = &object.Boolean{Value: false}
var Null = &object.Null{}

func New(bytecode *compiler.Bytecode) *VM {
	mainFn := &object.CompiledFunction{Instructions: bytecode.Instructions}
	mainFrame := NewFrame(mainFn, 0)
	frames := make([]*Frame, MaxFrames)
	frames[0] = mainFrame
	return &VM{
		globals:     make([]object.Object, GlobalsSize),
		constants:   bytecode.Constans,
		stack:       make([]object.Object, StackSize),
		sp:          0,
		frames:      frames,
		framesIndex: 1,
	}
}

func NewWithGlobalsStore(bytecode *compiler.Bytecode, s []object.Object) *VM {
	vm := New(bytecode)
	vm.globals = s
	return vm
}

func (vm *VM) StackTop() object.Object {
	if vm.sp == 0 {
		return nil
	}
	return vm.stack[vm.sp-1]
}

func (vm *VM) Run() error {
	var ip int
	var ins code.Instructions
	var op code.Opcode
	for vm.currentFrame().ip < len(vm.currentFrame().Instructions())-1 {
		vm.currentFrame().ip++
		ip = vm.currentFrame().ip
		ins = vm.currentFrame().Instructions()
		op = code.Opcode(ins[ip])
		switch op {
		case code.OpArray:
			numElements := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			// e.g. vm.sp =3, numElements = 3
			// startIndex = 3, endIndex=0
			array := vm.buildArray(vm.sp-numElements, vm.sp)
			vm.push(array)
		case code.OpCall:
			args := int(ins[ip+1])
			vm.currentFrame().ip++
			err := vm.callFunction(args)
			if err != nil {
				return err
			}
		case code.OpConstant:
			// Why do we use slice for argument?
			//const index is 2byte, so use ReadUint16(this func read 2byte).
			constIndex := code.ReadUint16(ins[ip+1:])
			// Why do we add 2?
			// vm.instrction[ip] is Opcode, OpConstant is 2 byte, so to point next Opcode we should add 2.
			vm.currentFrame().ip += 2
			err := vm.push(vm.constants[constIndex])
			if err != nil {
				return err
			}
		case code.OpPop:
			vm.pop()
		case code.OpAdd, code.OpDiv, code.OpMul, code.OpSub:
			err := vm.executeBinaryOperation(op)
			if err != nil {
				return err
			}
		case code.OpNull:
			err := vm.push(Null)
			if err != nil {
				return err
			}
		case code.OpTrue:
			err := vm.push(True)
			if err != nil {
				return err
			}
		case code.OpFalse:
			err := vm.push(False)
			if err != nil {
				return err
			}
		case code.OpEqual, code.OpGreaterThan, code.OpNotEqual:
			err := vm.executeComparision(op)
			if err != nil {
				return err
			}
		case code.OpMinus:
			err := vm.executeMinusOperator()
			if err != nil {
				return err
			}
		case code.OpBang:
			err := vm.executeBangOperator()
			if err != nil {
				return err
			}

		case code.OpJump:
			//jump index is 2byte, so use ReadUint16(this func read 2byte).
			// ip point OpJump, to read index next point
			pos := int(code.ReadUint16(ins[ip+1:]))
			// end for loop, ip is added 1, so we have to substruct 1.
			vm.currentFrame().ip = pos - 1
		case code.OpJumpNotTruthy:
			pos := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			condition := vm.pop()
			if !isTruty(condition) {
				vm.currentFrame().ip = pos - 1
			}
		case code.OpGetGlobal:
			//read index of
			globalIndex := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			err := vm.push(vm.globals[globalIndex])
			if err != nil {
				return err
			}
		case code.OpGetLocal:
			localIndex := int(ins[ip+1])
			vm.currentFrame().ip++
			frame := vm.currentFrame()
			err := vm.push(vm.stack[frame.basePointer+localIndex])
			if err != nil {
				return err
			}
		case code.OpReturn:
			frame := vm.popFrame()
			vm.sp = frame.basePointer - 1
			err := vm.push(Null)
			if err != nil {
				return err
			}
		case code.OpReturnValue:
			returnValue := vm.pop()
			frame := vm.popFrame()
			vm.sp = frame.basePointer - 1
			err := vm.push(returnValue)
			if err != nil {
				return err
			}
		case code.OpSetGlobal:
			globalIndex := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			vm.globals[globalIndex] = vm.pop()
		case code.OpSetLocal:
			localIndex := int(ins[ip+1])
			vm.currentFrame().ip++
			frame := vm.currentFrame()
			vm.stack[frame.basePointer+int(localIndex)] = vm.pop()
		case code.OpHash:
			numElements := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			hash, err := vm.buildHash(vm.sp-numElements, vm.sp)
			if err != nil {
				return err
			}
			vm.sp = vm.sp - numElements
			err = vm.push(hash)
			if err != nil {
				return err
			}
		case code.OpIndex:
			index := vm.pop()
			left := vm.pop()
			err := vm.executeIndexExpression(left, index)
			if err != nil {
				return err
			}

		}

	}
	return nil
}

func (vm *VM) buildArray(startIndex, endIndex int) object.Object {
	// e.g. vm.sp = 3, numElements = 3
	// startIndex = 0, endIndex = 3
	// make( ~ , 3)
	elements := make([]object.Object, endIndex-startIndex)
	for i := startIndex; i < endIndex; i++ {
		// we load bottom of stack(i)
		elements[i-startIndex] = vm.stack[i]
	}
	return &object.Array{Elements: elements}
}

func (vm *VM) buildHash(startIndex, endIndex int) (object.Object, error) {
	hashedPairs := make(map[object.HashKey]object.HashPair)
	for i := startIndex; i < endIndex; i += 2 {
		key := vm.stack[i]
		value := vm.stack[i+1]

		pair := object.HashPair{Key: key, Value: value}
		hashKey, ok := key.(object.Hashable)

		if !ok {
			return nil, fmt.Errorf("unusable as hash key %s", key.Type())
		}
		hashedPairs[hashKey.HashKey()] = pair
	}
	return &object.Hash{Pairs: hashedPairs}, nil
}

func (vm *VM) callFunction(numArgs int) error {
	fn, ok := vm.stack[vm.sp-1-numArgs].(*object.CompiledFunction)

	if !ok {
		return fmt.Errorf("calling non-function")
	}

	if fn.NumParameters != numArgs {
		return fmt.Errorf("wrong number of arguments: want=%d, got=%d", fn.NumParameters, numArgs)
	}

	frame := NewFrame(fn, vm.sp-numArgs)
	vm.pushFrame(frame)
	vm.sp = frame.basePointer + fn.NumLocals
	return nil
}

func (vm *VM) currentFrame() *Frame {
	return vm.frames[vm.framesIndex-1]
}

func isTruty(obj object.Object) bool {
	switch obj := obj.(type) {
	case *object.Boolean:
		return obj.Value
	case *object.Null:
		return false
	default:
		// integer 0 is also true.
		return true
	}
}

func (vm *VM) push(o object.Object) error {
	if vm.sp >= StackSize {
		return fmt.Errorf("stack overflow")

	}
	vm.stack[vm.sp] = o
	vm.sp++
	return nil
}

func (vm *VM) pushFrame(f *Frame) {
	vm.frames[vm.framesIndex] = f
	vm.framesIndex++
}

func (vm *VM) pop() object.Object {
	o := vm.stack[vm.sp-1]
	vm.sp--
	return o
}

func (vm *VM) popFrame() *Frame {
	vm.framesIndex--
	return vm.frames[vm.framesIndex]
}

func (vm *VM) LastPoppedStackElm() object.Object {
	return vm.stack[vm.sp]
}

func (vm *VM) executeArrayIndex(left, index object.Object) error {
	indexValue := index.(*object.Integer).Value
	array := left.(*object.Array)
	if indexValue < 0 || int64(len(array.Elements)) <= indexValue {
		return vm.push(Null)
	}
	return vm.push(array.Elements[indexValue])
}

func (vm *VM) executeBangOperator() error {
	operand := vm.pop()
	switch operand {
	case True:
		return vm.push(False)
	case False:
		return vm.push(True)
	case Null:
		return vm.push(True)
	default:
		return vm.push(False)
	}
}

func (vm *VM) executeBinaryOperation(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()
	leftType := left.Type()
	rightType := right.Type()
	if leftType == object.INTEGER_OBJ && rightType == object.INTEGER_OBJ {
		return vm.executeBinaryIntegerOperation(op, left, right)
	}
	if leftType == object.STRING_OBJ && rightType == object.STRING_OBJ {
		return vm.executeBinaryStringOperation(op, left, right)
	}
	return fmt.Errorf("unsupported types for binary operation %s %s", leftType, rightType)
}

func (vm *VM) executeBinaryIntegerOperation(op code.Opcode, left, right object.Object) error {
	leftValue := left.(*object.Integer).Value
	rightValue := right.(*object.Integer).Value
	var result int64
	switch op {
	case code.OpAdd:
		result = leftValue + rightValue
	case code.OpMul:
		result = leftValue * rightValue
	case code.OpDiv:
		result = leftValue / rightValue
	case code.OpSub:
		result = leftValue - rightValue
	default:
		return fmt.Errorf("unknown integer operator%d", op)
	}

	return vm.push(&object.Integer{Value: result})
}

func (vm *VM) executeBinaryStringOperation(op code.Opcode, left, right object.Object) error {
	leftValue := left.(*object.String).Value
	rightValue := right.(*object.String).Value
	if op == code.OpAdd {
		return vm.push(&object.String{Value: leftValue + rightValue})
	}
	return fmt.Errorf("unknown string operator%d", op)
}

func (vm *VM) executeComparision(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()
	if left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ {
		return vm.executeIntegerComparision(op, left, right)
	}

	var result bool
	switch op {
	case code.OpEqual:
		result = left == right
	case code.OpNotEqual:
		result = left != right
	default:
		return fmt.Errorf("unknown bool comparision operator %d", op)
	}
	return vm.push(nativeBoolToBooleanObject(result))
}

func (vm *VM) executeHashIndex(hash, index object.Object) error {
	hashObject := hash.(*object.Hash)
	key, ok := index.(object.Hashable)
	if !ok {
		return fmt.Errorf("unusable as hash key: %s", index.Type())
	}
	pair, ok := hashObject.Pairs[key.HashKey()]
	if !ok {
		return vm.push(Null)
	}
	return vm.push(pair.Value)
}

func (vm *VM) executeIndexExpression(left, index object.Object) error {
	switch {
	case left.Type() == object.ARRAY_OBJ && index.Type() == object.INTEGER_OBJ:
		return vm.executeArrayIndex(left, index)
	case left.Type() == object.HASH_OBJ:
		return vm.executeHashIndex(left, index)
	default:
		return fmt.Errorf("index operator not supported:%s", left.Type())
	}
}

func (vm *VM) executeIntegerComparision(op code.Opcode, left, right object.Object) error {
	leftValue := left.(*object.Integer).Value
	rightValue := right.(*object.Integer).Value
	var result bool
	switch op {
	case code.OpEqual:
		result = leftValue == rightValue
	case code.OpNotEqual:
		result = leftValue != rightValue
	case code.OpGreaterThan:
		result = leftValue > rightValue
	default:
		return fmt.Errorf("unknown integer comparision operator%d", op)
	}
	return vm.push(nativeBoolToBooleanObject(result))
}

func (vm *VM) executeMinusOperator() error {
	operand := vm.pop()
	if operand.Type() != object.INTEGER_OBJ {
		return fmt.Errorf("unsupported types for minus operation %s", operand.Type())
	}
	val := operand.(*object.Integer).Value
	vm.push(&object.Integer{Value: -val})
	return nil
}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return True
	}
	return False
}
