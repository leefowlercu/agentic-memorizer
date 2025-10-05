package main

import (
	"fmt"
	"os"
)

// main is the entry point of the application
func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: program <name>")
		os.Exit(1)
	}

	name := os.Args[1]
	greet(name)
}

// greet prints a greeting message
func greet(name string) {
	fmt.Printf("Hello, %s!\n", name)
}

// add returns the sum of two integers
func add(a, b int) int {
	return a + b
}

// subtract returns the difference of two integers
func subtract(a, b int) int {
	return a - b
}

// multiply returns the product of two integers
func multiply(a, b int) int {
	return a * b
}

// divide returns the quotient of two integers
func divide(a, b int) (int, error) {
	if b == 0 {
		return 0, fmt.Errorf("division by zero")
	}
	return a / b, nil
}
