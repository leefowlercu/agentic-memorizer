package events

import "errors"

// ErrBusClosed is returned when attempting to publish to a closed bus.
var ErrBusClosed = errors.New("event bus is closed")
