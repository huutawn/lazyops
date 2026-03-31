package control

import "errors"

var (
	ErrBootstrapTokenUnknown     = errors.New("bootstrap token not recognized")
	ErrBootstrapTokenExpired     = errors.New("bootstrap token expired")
	ErrBootstrapTokenReused      = errors.New("bootstrap token already used")
	ErrBootstrapTargetMismatch   = errors.New("bootstrap token target mismatch")
	ErrControlClientClosed       = errors.New("control client closed")
	ErrControlClientNotConnected = errors.New("control client is not connected")
)
