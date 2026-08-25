// backend/internal/dto/dm_request.go
package dto

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ======================================================================
// Validation Constants
// ======================================================================

const (
	MaxDMMessageLength    = 5000
	MinDMMessageLength    = 1
	MaxDMMediaCount       = 10
	MaxDMMessageLimit     = 100
	MinDMMessageLimit     = 1
	DefaultDMLimit        = 20
	MaxDMConversationName = 100
)

// ======================================================================
// Common Validation Errors
// ======================================================================

var (
	ErrDMContentRequired     = errors.New("message content is required")
	ErrDMContentTooLong      = fmt.Errorf("message content exceeds maximum length of %d characters", MaxDMMessageLength)
	ErrDMContentTooShort     = fmt.Errorf("message content must be at least %d character", MinDMMessageLength)
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
	ErrDMUserIDRequired      = errors.New("user ID is required")
	ErrDMConversationIDRequired = errors.New("conversation ID is required")
	ErrDMInvalidAction       = errors.New("invalid action")
)

// ======================================================================
// Request DTOs
// ======================================================================

// SendMessageRequest represents the request to send a direct message.
type SendMessageRequest struct {
	ReceiverID string            `json:"receiver_id" binding:"required"`
	Content    string            `json:"content" binding:"required"`
	MediaURLs  []string          `json:"media_urls"`
	ReplyToID  string            `json:"reply_to_id,omitempty"`
	Forwarded  bool              `json:"forwarded,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// Validate validates the send message request.
func (r *SendMessageRequest) Validate() error {
	receiverTrimmed := strings.TrimSpace(r.ReceiverID)
	if receiverTrimmed == "" {
		return ErrDMRecipientRequired
	}
	r.ReceiverID = receiverTrimmed
	contentTrimmed := strings.TrimSpace(r.Content)
	if contentTrimmed == "" && len(r.MediaURLs) == 0 {
		return ErrDMContentRequired
	}
	if contentTrimmed != "" {
		if len(contentTrimmed) < MinDMMessageLength {
			return ErrDMContentTooShort
		}
		if len(contentTrimmed) > MaxDMMessageLength {
			return ErrDMContentTooLong
		}
		r.Content = contentTrimmed
	}
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
	cleaned := make([]string, 0, len(r.MediaURLs))
	for _, url := range r.MediaURLs {
		if url != "" {
			cleaned = append(cleaned, url)
		}
	}
	r.MediaURLs = cleaned
	if r.ReplyToID != "" {
		r.ReplyToID = strings.TrimSpace(r.ReplyToID)
	}
	return nil
}

// Sanitize sanitizes the send message request.
func (r *SendMessageRequest) Sanitize() {
	r.ReceiverID = strings.TrimSpace(r.ReceiverID)
	r.Content = strings.TrimSpace(r.Content)
	r.ReplyToID = strings.TrimSpace(r.ReplyToID)
	cleaned := make([]string, 0, len(r.MediaURLs))
	for _, url := range r.MediaURLs {
		url = strings.TrimSpace(url)
		if url != "" {
			cleaned = append(cleaned, url)
		}
	}
	r.MediaURLs = cleaned
	if r.Metadata == nil {
		r.Metadata = make(map[string]string)
	}
}

// GetConversationRequest represents the request for getting messages.
type GetConversationRequest struct {
	OtherUserID string `json:"other_user_id" binding:"required"`
	Cursor      string `json:"cursor"`
	Limit       int    `json:"limit"`
}

// Validate validates the get conversation request.
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

// Sanitize sanitizes the get conversation request.
func (r *GetConversationRequest) Sanitize() {
	r.OtherUserID = strings.TrimSpace(r.OtherUserID)
	r.Cursor = strings.TrimSpace(r.Cursor)
	if r.Limit < 1 {
		r.Limit = DefaultDMLimit
	}
	if r.Limit > MaxDMMessageLimit {
		r.Limit = MaxDMMessageLimit
	}
}

// MarkReadRequest represents the request for marking messages as read.
type MarkReadRequest struct {
	MessageIDs   []string `json:"message_ids"`
	Conversation string   `json:"conversation,omitempty"` // other user ID
}

// Validate validates the mark read request.
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

// Sanitize sanitizes the mark read request.
func (r *MarkReadRequest) Sanitize() {
	cleaned := make([]string, 0, len(r.MessageIDs))
	for _, id := range r.MessageIDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	r.MessageIDs = cleaned
	r.Conversation = strings.TrimSpace(r.Conversation)
}

// DeleteMessageRequest represents the request for deleting messages.
type DeleteMessageRequest struct {
	MessageIDs   []string `json:"message_ids" binding:"required"`
	DeleteForAll bool     `json:"delete_for_all"`
}

// Validate validates the delete message request.
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

// Sanitize sanitizes the delete message request.
func (r *DeleteMessageRequest) Sanitize() {
	cleaned := make([]string, 0, len(r.MessageIDs))
	for _, id := range r.MessageIDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	r.MessageIDs = cleaned
}

// SearchMessagesRequest represents the request for searching messages.
type SearchMessagesRequest struct {
	Query  string `json:"query" binding:"required"`
	With   string `json:"with,omitempty"` // filter by user ID
	Cursor string `json:"cursor"`
	Limit  int    `json:"limit"`
}

// Validate validates the search messages request.
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

// Sanitize sanitizes the search messages request.
func (r *SearchMessagesRequest) Sanitize() {
	r.Query = strings.TrimSpace(r.Query)
	r.With = strings.TrimSpace(r.With)
	r.Cursor = strings.TrimSpace(r.Cursor)
	if r.Limit < 1 {
		r.Limit = DefaultDMLimit
	}
	if r.Limit > MaxDMMessageLimit {
		r.Limit = MaxDMMessageLimit
	}
}

// ======================================================================
// Helper Function
// ======================================================================

// isValidDMURL validates URL format for DM media.
func isValidDMURL(url string) bool {
	if len(url) > 2048 {
		return false
	}
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

// ======================================================================
// Response DTOs
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

// UnreadCountResponse represents unread count response.
type UnreadCountResponse struct {
	UserID   string `json:"user_id"`
	Unread   int64  `json:"unread"`
	Total    int64  `json:"total"`
}

// DMStatsResponse represents direct message statistics.
type DMStatsResponse struct {
	TotalMessages   int64     `json:"total_messages"`
	TotalSent       int64     `json:"total_sent"`
	TotalReceived   int64     `json:"total_received"`
	UnreadCount     int64     `json:"unread_count"`
	Conversations   int64     `json:"conversations"`
	LastMessageAt   time.Time `json:"last_message_at"`
	FirstMessageAt  time.Time `json:"first_message_at"`
}

// ======================================================================
// Builder Methods for MessageResponse
// ======================================================================

// NewMessageResponse creates a new message response.
func NewMessageResponse() *MessageResponse {
	return &MessageResponse{
		MediaURLs:  []string{},
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
}

// WithID sets the message ID.
func (r *MessageResponse) WithID(id string) *MessageResponse {
	r.ID = id
	return r
}

// WithSenderID sets the sender ID.
func (r *MessageResponse) WithSenderID(senderID string) *MessageResponse {
	r.SenderID = senderID
	return r
}

// WithReceiverID sets the receiver ID.
func (r *MessageResponse) WithReceiverID(receiverID string) *MessageResponse {
	r.ReceiverID = receiverID
	return r
}

// WithContent sets the content.
func (r *MessageResponse) WithContent(content string) *MessageResponse {
	r.Content = content
	return r
}

// WithMediaURLs sets the media URLs.
func (r *MessageResponse) WithMediaURLs(urls ...string) *MessageResponse {
	r.MediaURLs = append(r.MediaURLs, urls...)
	return r
}

// WithRead sets the read status.
func (r *MessageResponse) WithRead(read bool) *MessageResponse {
	r.Read = read
	return r
}

// WithReadAt sets the read at time.
func (r *MessageResponse) WithReadAt(t time.Time) *MessageResponse {
	r.ReadAt = &t
	return r
}

// WithReplyToID sets the reply-to ID.
func (r *MessageResponse) WithReplyToID(id string) *MessageResponse {
	r.ReplyToID = id
	return r
}

// WithForwarded sets the forwarded flag.
func (r *MessageResponse) WithForwarded(forwarded bool) *MessageResponse {
	r.Forwarded = forwarded
	return r
}

// WithEdited sets the edited flag.
func (r *MessageResponse) WithEdited(edited bool) *MessageResponse {
	r.IsEdited = edited
	return r
}

// WithEditedAt sets the edited at time.
func (r *MessageResponse) WithEditedAt(t time.Time) *MessageResponse {
	r.EditedAt = &t
	return r
}

// WithCustomData sets the custom data.
func (r *MessageResponse) WithCustomData(data map[string]string) *MessageResponse {
	r.CustomData = data
	return r
}

// WithCreatedAt sets the creation time.
func (r *MessageResponse) WithCreatedAt(t time.Time) *MessageResponse {
	r.CreatedAt = t
	return r
}

// WithUpdatedAt sets the update time.
func (r *MessageResponse) WithUpdatedAt(t time.Time) *MessageResponse {
	r.UpdatedAt = t
	return r
}

// ======================================================================
// Builder Methods for ConversationResponse
// ======================================================================

// NewConversationResponse creates a new conversation response.
func NewConversationResponse() *ConversationResponse {
	return &ConversationResponse{}
}

// WithOtherUserID sets the other user ID.
func (r *ConversationResponse) WithOtherUserID(id string) *ConversationResponse {
	r.OtherUserID = id
	return r
}

// WithLastMessageID sets the last message ID.
func (r *ConversationResponse) WithLastMessageID(id string) *ConversationResponse {
	r.LastMessageID = id
	return r
}

// WithLastMessageContent sets the last message content.
func (r *ConversationResponse) WithLastMessageContent(content string) *ConversationResponse {
	r.LastMessageContent = content
	return r
}

// WithLastMessageAt sets the last message time.
func (r *ConversationResponse) WithLastMessageAt(t time.Time) *ConversationResponse {
	r.LastMessageAt = t
	return r
}

// WithLastMessageRead sets the last message read status.
func (r *ConversationResponse) WithLastMessageRead(read bool) *ConversationResponse {
	r.LastMessageRead = read
	return r
}

// WithUnreadCount sets the unread count.
func (r *ConversationResponse) WithUnreadCount(count int) *ConversationResponse {
	r.UnreadCount = count
	return r
}

// ======================================================================
// Builder Methods for ConversationListResponse
// ======================================================================

// NewConversationListResponse creates a new conversation list response.
func NewConversationListResponse() *ConversationListResponse {
	return &ConversationListResponse{
		Conversations: []*ConversationResponse{},
	}
}

// Add adds a conversation to the response.
func (r *ConversationListResponse) Add(conv *ConversationResponse) {
	r.Conversations = append(r.Conversations, conv)
}

// WithTotalCount sets the total count.
func (r *ConversationListResponse) WithTotalCount(count int64) *ConversationListResponse {
	r.TotalCount = count
	return r
}

// ======================================================================
// Builder Methods for MessageListResponse
// ======================================================================

// NewMessageListResponse creates a new message list response.
func NewMessageListResponse() *MessageListResponse {
	return &MessageListResponse{
		Messages: []*MessageResponse{},
	}
}

// Add adds a message to the response.
func (r *MessageListResponse) Add(msg *MessageResponse) {
	r.Messages = append(r.Messages, msg)
}

// WithNextCursor sets the next cursor.
func (r *MessageListResponse) WithNextCursor(cursor string) *MessageListResponse {
	r.NextCursor = cursor
	r.HasMore = cursor != ""
	return r
}

// WithTotalCount sets the total count.
func (r *MessageListResponse) WithTotalCount(count int64) *MessageListResponse {
	r.TotalCount = count
	return r
}

// ======================================================================
// Builder Methods for DMStatsResponse
// ======================================================================

// NewDMStatsResponse creates a new DM stats response.
func NewDMStatsResponse() *DMStatsResponse {
	return &DMStatsResponse{}
}

// WithTotalMessages sets the total messages.
func (r *DMStatsResponse) WithTotalMessages(total int64) *DMStatsResponse {
	r.TotalMessages = total
	return r
}

// WithTotalSent sets the total sent.
func (r *DMStatsResponse) WithTotalSent(sent int64) *DMStatsResponse {
	r.TotalSent = sent
	return r
}

// WithTotalReceived sets the total received.
func (r *DMStatsResponse) WithTotalReceived(received int64) *DMStatsResponse {
	r.TotalReceived = received
	return r
}

// WithUnreadCount sets the unread count.
func (r *DMStatsResponse) WithUnreadCount(unread int64) *DMStatsResponse {
	r.UnreadCount = unread
	return r
}

// WithConversations sets the conversations count.
func (r *DMStatsResponse) WithConversations(convs int64) *DMStatsResponse {
	r.Conversations = convs
	return r
}

// WithLastMessageAt sets the last message time.
func (r *DMStatsResponse) WithLastMessageAt(t time.Time) *DMStatsResponse {
	r.LastMessageAt = t
	return r
}

// WithFirstMessageAt sets the first message time.
func (r *DMStatsResponse) WithFirstMessageAt(t time.Time) *DMStatsResponse {
	r.FirstMessageAt = t
	return r
}

// ======================================================================
// Conversion Helpers
// ======================================================================

// ToMessageResponse converts message data to a response.
func ToMessageResponse(id, senderID, receiverID, content string, mediaURLs []string, read bool, readAt, editedAt *time.Time, replyToID string, forwarded, isEdited bool, customData map[string]string, createdAt, updatedAt time.Time) *MessageResponse {
	resp := NewMessageResponse()
	resp.WithID(id).
		WithSenderID(senderID).
		WithReceiverID(receiverID).
		WithContent(content).
		WithMediaURLs(mediaURLs...).
		WithRead(read).
		WithReplyToID(replyToID).
		WithForwarded(forwarded).
		WithEdited(isEdited).
		WithCustomData(customData).
		WithCreatedAt(createdAt).
		WithUpdatedAt(updatedAt)
	if readAt != nil {
		resp.WithReadAt(*readAt)
	}
	if editedAt != nil {
		resp.WithEditedAt(*editedAt)
	}
	return resp
}

// ToConversationResponse converts conversation data to a response.
func ToConversationResponse(otherUserID, lastMessageID, lastMessageContent string, lastMessageAt time.Time, lastMessageRead bool, unreadCount int) *ConversationResponse {
	resp := NewConversationResponse()
	resp.WithOtherUserID(otherUserID).
		WithLastMessageID(lastMessageID).
		WithLastMessageContent(lastMessageContent).
		WithLastMessageAt(lastMessageAt).
		WithLastMessageRead(lastMessageRead).
		WithUnreadCount(unreadCount)
	return resp
}

// ======================================================================
// JSON Serialization
// ======================================================================

// MarshalJSON implements custom JSON marshaling.
func (r *MessageResponse) MarshalJSON() ([]byte, error) {
	type Alias MessageResponse
	return json.Marshal(&struct {
		*Alias
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}{
		Alias:     (*Alias)(r),
		CreatedAt: r.CreatedAt.Format(time.RFC3339),
		UpdatedAt: r.UpdatedAt.Format(time.RFC3339),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (r *MessageResponse) UnmarshalJSON(data []byte) error {
	type Alias MessageResponse
	aux := &struct {
		*Alias
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.CreatedAt != "" {
		t, err := time.Parse(time.RFC3339, aux.CreatedAt)
		if err == nil {
			r.CreatedAt = t
		}
	}
	if aux.UpdatedAt != "" {
		t, err := time.Parse(time.RFC3339, aux.UpdatedAt)
		if err == nil {
			r.UpdatedAt = t
		}
	}
	return nil
}

// ======================================================================
// Test Helpers
// ======================================================================

// NewTestSendMessageRequest creates a test send message request.
func NewTestSendMessageRequest() *SendMessageRequest {
	return &SendMessageRequest{
		ReceiverID: "user2",
		Content:    "Hello there!",
		MediaURLs:  []string{},
	}
}

// NewTestGetConversationRequest creates a test get conversation request.
func NewTestGetConversationRequest() *GetConversationRequest {
	return &GetConversationRequest{
		OtherUserID: "user2",
		Limit:       20,
	}
}

// NewTestMarkReadRequest creates a test mark read request.
func NewTestMarkReadRequest() *MarkReadRequest {
	return &MarkReadRequest{
		MessageIDs: []string{"msg1", "msg2"},
	}
}

// NewTestMessageResponse creates a test message response.
func NewTestMessageResponse() *MessageResponse {
	resp := NewMessageResponse().
		WithID("msg1").
		WithSenderID("user1").
		WithReceiverID("user2").
		WithContent("Hello there!").
		WithRead(false).
		WithForwarded(false).
		WithEdited(false)
	resp.WithCustomData(map[string]string{
		"source": "web",
	})
	return resp
}

// NewTestConversationResponse creates a test conversation response.
func NewTestConversationResponse() *ConversationResponse {
	return NewConversationResponse().
		WithOtherUserID("user2").
		WithLastMessageID("msg1").
		WithLastMessageContent("Hello there!").
		WithLastMessageAt(time.Now().UTC()).
		WithLastMessageRead(false).
		WithUnreadCount(3)
}

// ======================================================================
// API Documentation Constants
// ======================================================================

const (
	APITagDM = "Direct Messages"
)