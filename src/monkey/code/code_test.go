package code

import (
	"testing"
)

func TestMake(t *testing.T) {
	tests := []struct {
		op       Opcode
		operands []int
		expected []byte
	}{
		// Why does third argment have three
		// Because we have 1 byte Opcode and OpConstant has 2 byte operand.
		{OpConstant, []int{65534}, []byte{byte(OpConstant), 255, 254}},
		{OpGetLocal, []int{255}, []byte{byte(OpGetLocal), 255}},
	}

	for _, tt := range tests {
		instruction := Make(tt.op, tt.operands...)
		if len(instruction) != len(tt.expected) {
			t.Errorf("instruction has wrong length. want=%d, got=%d", len(tt.expected), len(instruction))
		}
		for i, b := range tt.expected {
			if instruction[i] != tt.expected[i] {
				t.Errorf("wrong byte at pos %d. want=%d, got=%d", i, b, instruction[i])
			}
		}
	}
}

func TestInstrctionsString(t *testing.T) {
	// instructions := []Instructions{
	// 	Make(OpConstant, 1),
	// 	Make(OpConstant, 2),
	// 	Make(OpConstant, 65535),
	// }
	// expected := "0000 OpConstant 1\n0003 OpConstant 2\n0006 OpConstant 65535\n"
	// 	expected := `0000 OpConstant 1
	// 0003 OpConstant 2
	// 0006 OpConstant 65535
	// `

	instructions := []Instructions{
		Make(OpAdd),
		Make(OpConstant, 2),
		Make(OpConstant, 65535),
		Make(OpGetLocal, 1),
	}
	expected := "0000 OpAdd\n0001 OpConstant 2\n0004 OpConstant 65535\n0007 OpGetLocal 1\n"
	concatted := Instructions{}
	for _, ins := range instructions {
		concatted = append(concatted, ins...)
	}
	if concatted.String() != expected {
		t.Errorf("instructions wrongly formatted. \nwant=%q \ngot=%q", expected, concatted.String())
	}
}

func TestReadOperands(t *testing.T) {
	tests := []struct {
		op        Opcode
		operands  []int
		bytesRead int
	}{
		{OpConstant, []int{65535}, 2},
		{OpGetLocal, []int{255}, 1},
	}
	for _, tt := range tests {
		instruction := Make(tt.op, tt.operands...)
		def, err := Lookup(byte(tt.op))
		if err != nil {
			t.Fatalf("definition not found: %q\n", err)
		}
		operandsRead, n := ReadOperands(def, instruction[1:])
		if n != tt.bytesRead {
			t.Fatalf("n wrong. want=%d, got=%d", tt.bytesRead, n)
		}
		for i, want := range tt.operands {
			if operandsRead[i] != want {
				t.Errorf("operand wrong. want=%d, got=%d", want, operandsRead[i])
			}
		}
	}
}
