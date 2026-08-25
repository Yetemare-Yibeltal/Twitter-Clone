// backend/internal/service/dm_service.go
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/adapter"
	"twitter-clone/backend/internal/domain/entities"
	"twitter-clone/backend/internal/dto"
	"twitter-clone/backend/internal/repository/interfaces"
	"twitter-clone/backend/pkg/logger"
)

// ======================================================================
// Constants and Errors
// ======================================================================

const (
	MaxDMMessageLength    = 5000
	MinDMMessageLength    = 1
	MaxDMMediaCount       = 10
	MaxDMMessagesPerBatch = 100
	DefaultDMLimit        = 20
	MaxDMLimit            = 100
)

var (
	ErrDMSenderNotFound    = errors.New("sender not found")
	ErrDMReceiverNotFound  = errors.New("receiver not found")
	ErrDMConversationEmpty = errors.New("conversation is empty")
	ErrDMMessageNotFound   = errors.New("message not found")
	ErrDMPermissionDenied  = errors.New("permission denied")
	ErrDMSelfMessage       = errors.New("cannot send message to yourself")
	ErrDMContentRequired   = errors.New("message content is required")
	ErrDMContentTooLong    = errors.New("message content exceeds maximum length")
	ErrDMMediaTooMany      = errors.New("too many media files")
	ErrDMMediaInvalid      = errors.New("invalid media URL")
	ErrDMSearchQueryEmpty  = errors.New("search query cannot be empty")
	ErrDMUserNotFound      = errors.New("user not found")
	ErrDMMessageAlreadyRead = errors.New("message already read")
	ErrDMInvalidLimit      = errors.New("invalid limit")
	ErrDMInvalidCursor     = errors.New("invalid cursor")
)

// ======================================================================
// DMService Interface
// ======================================================================

// DMService defines the direct message service interface.
type DMService interface {
	// SendMessage sends a message to a recipient.
	SendMessage(ctx context.Context, senderID string, req *dto.SendMessageRequest) (*dto.MessageResponse, error)
	
	// GetConversation retrieves messages between two users.
	GetConversation(ctx context.Context, userID string, req *dto.GetConversationRequest) (*dto.MessageListResponse, error)
	
	// GetConversations retrieves all conversations for a user.
	GetConversations(ctx context.Context, userID string) (*dto.ConversationListResponse, error)
	
	// MarkAsRead marks a message as read.
	MarkAsRead(ctx context.Context, userID string, req *dto.MarkReadRequest) error
	
	// MarkConversationAsRead marks all messages in a conversation as read.
	MarkConversationAsRead(ctx context.Context, userID, otherUserID string) error
	
	// MarkAllAsRead marks all messages for a user as read.
	MarkAllAsRead(ctx context.Context, userID string) error
	
	// DeleteMessage deletes a message (soft delete).
	DeleteMessage(ctx context.Context, userID string, req *dto.DeleteMessageRequest) error
	
	// DeleteConversation deletes all messages in a conversation.
	DeleteConversation(ctx context.Context, userID, otherUserID string) error
	
	// SearchMessages searches messages by content.
	SearchMessages(ctx context.Context, userID string, req *dto.SearchMessagesRequest) (*dto.MessageListResponse, error)
	
	// GetUnreadCount returns the total unread messages for a user.
	GetUnreadCount(ctx context.Context, userID string) (int64, error)
	
	// GetUnreadFromUser returns unread messages from a specific user.
	GetUnreadFromUser(ctx context.Context, userID, senderID string) (int64, error)
	
	// GetMessageStats returns message statistics for a user.
	GetMessageStats(ctx context.Context, userID string) (*dto.DMStatsResponse, error)
	
	// GetConversationSummary returns summary for a conversation.
	GetConversationSummary(ctx context.Context, userID, otherUserID string) (*dto.ConversationResponse, error)
}

// ======================================================================
// dmService Implementation
// ======================================================================

// dmService implements DMService.
type dmService struct {
	messageRepo      interfaces.MessageRepository
	userRepo         interfaces.UserRepository
	notificationRepo interfaces.NotificationRepository
	redisAdapter     adapter.RedisAdapter
	wsHub            *adapter.WebSocketHub
	log              *logrus.Entry
}

// NewDMService creates a new DM service.
func NewDMService(
	messageRepo interfaces.MessageRepository,
	userRepo interfaces.UserRepository,
	notificationRepo interfaces.NotificationRepository,
	redisAdapter adapter.RedisAdapter,
	wsHub *adapter.WebSocketHub,
) DMService {
	return &dmService{
		messageRepo:      messageRepo,
		userRepo:         userRepo,
		notificationRepo: notificationRepo,
		redisAdapter:     redisAdapter,
		wsHub:            wsHub,
		log:              logger.WithField("service", "dm"),
	}
}

// ======================================================================
// Send Message
// ======================================================================

// SendMessage sends a direct message to a recipient.
func (s *dmService) SendMessage(ctx context.Context, senderID string, req *dto.SendMessageRequest) (*dto.MessageResponse, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}
	// Check if sender exists
	sender, err := s.userRepo.GetByID(ctx, senderID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrDMSenderNotFound
		}
		return nil, fmt.Errorf("failed to get sender: %w", err)
	}
	// Check if receiver exists
	receiver, err := s.userRepo.GetByID(ctx, req.ReceiverID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrDMReceiverNotFound
		}
		return nil, fmt.Errorf("failed to get receiver: %w", err)
	}
	// Check self message
	if senderID == req.ReceiverID {
		return nil, ErrDMSelfMessage
	}
	// Create message entity
	msg := &entities.Message{
		ID:         uuid.New().String(),
		SenderID:   senderID,
		ReceiverID: req.ReceiverID,
		Content:    req.Content,
		MediaURLs:  req.MediaURLs,
		Read:       false,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Metadata: entities.MessageMetadata{
			ReplyToID:  req.ReplyToID,
			CustomData: req.Metadata,
		},
	}
	if req.Forwarded {
		msg.Metadata.ForwardedFrom = senderID
	}
	// Save message
	if err := s.messageRepo.Create(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to save message: %w", err)
	}
	// Update last active for both users
	_ = s.userRepo.UpdateLastActive(ctx, senderID)
	_ = s.userRepo.UpdateLastActive(ctx, req.ReceiverID)
	// Invalidate conversation cache for receiver
	_ = s.invalidateConversationCache(ctx, req.ReceiverID)
	_ = s.invalidateConversationCache(ctx, senderID)
	// Create notification
	_ = s.createMessageNotification(ctx, req.ReceiverID, senderID, msg.ID, msg.Content)
	// Send WebSocket notification to receiver
	if s.wsHub != nil {
		s.wsHub.BroadcastToUser(req.ReceiverID, map[string]interface{}{
			"type":    "new_message",
			"message": s.toMessageResponse(msg),
			"sender": map[string]interface{}{
				"id":         sender.ID,
				"username":   sender.Username,
				"full_name":  sender.FullName,
				"avatar_url": sender.AvatarURL,
			},
		})
	}
	s.log.WithFields(logrus.Fields{
		"sender_id":   senderID,
		"receiver_id": req.ReceiverID,
		"message_id":  msg.ID,
	}).Info("Message sent")
	return s.toMessageResponse(msg), nil
}

// ======================================================================
// Get Conversation
// ======================================================================

// GetConversation retrieves messages between two users.
func (s *dmService) GetConversation(ctx context.Context, userID string, req *dto.GetConversationRequest) (*dto.MessageListResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	// Check if other user exists
	_, err := s.userRepo.GetByID(ctx, req.OtherUserID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrDMUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	// Check cache first (for recent conversations)
	cacheKey := fmt.Sprintf("conversation:%s:%s:%s:%d", userID, req.OtherUserID, req.Cursor, req.Limit)
	if s.redisAdapter != nil {
		var cached dto.MessageListResponse
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			s.log.WithField("user_id", userID).Debug("Conversation served from cache")
			return &cached, nil
		}
	}
	// Get messages from repository
	messages, nextCursor, err := s.messageRepo.GetConversation(ctx, userID, req.OtherUserID, req.Cursor, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}
	// Get total count
	totalCount, err := s.messageRepo.CountTotalMessages(ctx, userID)
	if err != nil {
		totalCount = int64(len(messages))
	}
	// Build response
	responses := make([]*dto.MessageResponse, 0, len(messages))
	for _, msg := range messages {
		responses = append(responses, s.toMessageResponse(msg))
	}
	response := &dto.MessageListResponse{
		Messages:   responses,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
		TotalCount: totalCount,
	}
	// Cache for 30 seconds
	if s.redisAdapter != nil {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, response, 30*time.Second)
	}
	return response, nil
}

// ======================================================================
// Get Conversations
// ======================================================================

// GetConversations retrieves all conversations for a user.
func (s *dmService) GetConversations(ctx context.Context, userID string) (*dto.ConversationListResponse, error) {
	// Check cache
	cacheKey := fmt.Sprintf("conversations:%s", userID)
	if s.redisAdapter != nil {
		var cached dto.ConversationListResponse
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &cached); err == nil {
			s.log.WithField("user_id", userID).Debug("Conversations served from cache")
			return &cached, nil
		}
	}
	// Get conversations from repository
	conversations, err := s.messageRepo.GetConversations(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversations: %w", err)
	}
	// Get total count
	totalCount, err := s.messageRepo.CountTotalConversations(ctx, userID)
	if err != nil {
		totalCount = int64(len(conversations))
	}
	// Build response
	convResponses := make([]*dto.ConversationResponse, 0, len(conversations))
	for _, conv := range conversations {
		// Get user info for other user
		otherUser, err := s.userRepo.GetByID(ctx, conv.OtherUserID)
		if err != nil {
			otherUser = nil
		}
		resp := &dto.ConversationResponse{
			OtherUserID:         conv.OtherUserID,
			LastMessageID:       conv.LastMessageID,
			LastMessageContent:  conv.LastMessageContent,
			LastMessageAt:       conv.LastMessageAt,
			LastMessageRead:     conv.LastMessageRead,
			UnreadCount:         conv.UnreadCount,
		}
		if otherUser != nil {
			resp.OtherUsername = otherUser.Username
			resp.OtherFullName = otherUser.FullName
			resp.OtherAvatarURL = otherUser.AvatarURL
		}
		convResponses = append(convResponses, resp)
	}
	response := &dto.ConversationListResponse{
		Conversations: convResponses,
		TotalCount:    totalCount,
	}
	// Cache for 30 seconds
	if s.redisAdapter != nil {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, response, 30*time.Second)
	}
	return response, nil
}

// ======================================================================
// Read Status Operations
// ======================================================================

// MarkAsRead marks a message as read.
func (s *dmService) MarkAsRead(ctx context.Context, userID string, req *dto.MarkReadRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}
	// Mark individual messages
	if len(req.MessageIDs) > 0 {
		for _, id := range req.MessageIDs {
			msg, err := s.messageRepo.GetByID(ctx, id)
			if err != nil {
				if errors.Is(err, interfaces.ErrMessageNotFound) {
					continue
				}
				return fmt.Errorf("failed to get message: %w", err)
			}
			// Verify receiver is the user
			if msg.ReceiverID != userID {
				continue // skip messages not for this user
			}
			if msg.Read {
				continue
			}
			if err := s.messageRepo.MarkAsRead(ctx, id); err != nil {
				s.log.WithError(err).WithField("message_id", id).Warn("Failed to mark message as read")
			}
		}
	}
	// Mark conversation as read
	if req.Conversation != "" {
		if err := s.messageRepo.MarkConversationAsRead(ctx, userID, req.Conversation); err != nil {
			return fmt.Errorf("failed to mark conversation as read: %w", err)
		}
		// Send WebSocket notification
		if s.wsHub != nil {
			s.wsHub.BroadcastToUser(req.Conversation, map[string]interface{}{
				"type":      "read_receipt",
				"reader_id": userID,
				"timestamp": time.Now().Unix(),
			})
		}
		_ = s.invalidateConversationCache(ctx, userID)
	}
	return nil
}

// MarkConversationAsRead marks all messages in a conversation as read.
func (s *dmService) MarkConversationAsRead(ctx context.Context, userID, otherUserID string) error {
	if userID == "" || otherUserID == "" {
		return errors.New("user IDs are required")
	}
	if err := s.messageRepo.MarkConversationAsRead(ctx, userID, otherUserID); err != nil {
		return fmt.Errorf("failed to mark conversation as read: %w", err)
	}
	_ = s.invalidateConversationCache(ctx, userID)
	_ = s.invalidateConversationCache(ctx, otherUserID)
	return nil
}

// MarkAllAsRead marks all messages for a user as read.
func (s *dmService) MarkAllAsRead(ctx context.Context, userID string) error {
	if userID == "" {
		return errors.New("user ID is required")
	}
	if err := s.messageRepo.MarkAllAsRead(ctx, userID); err != nil {
		return fmt.Errorf("failed to mark all messages as read: %w", err)
	}
	_ = s.invalidateConversationCache(ctx, userID)
	return nil
}

// ======================================================================
// Delete Operations
// ======================================================================

// DeleteMessage deletes a message (soft delete).
func (s *dmService) DeleteMessage(ctx context.Context, userID string, req *dto.DeleteMessageRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}
	for _, id := range req.MessageIDs {
		msg, err := s.messageRepo.GetByID(ctx, id)
		if err != nil {
			if errors.Is(err, interfaces.ErrMessageNotFound) {
				continue
			}
			return fmt.Errorf("failed to get message: %w", err)
		}
		// Verify user is participant
		if msg.SenderID != userID && msg.ReceiverID != userID {
			return ErrDMPermissionDenied
		}
		// If delete for all, only sender can do it
		if req.DeleteForAll && msg.SenderID != userID {
			return errors.New("only sender can delete for all")
		}
		// If delete for all, hard delete
		if req.DeleteForAll {
			if err := s.messageRepo.HardDelete(ctx, id); err != nil {
				s.log.WithError(err).WithField("message_id", id).Warn("Failed to hard delete message")
			}
		} else {
			// For single user deletion, soft delete
			if err := s.messageRepo.SoftDelete(ctx, id); err != nil {
				s.log.WithError(err).WithField("message_id", id).Warn("Failed to soft delete message")
			}
		}
	}
	_ = s.invalidateConversationCache(ctx, userID)
	return nil
}

// DeleteConversation deletes all messages in a conversation.
func (s *dmService) DeleteConversation(ctx context.Context, userID, otherUserID string) error {
	if userID == "" || otherUserID == "" {
		return errors.New("user IDs are required")
	}
	// Verify other user exists
	_, err := s.userRepo.GetByID(ctx, otherUserID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return ErrDMUserNotFound
		}
		return fmt.Errorf("failed to get user: %w", err)
	}
	if err := s.messageRepo.BulkDeleteConversation(ctx, userID, otherUserID); err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}
	_ = s.invalidateConversationCache(ctx, userID)
	return nil
}

// ======================================================================
// Search Messages
// ======================================================================

// SearchMessages searches messages by content.
func (s *dmService) SearchMessages(ctx context.Context, userID string, req *dto.SearchMessagesRequest) (*dto.MessageListResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	// Search in messages
	messages, nextCursor, err := s.messageRepo.SearchMessages(ctx, userID, req.Query, req.Cursor, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search messages: %w", err)
	}
	// If filtering by user, filter results
	if req.With != "" {
		filtered := make([]*entities.Message, 0, len(messages))
		for _, msg := range messages {
			if msg.SenderID == req.With || msg.ReceiverID == req.With {
				filtered = append(filtered, msg)
			}
		}
		messages = filtered
	}
	// Build responses
	responses := make([]*dto.MessageResponse, 0, len(messages))
	for _, msg := range messages {
		responses = append(responses, s.toMessageResponse(msg))
	}
	totalCount := int64(len(responses))
	response := &dto.MessageListResponse{
		Messages:   responses,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
		TotalCount: totalCount,
	}
	return response, nil
}

// ======================================================================
// Unread Count Operations
// ======================================================================

// GetUnreadCount returns the total unread messages for a user.
func (s *dmService) GetUnreadCount(ctx context.Context, userID string) (int64, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("dm_unread:%s", userID)
	if s.redisAdapter != nil {
		var count int64
		if err := s.redisAdapter.GetJSON(ctx, cacheKey, &count); err == nil {
			return count, nil
		}
	}
	count, err := s.messageRepo.CountUnread(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}
	// Cache for 10 seconds
	if s.redisAdapter != nil {
		_ = s.redisAdapter.CacheSet(ctx, cacheKey, count, 10*time.Second)
	}
	return count, nil
}

// GetUnreadFromUser returns unread messages from a specific user.
func (s *dmService) GetUnreadFromUser(ctx context.Context, userID, senderID string) (int64, error) {
	return s.messageRepo.CountUnreadFromUser(ctx, userID, senderID)
}

// ======================================================================
= Conversation Summary
// ======================================================================

// GetConversationSummary returns summary for a conversation.
func (s *dmService) GetConversationSummary(ctx context.Context, userID, otherUserID string) (*dto.ConversationResponse, error) {
	conv, err := s.messageRepo.GetConversationSummary(ctx, userID, otherUserID)
	if err != nil {
		if errors.Is(err, interfaces.ErrConversationNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get conversation summary: %w", err)
	}
	if conv == nil {
		return nil, nil
	}
	// Get user info
	otherUser, err := s.userRepo.GetByID(ctx, conv.OtherUserID)
	if err != nil {
		otherUser = nil
	}
	resp := &dto.ConversationResponse{
		OtherUserID:        conv.OtherUserID,
		LastMessageID:      conv.LastMessageID,
		LastMessageContent: conv.LastMessageContent,
		LastMessageAt:      conv.LastMessageAt,
		LastMessageRead:    conv.LastMessageRead,
		UnreadCount:        conv.UnreadCount,
	}
	if otherUser != nil {
		resp.OtherUsername = otherUser.Username
		resp.OtherFullName = otherUser.FullName
		resp.OtherAvatarURL = otherUser.AvatarURL
	}
	return resp, nil
}

// ======================================================================
// Stats and Analytics
// ======================================================================

// GetMessageStats returns message statistics for a user.
func (s *dmService) GetMessageStats(ctx context.Context, userID string) (*dto.DMStatsResponse, error) {
	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, ErrDMUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	// Get stats
	totalMessages, err := s.messageRepo.CountTotalMessages(ctx, userID)
	if err != nil {
		totalMessages = 0
	}
	unreadCount, err := s.messageRepo.CountUnread(ctx, userID)
	if err != nil {
		unreadCount = 0
	}
	conversations, err := s.messageRepo.CountTotalConversations(ctx, userID)
	if err != nil {
		conversations = 0
	}
	// Get last message
	latest, err := s.messageRepo.GetLatestMessages(ctx, userID, 1)
	if err != nil || len(latest) == 0 {
		latest = []*entities.Message{}
	}
	// Get first message
	// For simplicity, we don't have a method for oldest, approximate with total count
	return &dto.DMStatsResponse{
		TotalMessages:  totalMessages,
		UnreadCount:    unreadCount,
		Conversations:  conversations,
		LastMessageAt: func() time.Time {
			if len(latest) > 0 {
				return latest[0].CreatedAt
			}
			return time.Time{}
		}(),
	}, nil
}

// ======================================================================
// Helper Methods
// ======================================================================

// toMessageResponse converts a message entity to response DTO.
func (s *dmService) toMessageResponse(msg *entities.Message) *dto.MessageResponse {
	return &dto.MessageResponse{
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

// createMessageNotification creates a notification for a new message.
func (s *dmService) createMessageNotification(ctx context.Context, userID, fromUserID, messageID, content string) error {
	// Truncate content for notification
	preview := content
	if len(preview) > 50 {
		preview = preview[:50] + "..."
	}
	notification := &entities.Notification{
		ID:          uuid.New().String(),
		UserID:      userID,
		FromUserID:  fromUserID,
		Type:        "message",
		ReferenceID: messageID,
		Read:        false,
		CreatedAt:   time.Now(),
		Metadata: entities.NotificationMetadata{
			Summary: preview,
		},
	}
	return s.notificationRepo.Create(ctx, notification)
}

// invalidateConversationCache invalidates conversation cache for a user.
func (s *dmService) invalidateConversationCache(ctx context.Context, userID string) error {
	if s.redisAdapter == nil {
		return nil
	}
	// Delete all conversation cache entries for this user
	pattern := fmt.Sprintf("conversation:%s:*", userID)
	iter := s.redisAdapter.Scan(ctx, 0, pattern, 100)
	var keys []string
	for {
		keysBatch, nextCursor, err := iter.Next()
		if err != nil {
			break
		}
		keys = append(keys, keysBatch...)
		if nextCursor == 0 {
			break
		}
	}
	if len(keys) > 0 {
		if err := s.redisAdapter.Delete(ctx, keys...); err != nil {
			return err
		}
	}
	// Also invalidate conversations list
	convKey := fmt.Sprintf("conversations:%s", userID)
	if err := s.redisAdapter.Delete(ctx, convKey); err != nil {
		s.log.WithError(err).Warn("Failed to invalidate conversations cache")
	}
	// Invalidate unread count
	unreadKey := fmt.Sprintf("dm_unread:%s", userID)
	if err := s.redisAdapter.Delete(ctx, unreadKey); err != nil {
		s.log.WithError(err).Warn("Failed to invalidate unread cache")
	}
	return nil
}

// ======================================================================
// Global Instance
// ======================================================================

var defaultDMService DMService

// InitDMService initializes the global DM service.
func InitDMService(
	messageRepo interfaces.MessageRepository,
	userRepo interfaces.UserRepository,
	notificationRepo interfaces.NotificationRepository,
	redisAdapter adapter.RedisAdapter,
	wsHub *adapter.WebSocketHub,
) {
	defaultDMService = NewDMService(
		messageRepo,
		userRepo,
		notificationRepo,
		redisAdapter,
		wsHub,
	)
}

// GetDMService returns the global DM service.
func GetDMService() DMService {
	if defaultDMService == nil {
		panic("DM service not initialized")
	}
	return defaultDMService
}