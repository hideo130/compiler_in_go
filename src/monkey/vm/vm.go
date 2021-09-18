package vm

import (
	"fmt"
	"monkey/code"
	"monkey/compiler"
	"monkey/object"
)

const GlobalsSize = 65536
const StackSize = 2048

type VM struct {
	constants   []object.Object
	globals     []object.Object
	instrctions code.Instructions
	stack       []object.Object
	sp          int //Always points to the next value. Top of stack is stack[sp-1]
}

var True = &object.Boolean{Value: true}
var False = &object.Boolean{Value: false}
var Null = &object.Null{}

func New(bytecode *compiler.Bytecode) *VM {
	return &VM{
		instrctions: bytecode.Instructions,
		globals:     make([]object.Object, GlobalsSize),
		constants:   bytecode.Constans,
		stack:       make([]object.Object, StackSize),
		sp:          0,
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
	for ip := 0; ip < len(vm.instrctions); ip++ {
		op := code.Opcode(vm.instrctions[ip])
		switch op {
		case code.OpConstant:
			// Why do we use slice for argument?
			//const index is 2byte, so use ReadUint16(this func read 2byte).
			constIndex := code.ReadUint16(vm.instrctions[ip+1:])
			// Why do we add 2?
			// vm.instrction[ip] is Opcode, OpConstant is 2 byte, so to point next Opcode we should add 2.
			ip += 2
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
			pos := int(code.ReadUint16(vm.instrctions[ip+1:]))
			// end for loop, ip is added 1, so we have to substruct 1.
			ip = pos - 1
		case code.OpJumpNotTruthy:
			pos := int(code.ReadUint16(vm.instrctions[ip+1:]))
			ip += 2
			condition := vm.pop()
			if !isTruty(condition) {
				ip = pos - 1
			}
		case code.OpGetGlobal:
			//read index of
			globalIndex := int(code.ReadUint16(vm.instrctions[ip+1:]))
			ip += 2
			err := vm.push(vm.globals[globalIndex])
			if err != nil {
				return err
			}
		case code.OpSetGlobal:
			globalIndex := int(code.ReadUint16(vm.instrctions[ip+1:]))
			ip += 2
			vm.globals[globalIndex] = vm.pop()
		}

	}
	return nil
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

func (vm *VM) pop() object.Object {
	o := vm.stack[vm.sp-1]
	vm.sp--
	return o
}

func (vm *VM) LastPoppedStackElm() object.Object {
	return vm.stack[vm.sp]
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
