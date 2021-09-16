package main

import (
	"fmt"
)

var global string

func main() {
	global = "initial state"
	global, err := double()
	if err != nil {
	}
	mistake()
	fmt.Println(global)
}

func double() (string, error) {
	return "double", nil
}
func mistake() {
	global = "mistake"
}
