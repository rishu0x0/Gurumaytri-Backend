package auth

import (
	"errors"
	"fmt"
)

var (
	ErrDuplicateMobile      = errors.New("duplicate_mobile")
	ErrSessionNotFound      = errors.New("session_not_found")
	ErrSessionAlreadyUsed   = errors.New("session_already_used")
	ErrSessionExpiredOrUsed = errors.New("session_expired_or_used")
	ErrOTPExpired           = errors.New("otp_expired")
	ErrOTPLocked            = errors.New("otp_locked")
	ErrOTPInvalid           = errors.New("otp_invalid")
	ErrInvalidCredentials   = errors.New("invalid_credentials")
	ErrRetryTooSoon         = errors.New("retry_too_soon")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}
