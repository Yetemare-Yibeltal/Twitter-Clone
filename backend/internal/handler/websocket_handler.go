// backend/internal/handler/webhook_handler.go
package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"twitter-clone/backend/internal/config"
	"twitter-clone/backend/internal/dto"
	"twitter-clone/backend/internal/middleware"
	"twitter-clone/backend/internal/service"
	"twitter-clone/backend/pkg/logger"
)

// WebhookHandler handles all webhook-related HTTP endpoints.
type WebhookHandler struct {
	webhookService service.WebhookService
	paymentService service.PaymentService
	userService    service.UserService
	config         *config.Config
	log            *logrus.Entry
}

// NewWebhookHandler creates a new webhook handler.
func NewWebhookHandler(
	webhookService service.WebhookService,
	paymentService service.PaymentService,
	userService service.UserService,
	cfg *config.Config,
) *WebhookHandler {
	return &WebhookHandler{
		webhookService: webhookService,
		paymentService: paymentService,
		userService:    userService,
		config:         cfg,
		log:            logger.WithField("handler", "webhook"),
	}
}

// ======================================================================
// Stripe Webhook
// ======================================================================

// HandleStripeWebhook handles incoming Stripe webhook events.
// @Summary Handle Stripe webhook
// @Description Processes incoming Stripe webhook events
// @Tags webhooks
// @Accept json
// @Produce json
// @Param payload body object true "Stripe webhook payload"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /webhooks/stripe [post]
func (h *WebhookHandler) HandleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	// Verify Stripe signature
	signature := r.Header.Get("Stripe-Signature")
	if signature == "" && h.config.Environment == "production" {
		h.sendError(w, http.StatusBadRequest, "Missing Stripe signature", nil)
		return
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Failed to read request body", nil)
		return
	}

	// Verify signature in production
	if h.config.Environment == "production" && h.config.StripeWebhookSecret != "" {
		if !h.verifyStripeSignature(signature, body) {
			h.sendError(w, http.StatusUnauthorized, "Invalid Stripe signature", nil)
			return
		}
	}

	// Parse event
	var event map[string]interface{}
	if err := json.Unmarshal(body, &event); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid webhook payload", nil)
		return
	}

	eventType, ok := event["type"].(string)
	if !ok {
		h.sendError(w, http.StatusBadRequest, "Missing event type", nil)
		return
	}

	// Process event
	if err := h.webhookService.ProcessStripeWebhook(r.Context(), eventType, event); err != nil {
		h.handleServiceError(w, err, "Failed to process Stripe webhook")
		return
	}

	// Handle specific events
	switch eventType {
	case "customer.subscription.created", "customer.subscription.updated":
		h.handleSubscriptionChange(r.Context(), event)
	case "customer.subscription.deleted":
		h.handleSubscriptionDeleted(r.Context(), event)
	case "invoice.payment_succeeded":
		h.handleInvoicePaymentSucceeded(r.Context(), event)
	case "invoice.payment_failed":
		h.handleInvoicePaymentFailed(r.Context(), event)
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"received": true,
		"event":    eventType,
	})
}

// verifyStripeSignature verifies the Stripe webhook signature.
func (h *WebhookHandler) verifyStripeSignature(signature string, body []byte) bool {
	if h.config.StripeWebhookSecret == "" {
		return true
	}
	// Parse timestamp and signature
	parts := strings.Split(signature, ",")
	var timestamp, sig string
	for _, part := range parts {
		if strings.HasPrefix(part, "t=") {
			timestamp = strings.TrimPrefix(part, "t=")
		} else if strings.HasPrefix(part, "v1=") {
			sig = strings.TrimPrefix(part, "v1=")
		}
	}
	if timestamp == "" || sig == "" {
		return false
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix()-ts > 300 { // 5 minutes
		return false
	}
	signedPayload := timestamp + "." + string(body)
	mac := hmac.New(sha256.New, []byte(h.config.StripeWebhookSecret))
	mac.Write([]byte(signedPayload))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expectedSig))
}

// handleSubscriptionChange handles subscription created/updated events.
func (h *WebhookHandler) handleSubscriptionChange(ctx context.Context, event map[string]interface{}) {
	data, _ := event["data"].(map[string]interface{})
	obj, _ := data["object"].(map[string]interface{})
	customerID, _ := obj["customer"].(string)
	subscriptionID, _ := obj["id"].(string)
	status, _ := obj["status"].(string)

	user, err := h.userService.GetUserByStripeCustomerID(ctx, customerID)
	if err != nil {
		h.log.WithError(err).Warn("User not found for Stripe customer")
		return
	}
	_ = h.userService.UpdateSubscription(ctx, user.ID, subscriptionID, status)
}

// handleSubscriptionDeleted handles subscription deletion.
func (h *WebhookHandler) handleSubscriptionDeleted(ctx context.Context, event map[string]interface{}) {
	data, _ := event["data"].(map[string]interface{})
	obj, _ := data["object"].(map[string]interface{})
	customerID, _ := obj["customer"].(string)
	user, err := h.userService.GetUserByStripeCustomerID(ctx, customerID)
	if err != nil {
		h.log.WithError(err).Warn("User not found for Stripe customer")
		return
	}
	_ = h.userService.RemoveSubscription(ctx, user.ID)
}

// handleInvoicePaymentSucceeded handles successful invoice payment.
func (h *WebhookHandler) handleInvoicePaymentSucceeded(ctx context.Context, event map[string]interface{}) {
	data, _ := event["data"].(map[string]interface{})
	obj, _ := data["object"].(map[string]interface{})
	customerID, _ := obj["customer"].(string)
	invoiceID, _ := obj["id"].(string)
	amountPaid, _ := obj["amount_paid"].(float64)

	user, err := h.userService.GetUserByStripeCustomerID(ctx, customerID)
	if err != nil {
		h.log.WithError(err).Warn("User not found for Stripe customer")
		return
	}
	_ = h.paymentService.RecordPayment(ctx, user.ID, invoiceID, amountPaid, "stripe")
}

// handleInvoicePaymentFailed handles failed invoice payment.
func (h *WebhookHandler) handleInvoicePaymentFailed(ctx context.Context, event map[string]interface{}) {
	data, _ := event["data"].(map[string]interface{})
	obj, _ := data["object"].(map[string]interface{})
	customerID, _ := obj["customer"].(string)
	invoiceID, _ := obj["id"].(string)

	user, err := h.userService.GetUserByStripeCustomerID(ctx, customerID)
	if err != nil {
		h.log.WithError(err).Warn("User not found for Stripe customer")
		return
	}
	_ = h.paymentService.RecordPaymentFailure(ctx, user.ID, invoiceID, "stripe")
}

// ======================================================================
// GitHub Webhook
// ======================================================================

// HandleGitHubWebhook handles incoming GitHub webhook events.
// @Summary Handle GitHub webhook
// @Description Processes incoming GitHub webhook events
// @Tags webhooks
// @Accept json
// @Produce json
// @Param event header string true "GitHub event type"
// @Param payload body object true "GitHub webhook payload"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /webhooks/github [post]
func (h *WebhookHandler) HandleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	// Verify GitHub signature
	signature := r.Header.Get("X-Hub-Signature-256")
	if signature == "" && h.config.Environment == "production" {
		h.sendError(w, http.StatusBadRequest, "Missing GitHub signature", nil)
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")
	if eventType == "" {
		h.sendError(w, http.StatusBadRequest, "Missing GitHub event type", nil)
		return
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Failed to read request body", nil)
		return
	}

	// Verify signature in production
	if h.config.Environment == "production" && h.config.GitHubWebhookSecret != "" {
		if !h.verifyGitHubSignature(signature, body) {
			h.sendError(w, http.StatusUnauthorized, "Invalid GitHub signature", nil)
			return
		}
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid webhook payload", nil)
		return
	}

	if err := h.webhookService.ProcessGitHubWebhook(r.Context(), eventType, payload); err != nil {
		h.handleServiceError(w, err, "Failed to process GitHub webhook")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"received":   true,
		"event_type": eventType,
	})
}

// verifyGitHubSignature verifies the GitHub webhook signature.
func (h *WebhookHandler) verifyGitHubSignature(signature string, body []byte) bool {
	if h.config.GitHubWebhookSecret == "" {
		return true
	}
	signature = strings.TrimPrefix(signature, "sha256=")
	if signature == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(h.config.GitHubWebhookSecret))
	mac.Write(body)
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expectedSig))
}

// ======================================================================
// SendGrid Webhook
// ======================================================================

// HandleSendGridWebhook handles incoming SendGrid webhook events.
// @Summary Handle SendGrid webhook
// @Description Processes incoming SendGrid webhook events for email delivery status
// @Tags webhooks
// @Accept json
// @Produce json
// @Param payload body object true "SendGrid webhook payload"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /webhooks/sendgrid [post]
func (h *WebhookHandler) HandleSendGridWebhook(w http.ResponseWriter, r *http.Request) {
	// Verify SendGrid signature (if configured)
	if h.config.SendGridWebhookSecret != "" {
		signature := r.Header.Get("X-Twilio-Email-Event-Webhook-Signature")
		if !h.verifySendGridSignature(signature, r) {
			h.sendError(w, http.StatusUnauthorized, "Invalid SendGrid signature", nil)
			return
		}
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Failed to read request body", nil)
		return
	}

	var events []map[string]interface{}
	if err := json.Unmarshal(body, &events); err != nil {
		// Try single event
		var event map[string]interface{}
		if err := json.Unmarshal(body, &event); err != nil {
			h.sendError(w, http.StatusBadRequest, "Invalid webhook payload", nil)
			return
		}
		events = []map[string]interface{}{event}
	}

	for _, event := range events {
		eventType, _ := event["event"].(string)
		email, _ := event["email"].(string)
		status, _ := event["status"].(string)
		reason, _ := event["reason"].(string)
		_ = h.webhookService.ProcessSendGridWebhook(r.Context(), eventType, email, status, reason)
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"received": true,
		"count":    len(events),
	})
}

// verifySendGridSignature verifies the SendGrid webhook signature.
func (h *WebhookHandler) verifySendGridSignature(signature string, r *http.Request) bool {
	if h.config.SendGridWebhookSecret == "" || signature == "" {
		return true
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return false
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	mac := hmac.New(sha256.New, []byte(h.config.SendGridWebhookSecret))
	mac.Write(body)
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expectedSig))
}

// ======================================================================
= Generic Webhook
// ======================================================================

// HandleGenericWebhook handles incoming generic webhook events.
// @Summary Handle generic webhook
// @Description Processes incoming generic webhook events
// @Tags webhooks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param provider path string true "Webhook provider name"
// @Param payload body object true "Webhook payload"
// @Success 200 {object} dto.SuccessResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /webhooks/generic/{provider} [post]
func (h *WebhookHandler) HandleGenericWebhook(w http.ResponseWriter, r *http.Request) {
	// Verify API key
	apiKey := r.Header.Get("X-Webhook-API-Key")
	if apiKey == "" {
		apiKey = r.URL.Query().Get("api_key")
	}
	if apiKey == "" || !h.validateWebhookAPIKey(apiKey) {
		h.sendError(w, http.StatusUnauthorized, "Invalid or missing API key", nil)
		return
	}

	vars := mux.Vars(r)
	provider := vars["provider"]
	if provider == "" {
		h.sendError(w, http.StatusBadRequest, "Provider name is required", nil)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Failed to read request body", nil)
		return
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid webhook payload", nil)
		return
	}

	if err := h.webhookService.ProcessGenericWebhook(r.Context(), provider, payload); err != nil {
		h.handleServiceError(w, err, "Failed to process webhook")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"received": true,
		"provider": provider,
	})
}

// validateWebhookAPIKey validates the webhook API key.
func (h *WebhookHandler) validateWebhookAPIKey(apiKey string) bool {
	if h.config.WebhookAPIKey == "" {
		return true
	}
	return apiKey == h.config.WebhookAPIKey
}

// ======================================================================
= Admin Webhook Management
// ======================================================================

// AdminListWebhooks handles admin listing of all webhooks.
// @Summary Admin list webhooks
// @Description Lists all registered webhooks (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param provider query string false "Filter by provider"
// @Param status query string false "Filter by status (active, inactive)"
// @Success 200 {object} dto.WebhookListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/webhooks [get]
func (h *WebhookHandler) AdminListWebhooks(w http.ResponseWriter, r *http.Request) {
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
	provider := r.URL.Query().Get("provider")
	status := r.URL.Query().Get("status")

	webhooks, nextCursor, total, err := h.webhookService.AdminListWebhooks(r.Context(), cursor, limit, provider, status)
	if err != nil {
		h.handleServiceError(w, err, "Failed to list webhooks")
		return
	}

	responses := make([]*dto.WebhookAdminResponse, 0, len(webhooks))
	for _, w := range webhooks {
		responses = append(responses, &dto.WebhookAdminResponse{
			ID:          w.ID,
			URL:         w.URL,
			Provider:    w.Provider,
			Status:      w.Status,
			Events:      w.Events,
			CreatedAt:   w.CreatedAt,
			UpdatedAt:   w.UpdatedAt,
			LastDelivery: w.LastDelivery,
			DeliveryCount: w.DeliveryCount,
			SuccessCount: w.SuccessCount,
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

// AdminGetWebhookDetails handles retrieving webhook details.
// @Summary Admin get webhook details
// @Description Retrieves detailed information about a specific webhook (admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "Webhook ID"
// @Success 200 {object} dto.WebhookDetailResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/webhooks/{id} [get]
func (h *WebhookHandler) AdminGetWebhookDetails(w http.ResponseWriter, r *http.Request) {
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	webhookID := vars["id"]
	if webhookID == "" {
		h.sendError(w, http.StatusBadRequest, "Webhook ID required", nil)
		return
	}

	details, err := h.webhookService.AdminGetWebhookDetails(r.Context(), webhookID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get webhook details")
		return
	}

	h.sendSuccess(w, http.StatusOK, details)
}

// AdminCreateWebhook handles creating a new webhook.
// @Summary Admin create webhook
// @Description Creates a new webhook endpoint (admin only)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.CreateWebhookRequest true "Webhook details"
// @Success 201 {object} dto.WebhookResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/webhooks [post]
func (h *WebhookHandler) AdminCreateWebhook(w http.ResponseWriter, r *http.Request) {
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	var req dto.CreateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}
	req.Sanitize()
	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	webhook, err := h.webhookService.AdminCreateWebhook(r.Context(), &req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to create webhook")
		return
	}

	h.sendSuccess(w, http.StatusCreated, webhook)
}

// AdminUpdateWebhook handles updating a webhook.
// @Summary Admin update webhook
// @Description Updates an existing webhook (admin only)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Webhook ID"
// @Param request body dto.UpdateWebhookRequest true "Webhook updates"
// @Success 200 {object} dto.WebhookResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/webhooks/{id} [put]
func (h *WebhookHandler) AdminUpdateWebhook(w http.ResponseWriter, r *http.Request) {
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	webhookID := vars["id"]
	if webhookID == "" {
		h.sendError(w, http.StatusBadRequest, "Webhook ID required", nil)
		return
	}

	var req dto.UpdateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}
	req.Sanitize()
	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	webhook, err := h.webhookService.AdminUpdateWebhook(r.Context(), webhookID, &req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to update webhook")
		return
	}

	h.sendSuccess(w, http.StatusOK, webhook)
}

// AdminDeleteWebhook handles deleting a webhook.
// @Summary Admin delete webhook
// @Description Deletes a webhook endpoint (admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "Webhook ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/webhooks/{id} [delete]
func (h *WebhookHandler) AdminDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	webhookID := vars["id"]
	if webhookID == "" {
		h.sendError(w, http.StatusBadRequest, "Webhook ID required", nil)
		return
	}

	if err := h.webhookService.AdminDeleteWebhook(r.Context(), webhookID); err != nil {
		h.handleServiceError(w, err, "Failed to delete webhook")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Webhook deleted successfully",
	})
}

// AdminRegenerateWebhookSecret handles regenerating a webhook secret.
// @Summary Admin regenerate webhook secret
// @Description Regenerates the secret for a webhook endpoint (admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "Webhook ID"
// @Success 200 {object} dto.WebhookSecretResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/webhooks/{id}/regenerate-secret [post]
func (h *WebhookHandler) AdminRegenerateWebhookSecret(w http.ResponseWriter, r *http.Request) {
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	webhookID := vars["id"]
	if webhookID == "" {
		h.sendError(w, http.StatusBadRequest, "Webhook ID required", nil)
		return
	}

	secret, err := h.webhookService.AdminRegenerateWebhookSecret(r.Context(), webhookID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to regenerate webhook secret")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"secret":  secret,
		"message": "Webhook secret regenerated successfully",
	})
}

// AdminGetWebhookStats handles retrieving webhook statistics.
// @Summary Admin get webhook stats
// @Description Retrieves global webhook statistics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Success 200 {object} dto.WebhookStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/webhooks/stats [get]
func (h *WebhookHandler) AdminGetWebhookStats(w http.ResponseWriter, r *http.Request) {
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 || days > 30 {
		days = 7
	}

	stats, err := h.webhookService.AdminGetWebhookStats(r.Context(), days)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get webhook stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// AdminTestWebhook handles testing a webhook endpoint.
// @Summary Admin test webhook
// @Description Sends a test event to a webhook endpoint (admin only)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.TestWebhookRequest true "Test details"
// @Success 200 {object} dto.TestWebhookResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/webhooks/test [post]
func (h *WebhookHandler) AdminTestWebhook(w http.ResponseWriter, r *http.Request) {
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	var req dto.TestWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}
	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	result, err := h.webhookService.TestWebhook(r.Context(), req.URL, req.Payload, req.Headers)
	if err != nil {
		h.handleServiceError(w, err, "Failed to test webhook")
		return
	}

	h.sendSuccess(w, http.StatusOK, result)
}

// ======================================================================
= Helper Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *WebhookHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *WebhookHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *WebhookHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *WebhookHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrWebhookNotFound):
		h.sendError(w, http.StatusNotFound, "Webhook not found", nil)
	case errors.Is(err, service.ErrWebhookAlreadyExists):
		h.sendError(w, http.StatusConflict, "Webhook already exists", nil)
	case errors.Is(err, service.ErrInvalidWebhookURL):
		h.sendError(w, http.StatusBadRequest, "Invalid webhook URL", nil)
	case errors.Is(err, service.ErrInvalidWebhookProvider):
		h.sendError(w, http.StatusBadRequest, "Invalid webhook provider", nil)
	case errors.Is(err, service.ErrWebhookVerificationFailed):
		h.sendError(w, http.StatusUnauthorized, "Webhook verification failed", nil)
	case errors.Is(err, service.ErrWebhookDeliveryFailed):
		h.sendError(w, http.StatusBadRequest, "Webhook delivery failed", nil)
	case errors.Is(err, service.ErrWebhookRetryLimitExceeded):
		h.sendError(w, http.StatusBadRequest, "Webhook retry limit exceeded", nil)
	case errors.Is(err, service.ErrWebhookSecretGenerationFailed):
		h.sendError(w, http.StatusInternalServerError, "Failed to generate webhook secret", nil)
	case errors.Is(err, service.ErrWebhookTestFailed):
		h.sendError(w, http.StatusInternalServerError, "Webhook test failed", nil)
	case errors.Is(err, service.ErrInvalidWebhookEvent):
		h.sendError(w, http.StatusBadRequest, "Invalid webhook event", nil)
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

// HealthCheck returns the health status of the webhook handler.
func (h *WebhookHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "webhook_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}