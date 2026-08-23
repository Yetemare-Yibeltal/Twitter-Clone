// backend/internal/handler/poll_handler.go
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/dto"
	"twitter-clone/backend/internal/middleware"
	"twitter-clone/backend/internal/service"
	"twitter-clone/backend/pkg/logger"
)

// PollHandler handles all poll-related HTTP endpoints.
type PollHandler struct {
	pollService    service.PollService
	tweetService   service.TweetService
	notificationService service.NotificationService
	log            *logrus.Entry
}

// NewPollHandler creates a new poll handler.
func NewPollHandler(
	pollService service.PollService,
	tweetService service.TweetService,
	notificationService service.NotificationService,
) *PollHandler {
	return &PollHandler{
		pollService:    pollService,
		tweetService:   tweetService,
		notificationService: notificationService,
		log:            logger.WithField("handler", "poll"),
	}
}

// ======================================================================
// Create Poll
// ======================================================================

// CreatePoll handles creating a poll (via tweet creation).
// @Summary Create poll
// @Description Creates a new poll within a tweet
// @Tags polls
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.CreatePollRequest true "Poll details"
// @Success 201 {object} dto.PollResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/polls [post]
func (h *PollHandler) CreatePoll(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.CreatePollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	req.Sanitize()
	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	poll, err := h.pollService.CreatePoll(r.Context(), userID, &req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to create poll")
		return
	}

	h.sendSuccess(w, http.StatusCreated, poll)
}

// ======================================================================
// Get Poll
// ======================================================================

// GetPoll handles retrieving a poll by ID.
// @Summary Get poll
// @Description Retrieves a poll by its ID
// @Tags polls
// @Produce json
// @Param id path string true "Poll ID"
// @Success 200 {object} dto.PollResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/polls/{id} [get]
func (h *PollHandler) GetPoll(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pollID := vars["id"]
	if pollID == "" {
		h.sendError(w, http.StatusBadRequest, "Poll ID required", nil)
		return
	}

	currentUserID, _ := middleware.GetUserID(r.Context())

	poll, err := h.pollService.GetPoll(r.Context(), pollID, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get poll")
		return
	}

	h.sendSuccess(w, http.StatusOK, poll)
}

// ======================================================================
= Get Poll by Tweet ID
// ======================================================================

// GetPollByTweetID handles retrieving a poll by tweet ID.
// @Summary Get poll by tweet
// @Description Retrieves a poll associated with a tweet
// @Tags polls
// @Produce json
// @Param tweet_id path string true "Tweet ID"
// @Success 200 {object} dto.PollResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/polls/tweet/{tweet_id} [get]
func (h *PollHandler) GetPollByTweetID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tweetID := vars["tweet_id"]
	if tweetID == "" {
		h.sendError(w, http.StatusBadRequest, "Tweet ID required", nil)
		return
	}

	currentUserID, _ := middleware.GetUserID(r.Context())

	poll, err := h.pollService.GetPollByTweetID(r.Context(), tweetID, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get poll")
		return
	}

	h.sendSuccess(w, http.StatusOK, poll)
}

// ======================================================================
= Vote
// ======================================================================

// Vote handles voting on a poll.
// @Summary Vote on poll
// @Description Casts a vote on a poll option
// @Tags polls
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Poll ID"
// @Param request body dto.VoteRequest true "Vote details"
// @Success 200 {object} dto.PollResultResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/polls/{id}/vote [post]
func (h *PollHandler) Vote(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	pollID := vars["id"]
	if pollID == "" {
		h.sendError(w, http.StatusBadRequest, "Poll ID required", nil)
		return
	}

	var req dto.VoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	result, err := h.pollService.Vote(r.Context(), pollID, userID, req.OptionID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to vote")
		return
	}

	// Create notification for tweet owner
	// This would be handled by the service

	h.sendSuccess(w, http.StatusOK, result)
}

// ======================================================================
= Unvote
// ======================================================================

// Unvote handles removing a vote from a poll.
// @Summary Unvote from poll
// @Description Removes a user's vote from a poll
// @Tags polls
// @Security BearerAuth
// @Param id path string true "Poll ID"
// @Success 200 {object} dto.PollResultResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/polls/{id}/unvote [post]
func (h *PollHandler) Unvote(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	pollID := vars["id"]
	if pollID == "" {
		h.sendError(w, http.StatusBadRequest, "Poll ID required", nil)
		return
	}

	result, err := h.pollService.Unvote(r.Context(), pollID, userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to remove vote")
		return
	}

	h.sendSuccess(w, http.StatusOK, result)
}

// ======================================================================
= Get Poll Results
// ======================================================================

// GetPollResults handles retrieving poll results.
// @Summary Get poll results
// @Description Retrieves the current results of a poll
// @Tags polls
// @Produce json
// @Param id path string true "Poll ID"
// @Success 200 {object} dto.PollResultResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/polls/{id}/results [get]
func (h *PollHandler) GetPollResults(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pollID := vars["id"]
	if pollID == "" {
		h.sendError(w, http.StatusBadRequest, "Poll ID required", nil)
		return
	}

	currentUserID, _ := middleware.GetUserID(r.Context())

	results, err := h.pollService.GetPollResults(r.Context(), pollID, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get poll results")
		return
	}

	h.sendSuccess(w, http.StatusOK, results)
}

// ======================================================================
= Get Poll Status
// ======================================================================

// GetPollStatus handles checking poll status (active/expired).
// @Summary Get poll status
// @Description Checks if a poll is active or expired
// @Tags polls
// @Produce json
// @Param id path string true "Poll ID"
// @Success 200 {object} dto.PollStatusResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/polls/{id}/status [get]
func (h *PollHandler) GetPollStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pollID := vars["id"]
	if pollID == "" {
		h.sendError(w, http.StatusBadRequest, "Poll ID required", nil)
		return
	}

	status, err := h.pollService.GetPollStatus(r.Context(), pollID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get poll status")
		return
	}

	h.sendSuccess(w, http.StatusOK, status)
}

// ======================================================================
= List Polls
// ======================================================================

// ListPolls handles listing polls with pagination and filters.
// @Summary List polls
// @Description Lists polls with pagination and filtering
// @Tags polls
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param status query string false "Filter by status (active, expired)"
// @Param user_id query string false "Filter by creator user ID"
// @Success 200 {object} dto.PollListResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/polls [get]
func (h *PollHandler) ListPolls(w http.ResponseWriter, r *http.Request) {
	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	status := r.URL.Query().Get("status")
	userID := r.URL.Query().Get("user_id")

	currentUserID, _ := middleware.GetUserID(r.Context())

	polls, nextCursor, total, err := h.pollService.ListPolls(r.Context(), cursor, limit, status, userID, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to list polls")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        polls,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// ======================================================================
= Get Trending Polls
// ======================================================================

// GetTrendingPolls handles retrieving trending polls.
// @Summary Get trending polls
// @Description Retrieves the most popular/trending polls
// @Tags polls
// @Produce json
// @Param limit query int false "Items per page (default 10, max 50)"
// @Param since query string false "Since timestamp"
// @Success 200 {object} dto.PollListResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/polls/trending [get]
func (h *PollHandler) GetTrendingPolls(w http.ResponseWriter, r *http.Request) {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}
	since := r.URL.Query().Get("since")
	var sinceTime time.Time
	if since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			sinceTime = t
		}
	}
	if sinceTime.IsZero() {
		sinceTime = time.Now().Add(-7 * 24 * time.Hour)
	}

	currentUserID, _ := middleware.GetUserID(r.Context())

	polls, err := h.pollService.GetTrendingPolls(r.Context(), limit, sinceTime, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get trending polls")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":  polls,
		"limit": limit,
		"since": sinceTime.Format(time.RFC3339),
	})
}

// ======================================================================
= Get Poll Stats (User)
// ======================================================================

// GetUserPollStats handles retrieving poll statistics for the user.
// @Summary Get user poll stats
// @Description Retrieves poll statistics for the authenticated user
// @Tags polls
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.UserPollStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/polls/stats [get]
func (h *PollHandler) GetUserPollStats(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	stats, err := h.pollService.GetUserPollStats(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get poll stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Admin Endpoints
// ======================================================================

// AdminListPolls handles admin listing of all polls.
// @Summary Admin list polls
// @Description Lists all polls for admin moderation
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param status query string false "Filter by status (active, expired)"
// @Param search query string false "Search by poll question"
// @Success 200 {object} dto.PollListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/polls [get]
func (h *PollHandler) AdminListPolls(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	status := r.URL.Query().Get("status")
	search := r.URL.Query().Get("search")

	polls, nextCursor, total, err := h.pollService.AdminListPolls(r.Context(), cursor, limit, status, search)
	if err != nil {
		h.handleServiceError(w, err, "Failed to list polls")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        polls,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// AdminDeletePoll handles admin deletion of a poll.
// @Summary Admin delete poll
// @Description Deletes a poll (admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "Poll ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/polls/{id} [delete]
func (h *PollHandler) AdminDeletePoll(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	pollID := vars["id"]
	if pollID == "" {
		h.sendError(w, http.StatusBadRequest, "Poll ID required", nil)
		return
	}

	if err := h.pollService.AdminDeletePoll(r.Context(), pollID); err != nil {
		h.handleServiceError(w, err, "Failed to delete poll")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Poll deleted successfully",
	})
}

// AdminGetPollStats handles retrieving global poll statistics.
// @Summary Admin get poll stats
// @Description Retrieves global poll statistics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.GlobalPollStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/polls/stats [get]
func (h *PollHandler) AdminGetPollStats(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	stats, err := h.pollService.AdminGetPollStats(r.Context())
	if err != nil {
		h.handleServiceError(w, err, "Failed to get poll stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Helper Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *PollHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *PollHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := dto.ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
		Code:    status,
		Details: details,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.log.WithError(err).Error("Failed to encode error response")
	}
}

// sendValidationError handles validation errors.
func (h *PollHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *PollHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrPollNotFound):
		h.sendError(w, http.StatusNotFound, "Poll not found", nil)
	case errors.Is(err, service.ErrPollExpired):
		h.sendError(w, http.StatusBadRequest, "Poll has expired", nil)
	case errors.Is(err, service.ErrPollAlreadyVoted):
		h.sendError(w, http.StatusConflict, "Already voted on this poll", nil)
	case errors.Is(err, service.ErrInvalidPollOption):
		h.sendError(w, http.StatusBadRequest, "Invalid poll option", nil)
	case errors.Is(err, service.ErrInvalidPollDuration):
		h.sendError(w, http.StatusBadRequest, "Invalid poll duration", nil)
	case errors.Is(err, service.ErrPollOptionsTooFew):
		h.sendError(w, http.StatusBadRequest, "Poll must have at least 2 options", nil)
	case errors.Is(err, service.ErrPollOptionsTooMany):
		h.sendError(w, http.StatusBadRequest, "Poll can have at most 4 options", nil)
	case errors.Is(err, service.ErrPollOptionEmpty):
		h.sendError(w, http.StatusBadRequest, "Poll option cannot be empty", nil)
	case errors.Is(err, service.ErrPollOptionTooLong):
		h.sendError(w, http.StatusBadRequest, "Poll option is too long", nil)
	case errors.Is(err, context.Canceled):
		h.sendError(w, http.StatusRequestTimeout, "Request cancelled", nil)
	case errors.Is(err, context.DeadlineExceeded):
		h.sendError(w, http.StatusGatewayTimeout, "Request timed out", nil)
	default:
		h.log.WithError(err).Error(defaultMsg)
		h.sendError(w, http.StatusInternalServerError, "Internal server error", nil)
	}
}

// ======================================================================
= Health Check
// ======================================================================

// HealthCheck returns the health status of the poll handler.
func (h *PollHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "poll_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}