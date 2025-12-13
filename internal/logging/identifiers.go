package logging

import "github.com/google/uuid"

// NewProcessID generates a UUIDv7 for process identification
func NewProcessID() string {
	id, err := uuid.NewV7()
	if err != nil {
		// UUIDv7 generation failure is extremely unlikely (requires time.Now() failure)
		// Fall back to UUIDv4 if it happens
		return uuid.New().String()
	}
	return id.String()
}

// NewSessionID generates a UUIDv7 for session identification
func NewSessionID() string {
	id, err := uuid.NewV7()
	if err != nil {
		// UUIDv7 generation failure is extremely unlikely (requires time.Now() failure)
		// Fall back to UUIDv4 if it happens
		return uuid.New().String()
	}
	return id.String()
}

// NewClientID generates a UUIDv7 for client identification
func NewClientID() string {
	id, err := uuid.NewV7()
	if err != nil {
		// UUIDv7 generation failure is extremely unlikely (requires time.Now() failure)
		// Fall back to UUIDv4 if it happens
		return uuid.New().String()
	}
	return id.String()
}

// IsValidUUIDv7 validates UUIDv7 format (for testing)
func IsValidUUIDv7(id string) bool {
	u, err := uuid.Parse(id)
	if err != nil {
		return false
	}
	return u.Version() == 7
}
