// backend/internal/dto/dm_request.go
package dto

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"twitter-clone/backend/internal/domain/entities"
)

// ======================================================================
// Constants
// ======================================================================

const (
	MaxDMContentLength = 5000
	MinDMContentLength = 1
	MaxDMMediaCount    = 10
	MaxDMMessageLimit  = 100
	MinDMMessageLimit  = 1
	DefaultDMLimit     = 20
)

// ======================================================================
// Validation Errors
// ======================================================================

var (
	ErrDMContentRequired     = errors.New("message content is required")
	ErrDMContentTooLong      = fmt.Errorf("message content exceeds maximum length of %d characters", MaxDMContentLength)
	ErrDMContentTooShort     = fmt.Errorf("message content must be at least %d character", MinDMContentLength)
	ErrDMRecipientRequired   = errors.New("recipient ID is required")
	ErrDMSenderRequired      = errors.New("sender ID is required")
	ErrDMSelfMessage         = errors.New("cannot send a message to yourself")
	ErrDMMediaTooMany        = fmt.Errorf("maximum %d media files allowed", MaxDMMediaCount)
	ErrDMMediaInvalid        = errors.New("invalid media URL")
	ErrDMInvalidLimit        = errors.New("limit must be between 1 and 100")
	ErrDMInvalidCursor       = errors.New("invalid pagination cursor")
	ErrDMConversationNotFound = errors.New("conversation not found")
	ErrDMMessageNotFound     = errors.New("message not found")
	ErrDMInvalidMessageID    = errors.New("invalid message ID")
)

// ======================================================================
= Send Message Request
// ======================================================================

// SendMessageRequest represents the request body for sending a message.
type SendMessageRequest struct {
	ReceiverID string   `json:"receiver_id" binding:"required"`
	Content    string   `json:"content" binding:"required"`
	MediaURLs  []string `json:"media_urls"`
	ReplyToID  string   `json:"reply_to_id,omitempty"`
	Forwarded  bool     `json:"forwarded,omitempty"`
	// Metadata for additional context (e.g., custom data)
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Validate performs comprehensive validation.
func (r *SendMessageRequest) Validate() error {
	// Validate recipient
	receiverTrimmed := strings.TrimSpace(r.ReceiverID)
	if receiverTrimmed == "" {
		return ErrDMRecipientRequired
	}
	r.ReceiverID = receiverTrimmed

	// Validate content
	contentTrimmed := strings.TrimSpace(r.Content)
	if contentTrimmed == "" && len(r.MediaURLs) == 0 {
		return ErrDMContentRequired
	}
	if contentTrimmed != "" {
		if len(contentTrimmed) < MinDMContentLength {
			return ErrDMContentTooShort
		}
		if len(contentTrimmed) > MaxDMContentLength {
			return ErrDMContentTooLong
		}
		r.Content = contentTrimmed
	}

	// Validate media URLs
	if len(r.MediaURLs) > MaxDMMediaCount {
		return ErrDMMediaTooMany
	}
	for i, url := range r.MediaURLs {
		url = strings.TrimSpace(url)
		if url == "" {
			return ErrDMMediaInvalid
		}
		if !isValidDMURL(url) {
			return ErrDMMediaInvalid
		}
		r.MediaURLs[i] = url
	}
	// Remove empty URLs
	cleaned := make([]string, 0, len(r.MediaURLs))
	for _, url := range r.MediaURLs {
		if url != "" {
			cleaned = append(cleaned, url)
		}
	}
	r.MediaURLs = cleaned

	// Validate reply_to_id (optional)
	if r.ReplyToID != "" {
		r.ReplyToID = strings.TrimSpace(r.ReplyToID)
	}

	return nil
}

// Sanitize cleans up the request fields.
func (r *SendMessageRequest) Sanitize() {
	r.ReceiverID = strings.TrimSpace(r.ReceiverID)
	r.Content = strings.TrimSpace(r.Content)
	r.ReplyToID = strings.TrimSpace(r.ReplyToID)
	// Clean media URLs
	cleaned := make([]string, 0, len(r.MediaURLs))
	for _, url := range r.MediaURLs {
		url = strings.TrimSpace(url)
		if url != "" {
			cleaned = append(cleaned, url)
		}
	}
	r.MediaURLs = cleaned
}

// ======================================================================
= Get Conversation Request
// ======================================================================

// GetConversationRequest represents the request for getting messages.
type GetConversationRequest struct {
	OtherUserID string `json:"other_user_id" binding:"required"`
	Cursor      string `json:"cursor"`
	Limit       int    `json:"limit"`
}

// Validate performs validation.
func (r *GetConversationRequest) Validate() error {
	if strings.TrimSpace(r.OtherUserID) == "" {
		return ErrDMRecipientRequired
	}
	if r.Limit < 1 || r.Limit > MaxDMMessageLimit {
		if r.Limit == 0 {
			r.Limit = DefaultDMLimit
		} else {
			return ErrDMInvalidLimit
		}
	}
	if r.Cursor != "" {
		r.Cursor = strings.TrimSpace(r.Cursor)
		if !r.isValidCursor() {
			return ErrDMInvalidCursor
		}
	}
	return nil
}

// isValidCursor checks cursor format.
func (r *GetConversationRequest) isValidCursor() bool {
	parts := strings.Split(r.Cursor, "|")
	if len(parts) != 2 {
		return false
	}
	_, err := time.Parse(time.RFC3339Nano, parts[0])
	return err == nil && parts[1] != ""
}

// ======================================================================
= Mark Read Request
// ======================================================================

// MarkReadRequest represents the request for marking messages as read.
type MarkReadRequest struct {
	MessageIDs   []string `json:"message_ids"`
	Conversation string   `json:"conversation,omitempty"` // other user ID
}

// Validate performs validation.
func (r *MarkReadRequest) Validate() error {
	if len(r.MessageIDs) == 0 && r.Conversation == "" {
		return errors.New("either message_ids or conversation is required")
	}
	for _, id := range r.MessageIDs {
		if strings.TrimSpace(id) == "" {
			return ErrDMInvalidMessageID
		}
	}
	if r.Conversation != "" {
		r.Conversation = strings.TrimSpace(r.Conversation)
	}
	return nil
}

// ======================================================================
= Delete Message Request
// ======================================================================

// DeleteMessageRequest represents the request for deleting messages.
type DeleteMessageRequest struct {
	MessageIDs   []string `json:"message_ids" binding:"required"`
	DeleteForAll bool     `json:"delete_for_all"`
}

// Validate performs validation.
func (r *DeleteMessageRequest) Validate() error {
	if len(r.MessageIDs) == 0 {
		return errors.New("message_ids is required")
	}
	for _, id := range r.MessageIDs {
		if strings.TrimSpace(id) == "" {
			return ErrDMInvalidMessageID
		}
	}
	return nil
}

// ======================================================================
= Search Messages Request
// ======================================================================

// SearchMessagesRequest represents the request for searching messages.
type SearchMessagesRequest struct {
	Query   string `json:"query" binding:"required"`
	With    string `json:"with,omitempty"` // filter by user ID
	Cursor  string `json:"cursor"`
	Limit   int    `json:"limit"`
}

// Validate performs validation.
func (r *SearchMessagesRequest) Validate() error {
	queryTrimmed := strings.TrimSpace(r.Query)
	if queryTrimmed == "" {
		return errors.New("search query is required")
	}
	r.Query = queryTrimmed

	if r.Limit < 1 || r.Limit > MaxDMMessageLimit {
		if r.Limit == 0 {
			r.Limit = DefaultDMLimit
		} else {
			return ErrDMInvalidLimit
		}
	}
	if r.Cursor != "" {
		r.Cursor = strings.TrimSpace(r.Cursor)
	}
	if r.With != "" {
		r.With = strings.TrimSpace(r.With)
	}
	return nil
}

// ======================================================================
= Helper Functions
// ======================================================================

// isValidDMURL validates URL format.
func isValidDMURL(url string) bool {
	if len(url) > 2048 {
		return false
	}
	re := regexp.MustCompile(`^(https?://|/)[^\s]+$`)
	return re.MatchString(url)
}

// ======================================================================
= DM Response DTOs
// ======================================================================

// MessageResponse represents a single message in responses.
type MessageResponse struct {
	ID         string              `json:"id"`
	SenderID   string              `json:"sender_id"`
	ReceiverID string              `json:"receiver_id"`
	Content    string              `json:"content"`
	MediaURLs  []string            `json:"media_urls"`
	Read       bool                `json:"read"`
	ReadAt     *time.Time          `json:"read_at,omitempty"`
	ReplyToID  string              `json:"reply_to_id,omitempty"`
	Forwarded  bool                `json:"forwarded"`
	IsEdited   bool                `json:"is_edited"`
	EditedAt   *time.Time          `json:"edited_at,omitempty"`
	CustomData map[string]string   `json:"custom_data,omitempty"`
	CreatedAt  time.Time           `json:"created_at"`
	UpdatedAt  time.Time           `json:"updated_at"`
}

// ConversationResponse represents a conversation in responses.
type ConversationResponse struct {
	OtherUserID         string    `json:"other_user_id"`
	LastMessageID       string    `json:"last_message_id"`
	LastMessageContent  string    `json:"last_message_content"`
	LastMessageAt       time.Time `json:"last_message_at"`
	LastMessageRead     bool      `json:"last_message_read"`
	UnreadCount         int       `json:"unread_count"`
}

// ConversationListResponse represents a list of conversations.
type ConversationListResponse struct {
	Conversations []*ConversationResponse `json:"conversations"`
	TotalCount    int64                   `json:"total_count"`
}

// MessageListResponse represents a list of messages.
type MessageListResponse struct {
	Messages   []*MessageResponse `json:"messages"`
	NextCursor string             `json:"next_cursor"`
	HasMore    bool               `json:"has_more"`
	TotalCount int64              `json:"total_count"`
}

// ======================================================================
= Builder Functions for Responses
// ======================================================================

// ToMessageResponse converts a message entity to a response DTO.
func ToMessageResponse(msg *entities.Message) *MessageResponse {
	return &MessageResponse{
		ID:         msg.ID,
		SenderID:   msg.SenderID,
		ReceiverID: msg.ReceiverID,
		Content:    msg.Content,
		MediaURLs:  msg.MediaURLs,
		Read:       msg.Read,
		ReadAt:     msg.ReadAt,
		ReplyToID:  msg.Metadata.ReplyToID,
		Forwarded:  msg.Metadata.ForwardedFrom != "",
		IsEdited:   msg.Metadata.IsEdited,
		EditedAt:   msg.Metadata.EditedAt,
		CustomData: msg.Metadata.CustomData,
		CreatedAt:  msg.CreatedAt,
		UpdatedAt:  msg.UpdatedAt,
	}
}

// ToMessageResponses converts multiple message entities to response DTOs.
func ToMessageResponses(messages []*entities.Message) []*MessageResponse {
	responses := make([]*MessageResponse, 0, len(messages))
	for _, msg := range messages {
		responses = append(responses, ToMessageResponse(msg))
	}
	return responses
}

// ToConversationResponse converts a conversation to a response DTO.
func ToConversationResponse(conv *entities.Conversation) *ConversationResponse {
	// Note: entities.Conversation may not exist; using the interface type
	return &ConversationResponse{
		OtherUserID:         conv.OtherUserID,
		LastMessageID:       conv.LastMessageID,
		LastMessageContent:  conv.LastMessageContent,
		LastMessageAt:       conv.LastMessageAt,
		LastMessageRead:     conv.LastMessageRead,
		UnreadCount:         conv.UnreadCount,
	}
}

// ======================================================================
= Builder Methods for Testing
// ======================================================================

// NewSendMessageRequest creates a new request with defaults.
func NewSendMessageRequest() *SendMessageRequest {
	return &SendMessageRequest{
		ReceiverID: "user123",
		Content:    "Hello, this is a test message!",
		MediaURLs:  []string{},
	}
}

// WithReceiver sets the receiver ID.
func (r *SendMessageRequest) WithReceiver(receiverID string) *SendMessageRequest {
	r.ReceiverID = receiverID
	return r
}

// WithContent sets the content.
func (r *SendMessageRequest) WithContent(content string) *SendMessageRequest {
	r.Content = content
	return r
}

// WithMedia adds media URLs.
func (r *SendMessageRequest) WithMedia(urls ...string) *SendMessageRequest {
	r.MediaURLs = append(r.MediaURLs, urls...)
	return r
}

// WithReplyTo sets the reply-to ID.
func (r *SendMessageRequest) WithReplyTo(replyToID string) *SendMessageRequest {
	r.ReplyToID = replyToID
	return r
}

// NewGetConversationRequest creates a new request.
func NewGetConversationRequest() *GetConversationRequest {
	return &GetConversationRequest{
		OtherUserID: "user123",
		Limit:       DefaultDMLimit,
	}
}

// NewMarkReadRequest creates a new mark read request.
func NewMarkReadRequest() *MarkReadRequest {
	return &MarkReadRequest{
		MessageIDs: []string{},
	}
}

// WithMessageIDs sets message IDs.
func (r *MarkReadRequest) WithMessageIDs(ids ...string) *MarkReadRequest {
	r.MessageIDs = append(r.MessageIDs, ids...)
	return r
}

// WithConversation sets the conversation identifier.
func (r *MarkReadRequest) WithConversation(otherUserID string) *MarkReadRequest {
	r.Conversation = otherUserID
	return r
}

// NewDeleteMessageRequest creates a new delete request.
func NewDeleteMessageRequest() *DeleteMessageRequest {
	return &DeleteMessageRequest{
		MessageIDs:   []string{},
		DeleteForAll: false,
	}
}

// WithMessageIDs sets message IDs.
func (r *DeleteMessageRequest) WithMessageIDs(ids ...string) *DeleteMessageRequest {
	r.MessageIDs = append(r.MessageIDs, ids...)
	return r
}

// WithDeleteForAll sets delete for all.
func (r *DeleteMessageRequest) WithDeleteForAll(deleteForAll bool) *DeleteMessageRequest {
	r.DeleteForAll = deleteForAll
	return r
}

// NewSearchMessagesRequest creates a new search request.
func NewSearchMessagesRequest() *SearchMessagesRequest {
	return &SearchMessagesRequest{
		Query: "test",
		Limit: DefaultDMLimit,
	}
}

// WithQuery sets the search query.
func (r *SearchMessagesRequest) WithQuery(query string) *SearchMessagesRequest {
	r.Query = query
	return r
}

// WithUser filters by user.
func (r *SearchMessagesRequest) WithUser(userID string) *SearchMessagesRequest {
	r.With = userID
	return r
}

// ======================================================================
= Error Response Helpers
// ======================================================================

// DMError represents a direct message error.
type DMError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// Error implements the error interface.
func (e DMError) Error() string {
	return e.Message
}

// NewDMError creates a new DM error.
func NewDMError(code, message string, details interface{}) *DMError {
	return &DMError{
		Code:    code,
		Message: message,
		Details: details,
	}
}

// ======================================================================
= JSON Custom Marshaling
// ======================================================================

// MarshalJSON implements custom JSON marshaling for SendMessageRequest.
func (r SendMessageRequest) MarshalJSON() ([]byte, error) {
	type Alias SendMessageRequest
	return json.Marshal(&struct {
		Alias
		Content string `json:"content,omitempty"`
	}{
		Alias:   (Alias)(r),
		Content: r.Content,
	})
}

// ======================================================================
= Validation Error Helpers
// ======================================================================

// DMValidationError represents a field validation error.
type DMValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e DMValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// DMValidationErrors is a collection of validation errors.
type DMValidationErrors []DMValidationError

func (ve DMValidationErrors) Error() string {
	messages := make([]string, 0, len(ve))
	for _, err := range ve {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "; ")
}

func (ve DMValidationErrors) ToMap() map[string]string {
	result := make(map[string]string)
	for _, err := range ve {
		result[err.Field] = err.Message
	}
	return result
}

// ======================================================================
= Test Helpers
// ======================================================================

var (
	TestSendDMRequest = NewSendMessageRequest()
	TestGetDMRequest  = NewGetConversationRequest()
	TestMarkDMRead    = NewMarkReadRequest().WithMessageIDs("msg1", "msg2")
)

// MustCreateSendRequest creates a request or panics on error.
func MustCreateSendRequest(receiverID, content string) *SendMessageRequest {
	req := NewSendMessageRequest().
		WithReceiver(receiverID).
		WithContent(content)
	if err := req.Validate(); err != nil {
		panic(err)
	}
	return req
}