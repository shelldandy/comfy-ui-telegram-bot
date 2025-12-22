package errors

import (
	"errors"
)

// UserError represents an error with both technical and user-friendly messages
type UserError struct {
	Err       error
	UserMsg   string
	Retryable bool
}

func (e *UserError) Error() string {
	return e.Err.Error()
}

func (e *UserError) Unwrap() error {
	return e.Err
}

// Predefined errors
var (
	ErrComfyUIUnavailable = &UserError{
		Err:       errors.New("comfyui server unavailable"),
		UserMsg:   "The image generation service is currently unavailable. Please try again later.",
		Retryable: true,
	}

	ErrGenerationTimeout = &UserError{
		Err:       errors.New("generation timeout"),
		UserMsg:   "Image generation took too long and was cancelled. Try a simpler prompt.",
		Retryable: true,
	}

	ErrInvalidWorkflow = &UserError{
		Err:       errors.New("invalid workflow"),
		UserMsg:   "There's a problem with the image generation configuration. Please contact the administrator.",
		Retryable: false,
	}

	ErrPromptRejected = &UserError{
		Err:       errors.New("prompt rejected by comfyui"),
		UserMsg:   "The prompt could not be processed. Please try rewording your request.",
		Retryable: true,
	}

	ErrUnauthorized = &UserError{
		Err:       errors.New("unauthorized user"),
		UserMsg:   "Sorry, you are not authorized to use this bot.",
		Retryable: false,
	}

	ErrGenerationInProgress = &UserError{
		Err:       errors.New("generation already in progress"),
		UserMsg:   "You already have a generation in progress. Please wait for it to complete.",
		Retryable: false,
	}
)

// Wrap wraps a technical error with a user message
func Wrap(err error, userMsg string, retryable bool) *UserError {
	return &UserError{
		Err:       err,
		UserMsg:   userMsg,
		Retryable: retryable,
	}
}

// GetUserMessage extracts user-friendly message from error
func GetUserMessage(err error) string {
	var userErr *UserError
	if errors.As(err, &userErr) {
		return userErr.UserMsg
	}
	// Default message for unexpected errors
	return "An unexpected error occurred. Please try again later."
}

// IsRetryable checks if an error can be retried
func IsRetryable(err error) bool {
	var userErr *UserError
	if errors.As(err, &userErr) {
		return userErr.Retryable
	}
	return false
}
