// backend/internal/pkg/errors/errors.go
package errors

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// ======================================================================
// Error Codes
// ======================================================================

// ErrorCode represents a unique error code.
type ErrorCode string

const (
	// General errors (1000-1999)
	CodeUnknown          ErrorCode = "UNKNOWN"
	CodeInternal         ErrorCode = "INTERNAL_ERROR"
	CodeValidation       ErrorCode = "VALIDATION_ERROR"
	CodeBadRequest       ErrorCode = "BAD_REQUEST"
	CodeUnauthorized     ErrorCode = "UNAUTHORIZED"
	CodeForbidden        ErrorCode = "FORBIDDEN"
	CodeNotFound         ErrorCode = "NOT_FOUND"
	CodeConflict         ErrorCode = "CONFLICT"
	CodeTooManyRequests  ErrorCode = "TOO_MANY_REQUESTS"
	CodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
	CodeTimeout          ErrorCode = "TIMEOUT"
	CodeCanceled         ErrorCode = "CANCELED"

	// Authentication errors (2000-2099)
	CodeInvalidToken     ErrorCode = "INVALID_TOKEN"
	CodeExpiredToken     ErrorCode = "EXPIRED_TOKEN"
	CodeInvalidCredentials ErrorCode = "INVALID_CREDENTIALS"
	CodeAccountLocked    ErrorCode = "ACCOUNT_LOCKED"
	CodeAccountSuspended ErrorCode = "ACCOUNT_SUSPENDED"
	CodeAccountInactive  ErrorCode = "ACCOUNT_INACTIVE"
	CodeEmailNotVerified ErrorCode = "EMAIL_NOT_VERIFIED"

	// User errors (2100-2199)
	CodeUserNotFound     ErrorCode = "USER_NOT_FOUND"
	CodeUserAlreadyExists ErrorCode = "USER_ALREADY_EXISTS"
	CodeDuplicateUsername ErrorCode = "DUPLICATE_USERNAME"
	CodeDuplicateEmail   ErrorCode = "DUPLICATE_EMAIL"
	CodeInvalidUsername  ErrorCode = "INVALID_USERNAME"
	CodeInvalidEmail     ErrorCode = "INVALID_EMAIL"

	// Tweet errors (3000-3099)
	CodeTweetNotFound    ErrorCode = "TWEET_NOT_FOUND"
	CodeTweetDeleted     ErrorCode = "TWEET_DELETED"
	CodeTweetContentTooLong ErrorCode = "TWEET_CONTENT_TOO_LONG"
	CodeTweetEmptyContent ErrorCode = "TWEET_EMPTY_CONTENT"

	// Follow errors (3100-3199)
	CodeAlreadyFollowing ErrorCode = "ALREADY_FOLLOWING"
	CodeNotFollowing     ErrorCode = "NOT_FOLLOWING"
	CodeCannotFollowSelf ErrorCode = "CANNOT_FOLLOW_SELF"

	// Like/Retweet errors (3200-3299)
	CodeAlreadyLiked     ErrorCode = "ALREADY_LIKED"
	CodeAlreadyRetweeted ErrorCode = "ALREADY_RETWEETED"
	CodeAlreadyBookmarked ErrorCode = "ALREADY_BOOKMARKED"

	// Poll errors (3300-3399)
	CodePollNotFound     ErrorCode = "POLL_NOT_FOUND"
	CodePollExpired      ErrorCode = "POLL_EXPIRED"
	CodePollAlreadyVoted ErrorCode = "POLL_ALREADY_VOTED"
	CodeInvalidPollOption ErrorCode = "INVALID_POLL_OPTION"

	// Community errors (4000-4099)
	CodeCommunityNotFound ErrorCode = "COMMUNITY_NOT_FOUND"
	CodeCommunityDeleted ErrorCode = "COMMUNITY_DELETED"
	CodeDuplicateSlug    ErrorCode = "DUPLICATE_SLUG"
	CodeNotMember        ErrorCode = "NOT_MEMBER"
	CodeNotAdmin         ErrorCode = "NOT_ADMIN"
	CodeNotModerator     ErrorCode = "NOT_MODERATOR"
	CodeCommunityPrivate ErrorCode = "COMMUNITY_PRIVATE"
	CodeCommunityFull    ErrorCode = "COMMUNITY_FULL"

	// Message errors (5000-5099)
	CodeMessageNotFound  ErrorCode = "MESSAGE_NOT_FOUND"
	CodeMessageTooLong   ErrorCode = "MESSAGE_TOO_LONG"
	CodeInvalidRecipient ErrorCode = "INVALID_RECIPIENT"

	// Rate limiting errors (6000-6099)
	CodeRateLimitExceeded ErrorCode = "RATE_LIMIT_EXCEEDED"
	CodeIPBlocked        ErrorCode = "IP_BLOCKED"

	// Database errors (7000-7099)
	CodeDatabaseError    ErrorCode = "DATABASE_ERROR"
	CodeDuplicateKey     ErrorCode = "DUPLICATE_KEY"
	CodeForeignKeyError  ErrorCode = "FOREIGN_KEY_ERROR"
	CodeConnectionError  ErrorCode = "CONNECTION_ERROR"

	// External service errors (8000-8099)
	CodeExternalServiceError ErrorCode = "EXTERNAL_SERVICE_ERROR"
	CodeStorageError    ErrorCode = "STORAGE_ERROR"
	CodeEmailSendError  ErrorCode = "EMAIL_SEND_ERROR"
	CodeQueueError      ErrorCode = "QUEUE_ERROR"
)

// ======================================================================
= Error Interface
// ======================================================================

// AppError is the base interface for all application errors.
type AppError interface {
	error
	Code() ErrorCode
	Message() string
	Status() int // HTTP status code
	Details() map[string]interface{}
	Unwrap() error
	WithDetails(details map[string]interface{}) AppError
	WithCause(err error) AppError
}

// ======================================================================
= Error Implementation
// ======================================================================

// appError implements the AppError interface.
type appError struct {
	code    ErrorCode              `json:"code"`
	message string                 `json:"message"`
	status  int                    `json:"status"`
	details map[string]interface{} `json:"details,omitempty"`
	cause   error                  `json:"-"`
	stack   []uintptr              `json:"-"`
}

// New creates a new application error.
func New(code ErrorCode, message string, status int) AppError {
	return &appError{
		code:    code,
		message: message,
		status:  status,
		details: make(map[string]interface{}),
		stack:   captureStack(2),
	}
}

// Error implements the error interface.
func (e *appError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.code, e.message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.code, e.message)
}

// Code returns the error code.
func (e *appError) Code() ErrorCode {
	return e.code
}

// Message returns the error message.
func (e *appError) Message() string {
	return e.message
}

// Status returns the HTTP status code.
func (e *appError) Status() int {
	return e.status
}

// Details returns the error details.
func (e *appError) Details() map[string]interface{} {
	return e.details
}

// Unwrap returns the wrapped error.
func (e *appError) Unwrap() error {
	return e.cause
}

// WithDetails adds details to the error.
func (e *appError) WithDetails(details map[string]interface{}) AppError {
	if e.details == nil {
		e.details = make(map[string]interface{})
	}
	for k, v := range details {
		e.details[k] = v
	}
	return e
}

// WithCause wraps another error as the cause.
func (e *appError) WithCause(err error) AppError {
	e.cause = err
	return e
}

// Stack returns the stack trace.
func (e *appError) Stack() []uintptr {
	return e.stack
}

// StackTrace returns the stack trace as a string.
func (e *appError) StackTrace() string {
	if len(e.stack) == 0 {
		return ""
	}
	frames := runtime.CallersFrames(e.stack)
	var sb strings.Builder
	for {
		frame, more := frames.Next()
		sb.WriteString(fmt.Sprintf("%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line))
		if !more {
			break
		}
	}
	return sb.String()
}

// ======================================================================
= HTTP Status Code Mapping
// ======================================================================

// codeStatusMap maps error codes to HTTP status codes.
var codeStatusMap = map[ErrorCode]int{
	CodeUnknown:            500,
	CodeInternal:           500,
	CodeValidation:         400,
	CodeBadRequest:         400,
	CodeUnauthorized:       401,
	CodeForbidden:          403,
	CodeNotFound:           404,
	CodeConflict:           409,
	CodeTooManyRequests:    429,
	CodeServiceUnavailable: 503,
	CodeTimeout:            504,
	CodeCanceled:           499,
	CodeInvalidToken:       401,
	CodeExpiredToken:       401,
	CodeInvalidCredentials: 401,
	CodeAccountLocked:      403,
	CodeAccountSuspended:   403,
	CodeAccountInactive:    403,
	CodeEmailNotVerified:   403,
	CodeUserNotFound:       404,
	CodeUserAlreadyExists:  409,
	CodeDuplicateUsername:  409,
	CodeDuplicateEmail:     409,
	CodeTweetNotFound:      404,
	CodeTweetDeleted:       404,
	CodeTweetContentTooLong: 400,
	CodeTweetEmptyContent:  400,
	CodeAlreadyFollowing:   409,
	CodeNotFollowing:       400,
	CodeCannotFollowSelf:   400,
	CodeAlreadyLiked:       409,
	CodeAlreadyRetweeted:   409,
	CodeAlreadyBookmarked:  409,
	CodePollNotFound:       404,
	CodePollExpired:        400,
	CodePollAlreadyVoted:   409,
	CodeInvalidPollOption:  400,
	CodeCommunityNotFound:  404,
	CodeCommunityDeleted:   404,
	CodeDuplicateSlug:      409,
	CodeNotMember:          403,
	CodeNotAdmin:           403,
	CodeNotModerator:       403,
	CodeCommunityPrivate:   403,
	CodeCommunityFull:      403,
	CodeMessageNotFound:    404,
	CodeRateLimitExceeded:  429,
	CodeDatabaseError:      500,
	CodeDuplicateKey:       409,
	CodeForeignKeyError:    400,
	CodeConnectionError:    503,
	CodeExternalServiceError: 502,
	CodeStorageError:       500,
	CodeEmailSendError:     500,
	CodeQueueError:         500,
}

// HTTPStatus returns the HTTP status for an error code.
func HTTPStatus(code ErrorCode) int {
	if status, ok := codeStatusMap[code]; ok {
		return status
	}
	return 500
}

// ======================================================================
= Error Creation Helpers
// ======================================================================

// InternalError creates an internal server error.
func InternalError(message string) AppError {
	if message == "" {
		message = "Internal server error"
	}
	return New(CodeInternal, message, 500)
}

// BadRequest creates a bad request error.
func BadRequest(message string) AppError {
	if message == "" {
		message = "Invalid request"
	}
	return New(CodeBadRequest, message, 400)
}

// ValidationError creates a validation error.
func ValidationError(message string) AppError {
	if message == "" {
		message = "Validation failed"
	}
	return New(CodeValidation, message, 400)
}

// NotFound creates a not found error.
func NotFound(message string) AppError {
	if message == "" {
		message = "Resource not found"
	}
	return New(CodeNotFound, message, 404)
}

// Unauthorized creates an unauthorized error.
func Unauthorized(message string) AppError {
	if message == "" {
		message = "Unauthorized"
	}
	return New(CodeUnauthorized, message, 401)
}

// Forbidden creates a forbidden error.
func Forbidden(message string) AppError {
	if message == "" {
		message = "Forbidden"
	}
	return New(CodeForbidden, message, 403)
}

// Conflict creates a conflict error.
func Conflict(message string) AppError {
	if message == "" {
		message = "Resource conflict"
	}
	return New(CodeConflict, message, 409)
}

// RateLimitExceeded creates a rate limit error.
func RateLimitExceeded(message string) AppError {
	if message == "" {
		message = "Rate limit exceeded"
	}
	return New(CodeRateLimitExceeded, message, 429)
}

// ======================================================================
= Error Wrapping
// ======================================================================

// Wrap wraps an error with additional context.
func Wrap(err error, code ErrorCode, message string) AppError {
	if err == nil {
		return nil
	}
	var appErr AppError
	if errors.As(err, &appErr) {
		return appErr.WithCause(err)
	}
	return New(code, message, HTTPStatus(code)).WithCause(err)
}

// Wrapf wraps an error with a formatted message.
func Wrapf(err error, code ErrorCode, format string, args ...interface{}) AppError {
	return Wrap(err, code, fmt.Sprintf(format, args...))
}

// WrapWithStatus wraps an error with a custom status.
func WrapWithStatus(err error, code ErrorCode, status int, message string) AppError {
	if err == nil {
		return nil
	}
	return New(code, message, status).WithCause(err)
}

// ======================================================================
= Error Checking
// ======================================================================

// IsCode checks if an error has a specific code.
func IsCode(err error, code ErrorCode) bool {
	var appErr AppError
	if errors.As(err, &appErr) {
		return appErr.Code() == code
	}
	return false
}

// IsNotFound checks if an error is a not found error.
func IsNotFound(err error) bool {
	return IsCode(err, CodeNotFound)
}

// IsUnauthorized checks if an error is an unauthorized error.
func IsUnauthorized(err error) bool {
	return IsCode(err, CodeUnauthorized)
}

// IsForbidden checks if an error is a forbidden error.
func IsForbidden(err error) bool {
	return IsCode(err, CodeForbidden)
}

// IsConflict checks if an error is a conflict error.
func IsConflict(err error) bool {
	return IsCode(err, CodeConflict)
}

// IsBadRequest checks if an error is a bad request error.
func IsBadRequest(err error) bool {
	return IsCode(err, CodeBadRequest)
}

// IsInternal checks if an error is an internal error.
func IsInternal(err error) bool {
	return IsCode(err, CodeInternal)
}

// ======================================================================
= Error Utilities
// ======================================================================

// ToJSON converts an error to JSON.
func ToJSON(err error) []byte {
	var result map[string]interface{}
	if appErr, ok := err.(AppError); ok {
		result = map[string]interface{}{
			"code":    appErr.Code(),
			"message": appErr.Message(),
			"status":  appErr.Status(),
			"details": appErr.Details(),
		}
	} else if err != nil {
		result = map[string]interface{}{
			"code":    CodeUnknown,
			"message": err.Error(),
			"status":  500,
		}
	} else {
		result = map[string]interface{}{
			"code":    CodeUnknown,
			"message": "unknown error",
			"status":  500,
		}
	}
	data, _ := json.Marshal(result)
	return data
}

// ToMap converts an error to a map.
func ToMap(err error) map[string]interface{} {
	if appErr, ok := err.(AppError); ok {
		return map[string]interface{}{
			"code":    appErr.Code(),
			"message": appErr.Message(),
			"status":  appErr.Status(),
			"details": appErr.Details(),
		}
	}
	if err != nil {
		return map[string]interface{}{
			"code":    CodeUnknown,
			"message": err.Error(),
			"status":  500,
		}
	}
	return map[string]interface{}{
		"code":    CodeUnknown,
		"message": "unknown error",
		"status":  500,
	}
}

// GetCode extracts the error code from an error.
func GetCode(err error) ErrorCode {
	if appErr, ok := err.(AppError); ok {
		return appErr.Code()
	}
	if err != nil {
		return CodeUnknown
	}
	return ""
}

// GetStatus extracts the HTTP status from an error.
func GetStatus(err error) int {
	if appErr, ok := err.(AppError); ok {
		return appErr.Status()
	}
	if err != nil {
		return 500
	}
	return 200
}

// GetMessage extracts the error message.
func GetMessage(err error) string {
	if appErr, ok := err.(AppError); ok {
		return appErr.Message()
	}
	if err != nil {
		return err.Error()
	}
	return ""
}

// ======================================================================
= Error Group
// ======================================================================

// ErrorGroup represents a group of errors.
type ErrorGroup struct {
	errors []error
}

// NewErrorGroup creates a new error group.
func NewErrorGroup() *ErrorGroup {
	return &ErrorGroup{
		errors: []error{},
	}
}

// Add adds an error to the group.
func (g *ErrorGroup) Add(err error) {
	if err != nil {
		g.errors = append(g.errors, err)
	}
}

// Error implements the error interface.
func (g *ErrorGroup) Error() string {
	if len(g.errors) == 0 {
		return ""
	}
	msgs := make([]string, len(g.errors))
	for i, err := range g.errors {
		msgs[i] = err.Error()
	}
	return strings.Join(msgs, "; ")
}

// Errors returns all errors in the group.
func (g *ErrorGroup) Errors() []error {
	return g.errors
}

// HasErrors returns true if there are any errors.
func (g *ErrorGroup) HasErrors() bool {
	return len(g.errors) > 0
}

// Count returns the number of errors.
func (g *ErrorGroup) Count() int {
	return len(g.errors)
}

// ======================================================================
= Stack Capture
// ======================================================================

// captureStack captures the stack trace.
func captureStack(skip int) []uintptr {
	stack := make([]uintptr, 32)
	n := runtime.Callers(skip+1, stack)
	return stack[:n]
}

// ======================================================================
= Test Helpers
// ======================================================================

// TestError creates an error for testing.
func TestError(code ErrorCode, message string) AppError {
	return New(code, message, HTTPStatus(code))
}

// Must panics if there is an error.
func Must(err error) {
	if err != nil {
		panic(err)
	}
}

// IsAnyCode checks if an error matches any of the given codes.
func IsAnyCode(err error, codes ...ErrorCode) bool {
	for _, code := range codes {
		if IsCode(err, code) {
			return true
		}
	}
	return false
}