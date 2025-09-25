package domain

import "errors"

var (
	ErrNotFound    = errors.New("not found")
	ErrInvalidType = errors.New("invalid metric type")
)
