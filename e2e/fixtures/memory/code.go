package main

import (
	"fmt"
	"log"
)

// ProcessData demonstrates a simple data processing function
func ProcessData(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("input cannot be empty")
	}

	// Simulate processing
	result := fmt.Sprintf("Processed: %s", input)
	return result, nil
}

func main() {
	data := "test data"
	result, err := ProcessData(data)
	if err != nil {
		log.Fatalf("Processing failed: %v", err)
	}

	fmt.Println(result)
}
