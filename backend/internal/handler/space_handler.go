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
	wsHub        *WebSocketHub
	upgrader     websocket.Upgrader
	log          *logrus.Entry
}

// NewSpaceHandler creates a new space handler.
func NewSpaceHandler(
	spaceService service.SpaceService,
	wsHub *WebSocketHub,
) *SpaceHandler {
	return &SpaceHandler{
		spaceService: spaceService,
		wsHub:        wsHub,
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

	h.sendSuccess(w, http.StatusOK, space)
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

	currentUserID, _ := middleware.GetUserID(r.Context())

	spaces, nextCursor, total, err := h.spaceService.ListSpaces(r.Context(), cursor, limit, status, userID, currentUserID)
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
// @Success 200 {object} dto.SpaceListResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/spaces/active [get]
func (h *SpaceHandler) GetActiveSpaces(w http.ResponseWriter, r *http.Request) {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}

	currentUserID, _ := middleware.GetUserID(r.Context())

	spaces, err := h.spaceService.GetActiveSpaces(r.Context(), limit, currentUserID)
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
// @Success 200 {object} dto.SpaceJoinResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
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

	space, err := h.spaceService.JoinSpace(r.Context(), spaceID, userID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to join space")
		return
	}

	// Notify other participants via WebSocket
	if h.wsHub != nil {
		h.wsHub.BroadcastToRoom("space:"+spaceID, map[string]interface{}{
			"type": "participant_joined",
			"user_id": userID,
			"space_id": spaceID,
			"timestamp": time.Now().Unix(),
		})
	}

	h.sendSuccess(w, http.StatusOK, space)
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

	// Notify other participants via WebSocket
	if h.wsHub != nil {
		h.wsHub.BroadcastToRoom("space:"+spaceID, map[string]interface{}{
			"type": "participant_left",
			"user_id": userID,
			"space_id": spaceID,
			"timestamp": time.Now().Unix(),
		})
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Left space successfully",
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

	// Notify all participants via WebSocket
	if h.wsHub != nil {
		h.wsHub.BroadcastToRoom("space:"+spaceID, map[string]interface{}{
			"type": "space_ended",
			"space_id": spaceID,
			"timestamp": time.Now().Unix(),
		})
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Space ended successfully",
	})
}

// ======================================================================
= Toggle Mute
// ======================================================================

// ToggleMute handles muting/unmuting a participant.
// @Summary Toggle mute
// @Description Mutes or unmutes a participant in a space
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

	// Notify all participants via WebSocket
	if h.wsHub != nil {
		h.wsHub.BroadcastToRoom("space:"+spaceID, map[string]interface{}{
			"type": "mute_toggled",
			"space_id": spaceID,
			"user_id": targetUserID,
			"muted": mute,
			"timestamp": time.Now().Unix(),
		})
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Mute toggled successfully",
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

	// Notify removed participant and others via WebSocket
	if h.wsHub != nil {
		h.wsHub.BroadcastToRoom("space:"+spaceID, map[string]interface{}{
			"type": "participant_removed",
			"space_id": spaceID,
			"user_id": targetUserID,
			"timestamp": time.Now().Unix(),
		})
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Participant removed successfully",
	})
}

// ======================================================================
= Invite to Space
// ======================================================================

// InviteToSpace handles inviting a user to a space.
// @Summary Invite to space
// @Description Invites a user to join an audio space
// @Tags spaces
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Space ID"
// @Param request body dto.InviteRequest true "Invite details"
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

	var req dto.InviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	if err := h.spaceService.InviteToSpace(r.Context(), spaceID, userID, req.UserID); err != nil {
		h.handleServiceError(w, err, "Failed to invite user")
		return
	}

	// Send notification to invited user via WebSocket
	if h.wsHub != nil {
		h.wsHub.BroadcastToUser(req.UserID, map[string]interface{}{
			"type": "space_invite",
			"space_id": spaceID,
			"inviter_id": userID,
			"timestamp": time.Now().Unix(),
		})
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Invitation sent successfully",
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

	participants, nextCursor, total, err := h.spaceService.GetSpaceParticipants(r.Context(), spaceID, cursor, limit)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get participants")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"data":        participants,
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

// ServeWS handles WebSocket connections for audio spaces.
// @Summary WebSocket for space
// @Description Establishes a WebSocket connection for real-time space audio
// @Tags spaces
// @Security BearerAuth
// @Param id path string true "Space ID"
// @Success 101 "Switching Protocols"
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/spaces/{id}/ws [get]
func (h *SpaceHandler) ServeWS(w http.ResponseWriter, r *http.Request) {
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

	// Verify user is a participant or host
	isParticipant, err := h.spaceService.IsParticipant(r.Context(), spaceID, userID)
	if err != nil || !isParticipant {
		h.sendError(w, http.StatusForbidden, "Not a participant of this space", nil)
		return
	}

	// Upgrade to WebSocket
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.WithError(err).Error("Failed to upgrade to WebSocket")
		return
	}

	// Create WebSocket client for this space
	client := &SpaceWebSocketClient{
		ID:      userID,
		SpaceID: spaceID,
		Conn:    conn,
		Hub:     h.wsHub,
		Send:    make(chan []byte, 256),
		Handler: h,
	}

	// Register client with WebSocket hub
	if h.wsHub != nil {
		h.wsHub.RegisterSpaceClient(spaceID, client)
	}

	// Add to space room
	if h.wsHub != nil {
		h.wsHub.JoinRoom("space:"+spaceID, conn)
	}

	// Notify others in the space
	if h.wsHub != nil {
		h.wsHub.BroadcastToRoom("space:"+spaceID, map[string]interface{}{
			"type": "user_connected",
			"user_id": userID,
			"space_id": spaceID,
			"timestamp": time.Now().Unix(),
		})
	}

	// Start goroutines
	go h.writePump(client)
	go h.readPump(client)

	h.log.WithFields(logrus.Fields{
		"user_id":  userID,
		"space_id": spaceID,
	}).Info("WebSocket connected for space")
}

// ======================================================================
= WebSocket Client Management
// ======================================================================

// SpaceWebSocketClient represents a WebSocket client for a space.
type SpaceWebSocketClient struct {
	ID      string
	SpaceID string
	Conn    *websocket.Conn
	Hub     *WebSocketHub
	Send    chan []byte
	Handler *SpaceHandler
}

// readPump reads messages from the WebSocket connection.
func (h *SpaceHandler) readPump(client *SpaceWebSocketClient) {
	defer func() {
		client.Conn.Close()
		if h.wsHub != nil {
			h.wsHub.UnregisterSpaceClient(client.SpaceID, client)
		}
	}()

	client.Conn.SetReadLimit(512 * 1024)
	client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			break
		}

		// Process message
		h.handleSpaceMessage(client, message)
	}
}

// writePump writes messages to the WebSocket connection.
func (h *SpaceHandler) writePump(client *SpaceWebSocketClient) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		client.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := client.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages
			n := len(client.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-client.Send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleSpaceMessage processes incoming WebSocket messages for a space.
func (h *SpaceHandler) handleSpaceMessage(client *SpaceWebSocketClient, data []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	msgType, ok := msg["type"].(string)
	if !ok {
		return
	}

	switch msgType {
	case "ping":
		h.sendSpaceMessage(client, map[string]interface{}{
			"type": "pong",
			"timestamp": time.Now().Unix(),
		})

	case "audio":
		// Forward audio data to other participants
		if h.wsHub != nil {
			h.wsHub.BroadcastToRoom("space:"+client.SpaceID, map[string]interface{}{
				"type": "audio",
				"user_id": client.ID,
				"data": msg["data"],
				"timestamp": time.Now().Unix(),
			})
		}

	case "raise_hand":
		if h.wsHub != nil {
			h.wsHub.BroadcastToRoom("space:"+client.SpaceID, map[string]interface{}{
				"type": "hand_raised",
				"user_id": client.ID,
				"timestamp": time.Now().Unix(),
			})
		}

	case "lower_hand":
		if h.wsHub != nil {
			h.wsHub.BroadcastToRoom("space:"+client.SpaceID, map[string]interface{}{
				"type": "hand_lowered",
				"user_id": client.ID,
				"timestamp": time.Now().Unix(),
			})
		}

	case "speaker_status":
		if h.wsHub != nil {
			h.wsHub.BroadcastToRoom("space:"+client.SpaceID, map[string]interface{}{
				"type": "speaker_status",
				"user_id": client.ID,
				"speaking": msg["speaking"],
				"timestamp": time.Now().Unix(),
			})
		}

	case "audio_settings":
		// Update audio settings (volume, etc.)
		// Store in user session
		h.log.WithFields(logrus.Fields{
			"user_id": client.ID,
			"space_id": client.SpaceID,
		}).Debug("Audio settings updated")

	default:
		h.log.WithField("type", msgType).Debug("Unknown WebSocket message type")
	}
}

// sendSpaceMessage sends a message to a space client.
func (h *SpaceHandler) sendSpaceMessage(client *SpaceWebSocketClient, data interface{}) {
	bytes, err := json.Marshal(data)
	if err != nil {
		h.log.WithError(err).Error("Failed to marshal message")
		return
	}
	select {
	case client.Send <- bytes:
	default:
		h.log.WithField("user_id", client.ID).Warn("Client send channel full")
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
// @Success 200 {object} dto.SpaceListResponse
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

	spaces, nextCursor, total, err := h.spaceService.AdminListSpaces(r.Context(), cursor, limit, status, search)
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

	// Notify all participants via WebSocket
	if h.wsHub != nil {
		h.wsHub.BroadcastToRoom("space:"+spaceID, map[string]interface{}{
			"type": "space_ended",
			"space_id": spaceID,
			"timestamp": time.Now().Unix(),
		})
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Space ended successfully",
	})
}

// AdminGetSpaceStats handles retrieving global space statistics.
// @Summary Admin get space stats
// @Description Retrieves global space statistics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.GlobalSpaceStatsResponse
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

	stats, err := h.spaceService.AdminGetSpaceStats(r.Context())
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