#!/usr/bin/env python3
"""Sample Python module for testing metadata extraction."""

def greet(name):
    """Print a greeting message."""
    print(f"Hello, {name}!")

def add(a, b):
    """Return the sum of two numbers."""
    return a + b

def subtract(a, b):
    """Return the difference of two numbers."""
    return a - b

def multiply(a, b):
    """Return the product of two numbers."""
    return a * b

def divide(a, b):
    """Return the quotient of two numbers."""
    if b == 0:
        raise ValueError("Division by zero")
    return a / b

if __name__ == "__main__":
    name = "World"
    greet(name)
    print(f"2 + 3 = {add(2, 3)}")
