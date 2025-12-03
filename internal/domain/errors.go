package domain

import "errors"

var (
	// ErrNotFound is returned when the requested metric does not exist.
	ErrNotFound = errors.New("not found")
	// ErrInvalidType indicates an unsupported metric type was supplied.
	ErrInvalidType = errors.New("invalid metric type")
)
