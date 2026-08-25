// backend/internal/handler/space_handler.go
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
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/dto"
	"twitter-clone/backend/internal/middleware"
	"twitter-clone/backend/internal/service"
	"twitter-clone/backend/pkg/logger"
)

// SpaceHandler handles all audio space-related HTTP endpoints.
type SpaceHandler struct {
	spaceService service.SpaceService
	userService  service.UserService
	upgrader     websocket.Upgrader
	log          *logrus.Entry
}

// NewSpaceHandler creates a new space handler.
func NewSpaceHandler(
	spaceService service.SpaceService,
	userService service.UserService,
) *SpaceHandler {
	return &SpaceHandler{
		spaceService: spaceService,
		userService:  userService,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for WebSocket
			},
		},
		log: logger.WithField("handler", "space"),
	}
}

// ======================================================================
// Create Space
// ======================================================================

// CreateSpace handles creating a new audio space.
// @Summary Create audio space
// @Description Creates a new audio space
// @Tags spaces
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.CreateSpaceRequest true "Space details"
// @Success 201 {object} dto.SpaceResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/spaces [post]
func (h *SpaceHandler) CreateSpace(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.CreateSpaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	req.Sanitize()
	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	space, err := h.spaceService.CreateSpace(r.Context(), userID, &req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to create space")
		return
	}

	h.sendSuccess(w, http.StatusCreated, space)
}

// ======================================================================
= Get Space
// ======================================================================

// GetSpace handles retrieving a space by ID.
// @Summary Get space
// @Description Retrieves an audio space by its ID
// @Tags spaces
// @Produce json
// @Param id path string true "Space ID"
// @Success 200 {object} dto.SpaceDetailResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/spaces/{id} [get]
func (h *SpaceHandler) GetSpace(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	spaceID := vars["id"]
	if spaceID == "" {
		h.sendError(w, http.StatusBadRequest, "Space ID required", nil)
		return
	}

	currentUserID, _ := middleware.GetUserID(r.Context())

	space, err := h.spaceService.GetSpace(r.Context(), spaceID, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get space")
		return
	}

	// Get participant count
	participantCount, _ := h.spaceService.GetParticipantCount(r.Context(), spaceID)

	// Get space status
	status, _ := h.spaceService.GetSpaceStatus(r.Context(), spaceID)

	response := &dto.SpaceDetailResponse{
		Space:            space,
		ParticipantCount: participantCount,
		Status:           status,
	}

	h.sendSuccess(w, http.StatusOK, response)
}

// ======================================================================
= List Spaces
// ======================================================================

// ListSpaces handles listing spaces with pagination and filters.
// @Summary List spaces
// @Description Lists audio spaces with pagination and filtering
// @Tags spaces
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param status query string false "Filter by status (active, ended, scheduled)"
// @Param user_id query string false "Filter by creator user ID"
// @Param is_public query bool false "Filter by public/private"
// @Success 200 {object} dto.SpaceListResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/spaces [get]
func (h *SpaceHandler) ListSpaces(w http.ResponseWriter, r *http.Request) {
	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	status := r.URL.Query().Get("status")
	userID := r.URL.Query().Get("user_id")
	isPublic := r.URL.Query().Get("is_public")
	isPublicBool, _ := strconv.ParseBool(isPublic)

	currentUserID, _ := middleware.GetUserID(r.Context())

	spaces, nextCursor, total, err := h.spaceService.ListSpaces(r.Context(), cursor, limit, status, userID, isPublicBool, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to list spaces")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        spaces,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// ======================================================================
= Get Active Spaces
// ======================================================================

// GetActiveSpaces handles retrieving currently active spaces.
// @Summary Get active spaces
// @Description Retrieves all currently active audio spaces
// @Tags spaces
// @Produce json
// @Param limit query int false "Items per page (default 10, max 50)"
// @Param include_scheduled query bool false "Include scheduled spaces"
// @Success 200 {object} dto.SpaceListResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/spaces/active [get]
func (h *SpaceHandler) GetActiveSpaces(w http.ResponseWriter, r *http.Request) {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}
	includeScheduled, _ := strconv.ParseBool(r.URL.Query().Get("include_scheduled"))

	currentUserID, _ := middleware.GetUserID(r.Context())

	spaces, err := h.spaceService.GetActiveSpaces(r.Context(), limit, includeScheduled, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get active spaces")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":  spaces,
		"limit": limit,
	})
}

// ======================================================================
= Join Space
// ======================================================================

// JoinSpace handles joining an audio space.
// @Summary Join space
// @Description Joins an audio space as a participant
// @Tags spaces
// @Security BearerAuth
// @Param id path string true "Space ID"
// @Param role query string false "Role (listener, speaker) default listener"
// @Success 200 {object} dto.SpaceJoinResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/spaces/{id}/join [post]
func (h *SpaceHandler) JoinSpace(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	spaceID := vars["id"]
	if spaceID == "" {
		h.sendError(w, http.StatusBadRequest, "Space ID required", nil)
		return
	}

	role := r.URL.Query().Get("role")
	if role == "" {
		role = "listener"
	}

	result, err := h.spaceService.JoinSpace(r.Context(), spaceID, userID, role)
	if err != nil {
		h.handleServiceError(w, err, "Failed to join space")
		return
	}

	// Get space details for WebSocket connection info
	space, _ := h.spaceService.GetSpace(r.Context(), spaceID, userID)
	participantCount, _ := h.spaceService.GetParticipantCount(r.Context(), spaceID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success":          true,
		"message":          "Joined space successfully",
		"space_id":         spaceID,
		"role":             role,
		"participant_count": participantCount,
		"ws_endpoint":      fmt.Sprintf("/ws/spaces/%s", spaceID),
		"space":            space,
	})
}

// ======================================================================
= Leave Space
// ======================================================================

// LeaveSpace handles leaving an audio space.
// @Summary Leave space
// @Description Leaves an audio space
// @Tags spaces
// @Security BearerAuth
// @Param id path string true "Space ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/spaces/{id}/leave [post]
func (h *SpaceHandler) LeaveSpace(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	spaceID := vars["id"]
	if spaceID == "" {
		h.sendError(w, http.StatusBadRequest, "Space ID required", nil)
		return
	}

	if err := h.spaceService.LeaveSpace(r.Context(), spaceID, userID); err != nil {
		h.handleServiceError(w, err, "Failed to leave space")
		return
	}

	// Get updated participant count
	participantCount, _ := h.spaceService.GetParticipantCount(r.Context(), spaceID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success":          true,
		"message":          "Left space successfully",
		"space_id":         spaceID,
		"participant_count": participantCount,
	})
}

// ======================================================================
= End Space
// ======================================================================

// EndSpace handles ending an audio space.
// @Summary End space
// @Description Ends an audio space (creator/admin only)
// @Tags spaces
// @Security BearerAuth
// @Param id path string true "Space ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/spaces/{id}/end [post]
func (h *SpaceHandler) EndSpace(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	spaceID := vars["id"]
	if spaceID == "" {
		h.sendError(w, http.StatusBadRequest, "Space ID required", nil)
		return
	}

	if err := h.spaceService.EndSpace(r.Context(), spaceID, userID); err != nil {
		h.handleServiceError(w, err, "Failed to end space")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Space ended successfully",
		"space_id": spaceID,
	})
}

// ======================================================================
= Toggle Mute
// ======================================================================

// ToggleMute handles muting/unmuting a participant.
// @Summary Toggle mute
// @Description Mutes or unmutes a participant in a space (host/admin only)
// @Tags spaces
// @Security BearerAuth
// @Param id path string true "Space ID"
// @Param user_id path string true "User ID"
// @Param mute query bool true "Mute status (true=mute, false=unmute)"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/spaces/{id}/mute/{user_id} [post]
func (h *SpaceHandler) ToggleMute(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	spaceID := vars["id"]
	if spaceID == "" {
		h.sendError(w, http.StatusBadRequest, "Space ID required", nil)
		return
	}
	targetUserID := vars["user_id"]
	if targetUserID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}
	mute, _ := strconv.ParseBool(r.URL.Query().Get("mute"))

	if err := h.spaceService.ToggleMute(r.Context(), spaceID, userID, targetUserID, mute); err != nil {
		h.handleServiceError(w, err, "Failed to toggle mute")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"message":     "Mute toggled successfully",
		"space_id":    spaceID,
		"user_id":     targetUserID,
		"muted":       mute,
	})
}

// ======================================================================
= Remove Participant
// ======================================================================

// RemoveParticipant handles removing a participant from a space.
// @Summary Remove participant
// @Description Removes a participant from a space (host/admin only)
// @Tags spaces
// @Security BearerAuth
// @Param id path string true "Space ID"
// @Param user_id path string true "User ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/spaces/{id}/remove/{user_id} [post]
func (h *SpaceHandler) RemoveParticipant(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	spaceID := vars["id"]
	if spaceID == "" {
		h.sendError(w, http.StatusBadRequest, "Space ID required", nil)
		return
	}
	targetUserID := vars["user_id"]
	if targetUserID == "" {
		h.sendError(w, http.StatusBadRequest, "User ID required", nil)
		return
	}

	if err := h.spaceService.RemoveParticipant(r.Context(), spaceID, userID, targetUserID); err != nil {
		h.handleServiceError(w, err, "Failed to remove participant")
		return
	}

	// Get updated participant count
	participantCount, _ := h.spaceService.GetParticipantCount(r.Context(), spaceID)

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success":          true,
		"message":          "Participant removed successfully",
		"space_id":         spaceID,
		"user_id":          targetUserID,
		"participant_count": participantCount,
	})
}

// ======================================================================
= Invite to Space
// ======================================================================

// InviteToSpace handles inviting a user to a space.
// @Summary Invite to space
// @Description Invites a user to join an audio space (host/admin only)
// @Tags spaces
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Space ID"
// @Param request body dto.InviteToSpaceRequest true "Invite details"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/spaces/{id}/invite [post]
func (h *SpaceHandler) InviteToSpace(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	spaceID := vars["id"]
	if spaceID == "" {
		h.sendError(w, http.StatusBadRequest, "Space ID required", nil)
		return
	}

	var req dto.InviteToSpaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	// Verify invited user exists
	_, err = h.userService.GetUserByID(r.Context(), req.UserID)
	if err != nil {
		h.sendError(w, http.StatusNotFound, "User not found", nil)
		return
	}

	if err := h.spaceService.InviteToSpace(r.Context(), spaceID, userID, req.UserID); err != nil {
		h.handleServiceError(w, err, "Failed to invite user")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Invitation sent successfully",
		"space_id": spaceID,
		"user_id":  req.UserID,
	})
}

// ======================================================================
= Get Space Participants
// ======================================================================

// GetSpaceParticipants handles retrieving space participants.
// @Summary Get space participants
// @Description Retrieves all participants in a space
// @Tags spaces
// @Produce json
// @Param id path string true "Space ID"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param role query string false "Filter by role (host, speaker, listener)"
// @Success 200 {object} dto.ParticipantListResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/spaces/{id}/participants [get]
func (h *SpaceHandler) GetSpaceParticipants(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	spaceID := vars["id"]
	if spaceID == "" {
		h.sendError(w, http.StatusBadRequest, "Space ID required", nil)
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	role := r.URL.Query().Get("role")

	currentUserID, _ := middleware.GetUserID(r.Context())

	participants, nextCursor, total, err := h.spaceService.GetSpaceParticipants(r.Context(), spaceID, cursor, limit, role, currentUserID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get participants")
		return
	}

	// Build response with user details
	participantResponses := make([]*dto.ParticipantResponse, 0, len(participants))
	for _, p := range participants {
		user, err := h.userService.GetUserByID(r.Context(), p.UserID)
		if err != nil {
			continue
		}
		participantResponses = append(participantResponses, &dto.ParticipantResponse{
			UserID:     p.UserID,
			Username:   user.Username,
			FullName:   user.FullName,
			AvatarURL:  user.AvatarURL,
			Role:       p.Role,
			IsMuted:    p.IsMuted,
			JoinedAt:   p.JoinedAt,
			IsHost:     p.IsHost,
		})
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        participantResponses,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// ======================================================================
= Get Space Stats (User)
// ======================================================================

// GetUserSpaceStats handles retrieving space statistics for the user.
// @Summary Get user space stats
// @Description Retrieves space statistics for the authenticated user
// @Tags spaces
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.UserSpaceStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/spaces/stats [get]
func (h *SpaceHandler) GetUserSpaceStats(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	stats, err := h.spaceService.GetUserSpaceStats(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get space stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= WebSocket Connection for Space
// ======================================================================

// ServeWebSocket handles WebSocket connections for audio spaces.
// @Summary WebSocket for space
// @Description Establishes a WebSocket connection for real-time space audio
// @Tags spaces
// @Security BearerAuth
// @Param id path string true "Space ID"
// @Success 101 "Switching Protocols"
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /ws/spaces/{id} [get]
func (h *SpaceHandler) ServeWebSocket(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	spaceID := vars["id"]
	if spaceID == "" {
		h.sendError(w, http.StatusBadRequest, "Space ID required", nil)
		return
	}

	// Verify user is a participant
	isParticipant, err := h.spaceService.IsParticipant(r.Context(), spaceID, userID)
	if err != nil || !isParticipant {
		h.sendError(w, http.StatusForbidden, "Not a participant of this space", nil)
		return
	}

	// Verify space is active
	status, err := h.spaceService.GetSpaceStatus(r.Context(), spaceID)
	if err != nil || status != "active" {
		h.sendError(w, http.StatusBadRequest, "Space is not active", nil)
		return
	}

	// Upgrade to WebSocket
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.WithError(err).Error("Failed to upgrade to WebSocket")
		return
	}
	defer conn.Close()

	// Handle WebSocket messages
	h.handleWebSocketConnection(conn, spaceID, userID)
}

// ======================================================================
= WebSocket Connection Handler
// ======================================================================

// handleWebSocketConnection handles a WebSocket connection.
func (h *SpaceHandler) handleWebSocketConnection(conn *websocket.Conn, spaceID, userID string) {
	// Set read/write deadlines
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Send initial connection confirmation
	h.sendWSMessage(conn, map[string]interface{}{
		"type":    "connected",
		"space_id": spaceID,
		"user_id": userID,
	})

	// Message loop
	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.log.WithError(err).Warn("WebSocket read error")
			}
			break
		}

		msgType, _ := msg["type"].(string)

		switch msgType {
		case "ping":
			h.sendWSMessage(conn, map[string]interface{}{
				"type": "pong",
				"timestamp": time.Now().Unix(),
			})

		case "audio":
			// Forward audio data to other participants
			h.spaceService.BroadcastAudio(spaceID, userID, msg)

		case "speaker_status":
			// Update speaker status
			speaking, _ := msg["speaking"].(bool)
			if err := h.spaceService.UpdateSpeakerStatus(spaceID, userID, speaking); err != nil {
				h.log.WithError(err).Warn("Failed to update speaker status")
			}

		case "raise_hand":
			if err := h.spaceService.RaiseHand(spaceID, userID); err != nil {
				h.log.WithError(err).Warn("Failed to raise hand")
			}

		case "lower_hand":
			if err := h.spaceService.LowerHand(spaceID, userID); err != nil {
				h.log.WithError(err).Warn("Failed to lower hand")
			}

		case "audio_settings":
			// Update audio settings (volume, bitrate, etc.)
			settings, _ := msg["settings"].(map[string]interface{})
			if err := h.spaceService.UpdateAudioSettings(spaceID, userID, settings); err != nil {
				h.log.WithError(err).Warn("Failed to update audio settings")
			}

		default:
			h.log.WithField("type", msgType).Debug("Unknown WebSocket message type")
		}
	}
}

// sendWSMessage sends a message to the WebSocket connection.
func (h *SpaceHandler) sendWSMessage(conn *websocket.Conn, data interface{}) {
	if err := conn.WriteJSON(data); err != nil {
		h.log.WithError(err).Warn("Failed to send WebSocket message")
	}
}

// ======================================================================
= Admin Endpoints
// ======================================================================

// AdminListSpaces handles admin listing of all spaces.
// @Summary Admin list spaces
// @Description Lists all spaces for admin moderation
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param status query string false "Filter by status (active, ended, scheduled)"
// @Param search query string false "Search by title or description"
// @Param created_by query string false "Filter by creator ID"
// @Success 200 {object} dto.SpaceAdminListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/spaces [get]
func (h *SpaceHandler) AdminListSpaces(w http.ResponseWriter, r *http.Request) {
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
	createdBy := r.URL.Query().Get("created_by")

	spaces, nextCursor, total, err := h.spaceService.AdminListSpaces(r.Context(), cursor, limit, status, search, createdBy)
	if err != nil {
		h.handleServiceError(w, err, "Failed to list spaces")
		return
	}

	// Build admin response
	responses := make([]*dto.SpaceAdminResponse, 0, len(spaces))
	for _, s := range spaces {
		creator, _ := h.userService.GetUserByID(r.Context(), s.CreatedBy)
		responses = append(responses, &dto.SpaceAdminResponse{
			ID:          s.ID,
			Title:       s.Title,
			Description: s.Description,
			Status:      s.Status,
			CreatedBy:   s.CreatedBy,
			CreatorUsername: func() string {
				if creator != nil {
					return creator.Username
				}
				return ""
			}(),
			ParticipantCount: s.ParticipantCount,
			IsPublic:         s.IsPublic,
			StartedAt:        s.StartedAt,
			EndedAt:          s.EndedAt,
			CreatedAt:        s.CreatedAt,
			UpdatedAt:        s.UpdatedAt,
			DeletedAt:        s.DeletedAt,
		})
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        responses,
		"next_cursor": nextCursor,
		"has_more":    nextCursor != "",
		"limit":       limit,
		"total":       total,
	})
}

// AdminEndSpace handles admin ending of a space.
// @Summary Admin end space
// @Description Ends a space (admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "Space ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/spaces/{id}/end [post]
func (h *SpaceHandler) AdminEndSpace(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	spaceID := vars["id"]
	if spaceID == "" {
		h.sendError(w, http.StatusBadRequest, "Space ID required", nil)
		return
	}

	if err := h.spaceService.AdminEndSpace(r.Context(), spaceID); err != nil {
		h.handleServiceError(w, err, "Failed to end space")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Space ended successfully",
		"space_id": spaceID,
	})
}

// AdminGetSpaceStats handles retrieving global space statistics.
// @Summary Admin get space stats
// @Description Retrieves global space statistics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Success 200 {object} dto.SpaceGlobalStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/spaces/stats [get]
func (h *SpaceHandler) AdminGetSpaceStats(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 30 {
		days = 7
	}

	stats, err := h.spaceService.AdminGetSpaceStats(r.Context(), days)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get space stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Helper Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *SpaceHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *SpaceHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *SpaceHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *SpaceHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrSpaceNotFound):
		h.sendError(w, http.StatusNotFound, "Space not found", nil)
	case errors.Is(err, service.ErrSpaceEnded):
		h.sendError(w, http.StatusBadRequest, "Space has ended", nil)
	case errors.Is(err, service.ErrSpaceFull):
		h.sendError(w, http.StatusBadRequest, "Space is full", nil)
	case errors.Is(err, service.ErrNotSpaceParticipant):
		h.sendError(w, http.StatusForbidden, "Not a participant of this space", nil)
	case errors.Is(err, service.ErrNotSpaceHost):
		h.sendError(w, http.StatusForbidden, "Not the host of this space", nil)
	case errors.Is(err, service.ErrNotSpaceModerator):
		h.sendError(w, http.StatusForbidden, "Not a moderator of this space", nil)
	case errors.Is(err, service.ErrUserAlreadyInSpace):
		h.sendError(w, http.StatusConflict, "User already in space", nil)
	case errors.Is(err, service.ErrSpaceAlreadyActive):
		h.sendError(w, http.StatusBadRequest, "Space is already active", nil)
	case errors.Is(err, service.ErrUserBannedFromSpace):
		h.sendError(w, http.StatusForbidden, "User is banned from this space", nil)
	case errors.Is(err, service.ErrInvalidSpaceStatus):
		h.sendError(w, http.StatusBadRequest, "Invalid space status", nil)
	case errors.Is(err, service.ErrSpaceTitleRequired):
		h.sendError(w, http.StatusBadRequest, "Space title is required", nil)
	case errors.Is(err, service.ErrSpaceTitleTooLong):
		h.sendError(w, http.StatusBadRequest, "Space title is too long", nil)
	case errors.Is(err, service.ErrSpaceDescriptionTooLong):
		h.sendError(w, http.StatusBadRequest, "Space description is too long", nil)
	case errors.Is(err, service.ErrSpaceMaxParticipantsExceeded):
		h.sendError(w, http.StatusBadRequest, "Maximum participants exceeded", nil)
	case errors.Is(err, service.ErrSpaceDurationTooLong):
		h.sendError(w, http.StatusBadRequest, "Space duration is too long", nil)
	case errors.Is(err, service.ErrSpaceStartTimeInvalid):
		h.sendError(w, http.StatusBadRequest, "Invalid start time", nil)
	case errors.Is(err, service.ErrSpaceEndTimeInvalid):
		h.sendError(w, http.StatusBadRequest, "Invalid end time", nil)
	case errors.Is(err, service.ErrSpaceRecordingFailed):
		h.sendError(w, http.StatusInternalServerError, "Failed to record space", nil)
	case errors.Is(err, service.ErrUserNotFound):
		h.sendError(w, http.StatusNotFound, "User not found", nil)
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

// HealthCheck returns the health status of the space handler.
func (h *SpaceHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "space_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}