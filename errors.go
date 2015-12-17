package airbrake

import "errors"

var (
	ErrQueueFull = errors.New("queue full - event dropped")
)
