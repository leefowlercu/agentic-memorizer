// Package sample provides a sample Go file for testing chunkers.
package sample

import "fmt"

// Greeter provides greeting functionality.
type Greeter struct {
	Name string
}

// NewGreeter creates a new Greeter with the given name.
func NewGreeter(name string) *Greeter {
	return &Greeter{Name: name}
}

// Greet returns a greeting message.
func (g *Greeter) Greet() string {
	return fmt.Sprintf("Hello, %s!", g.Name)
}

// SayGoodbye returns a farewell message.
func (g *Greeter) SayGoodbye() string {
	return fmt.Sprintf("Goodbye, %s!", g.Name)
}
