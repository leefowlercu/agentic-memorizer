"""Sample Python module for testing chunkers."""

from typing import Optional


class Calculator:
    """A simple calculator class."""

    def __init__(self, initial_value: float = 0):
        """Initialize the calculator with an optional value."""
        self.value = initial_value

    def add(self, n: float) -> float:
        """Add a number to the current value."""
        self.value += n
        return self.value

    def subtract(self, n: float) -> float:
        """Subtract a number from the current value."""
        self.value -= n
        return self.value

    def multiply(self, n: float) -> float:
        """Multiply the current value by a number."""
        self.value *= n
        return self.value

    def divide(self, n: float) -> Optional[float]:
        """Divide the current value by a number."""
        if n == 0:
            return None
        self.value /= n
        return self.value

    def reset(self) -> float:
        """Reset the calculator to zero."""
        self.value = 0
        return self.value
