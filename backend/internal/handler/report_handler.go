// backend/internal/handler/report_handler.go
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

// ReportHandler handles all report-related HTTP endpoints.
type ReportHandler struct {
	reportService service.ReportService
	tweetService  service.TweetService
	userService   service.UserService
	log           *logrus.Entry
}

// NewReportHandler creates a new report handler.
func NewReportHandler(
	reportService service.ReportService,
	tweetService service.TweetService,
	userService service.UserService,
) *ReportHandler {
	return &ReportHandler{
		reportService: reportService,
		tweetService:  tweetService,
		userService:   userService,
		log:           logger.WithField("handler", "report"),
	}
}

// ======================================================================
// Create Report
// ======================================================================

// CreateReport handles creating a new report.
// @Summary Create report
// @Description Reports a tweet or user
// @Tags reports
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.CreateReportRequest true "Report details"
// @Success 201 {object} dto.ReportResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/reports [post]
func (h *ReportHandler) CreateReport(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	var req dto.CreateReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	req.Sanitize()
	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	// Validate target exists
	if req.TargetType == "tweet" {
		_, err := h.tweetService.GetTweetByID(r.Context(), req.TargetID)
		if err != nil {
			h.sendError(w, http.StatusNotFound, "Tweet not found", nil)
			return
		}
	} else if req.TargetType == "user" {
		_, err := h.userService.GetUserByID(r.Context(), req.TargetID)
		if err != nil {
			h.sendError(w, http.StatusNotFound, "User not found", nil)
			return
		}
	} else {
		h.sendError(w, http.StatusBadRequest, "Invalid target type. Must be 'tweet' or 'user'", nil)
		return
	}

	// Check duplicate
	exists, err := h.reportService.CheckDuplicate(r.Context(), userID, req.TargetID, req.TargetType)
	if err != nil {
		h.handleServiceError(w, err, "Failed to check duplicate")
		return
	}
	if exists {
		h.sendError(w, http.StatusConflict, "You have already reported this content", nil)
		return
	}

	report, err := h.reportService.CreateReport(r.Context(), userID, &req)
	if err != nil {
		h.handleServiceError(w, err, "Failed to create report")
		return
	}

	h.sendSuccess(w, http.StatusCreated, report)
}

// ======================================================================
= Get My Reports
// ======================================================================

// GetMyReports handles retrieving reports filed by the authenticated user.
// @Summary Get my reports
// @Description Retrieves reports filed by the authenticated user
// @Tags reports
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param status query string false "Filter by status"
// @Param target_type query string false "Filter by target type"
// @Success 200 {object} dto.ReportListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/reports/mine [get]
func (h *ReportHandler) GetMyReports(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	status := r.URL.Query().Get("status")
	targetType := r.URL.Query().Get("target_type")

	reports, nextCursor, total, err := h.reportService.GetMyReports(r.Context(), userID, cursor, limit, status, targetType)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get reports")
		return
	}

	// Convert to response
	responses := make([]*dto.ReportResponse, 0, len(reports))
	for _, rpt := range reports {
		responses = append(responses, &dto.ReportResponse{
			ID:          rpt.ID,
			ReporterID:  rpt.ReporterID,
			TargetID:    rpt.TargetID,
			TargetType:  rpt.TargetType,
			Reason:      rpt.Reason,
			Description: rpt.Description,
			Status:      rpt.Status,
			Severity:    rpt.Severity,
			ReviewerID:  rpt.ReviewerID,
			ReviewNotes: rpt.ReviewNotes,
			CreatedAt:   rpt.CreatedAt,
			UpdatedAt:   rpt.UpdatedAt,
			ResolvedAt:  rpt.ResolvedAt,
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

// ======================================================================
= Get Report by ID
// ======================================================================

// GetReport handles retrieving a report by ID.
// @Summary Get report
// @Description Retrieves a report by its ID
// @Tags reports
// @Security BearerAuth
// @Produce json
// @Param id path string true "Report ID"
// @Success 200 {object} dto.ReportDetailResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/reports/{id} [get]
func (h *ReportHandler) GetReport(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	vars := mux.Vars(r)
	reportID := vars["id"]
	if reportID == "" {
		h.sendError(w, http.StatusBadRequest, "Report ID required", nil)
		return
	}

	// Check permission (admin or reporter)
	role, _ := middleware.GetUserRole(r.Context())
	isAdmin := role == "admin" || role == "moderator"

	report, err := h.reportService.GetReport(r.Context(), reportID, userID, isAdmin)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get report")
		return
	}

	// Get reporter info
	reporter, _ := h.userService.GetUserByID(r.Context(), report.ReporterID)
	reviewer, _ := h.userService.GetUserByID(r.Context(), report.ReviewerID)

	detailResponse := &dto.ReportDetailResponse{
		Report: &dto.ReportResponse{
			ID:          report.ID,
			ReporterID:  report.ReporterID,
			TargetID:    report.TargetID,
			TargetType:  report.TargetType,
			Reason:      report.Reason,
			Description: report.Description,
			Status:      report.Status,
			Severity:    report.Severity,
			ReviewerID:  report.ReviewerID,
			ReviewNotes: report.ReviewNotes,
			CreatedAt:   report.CreatedAt,
			UpdatedAt:   report.UpdatedAt,
			ResolvedAt:  report.ResolvedAt,
		},
		ReporterUsername: func() string {
			if reporter != nil {
				return reporter.Username
			}
			return ""
		}(),
		ReviewerUsername: func() string {
			if reviewer != nil {
				return reviewer.Username
			}
			return ""
		}(),
	}

	h.sendSuccess(w, http.StatusOK, detailResponse)
}

// ======================================================================
= Update Report Status
// ======================================================================

// UpdateReportStatus handles updating a report's status.
// @Summary Update report status
// @Description Updates the status of a report (admin/moderator only)
// @Tags reports
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Report ID"
// @Param request body dto.UpdateReportStatusRequest true "Status update"
// @Success 200 {object} dto.ReportResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/reports/{id}/status [put]
func (h *ReportHandler) UpdateReportStatus(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	// Check admin/moderator role
	role, _ := middleware.GetUserRole(r.Context())
	if role != "admin" && role != "moderator" {
		h.sendError(w, http.StatusForbidden, "Admin or moderator access required", nil)
		return
	}

	vars := mux.Vars(r)
	reportID := vars["id"]
	if reportID == "" {
		h.sendError(w, http.StatusBadRequest, "Report ID required", nil)
		return
	}

	var req dto.UpdateReportStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	report, err := h.reportService.UpdateReportStatus(r.Context(), reportID, userID, req.Status, req.Notes)
	if err != nil {
		h.handleServiceError(w, err, "Failed to update report status")
		return
	}

	// Convert to response
	resp := &dto.ReportResponse{
		ID:          report.ID,
		ReporterID:  report.ReporterID,
		TargetID:    report.TargetID,
		TargetType:  report.TargetType,
		Reason:      report.Reason,
		Description: report.Description,
		Status:      report.Status,
		Severity:    report.Severity,
		ReviewerID:  report.ReviewerID,
		ReviewNotes: report.ReviewNotes,
		CreatedAt:   report.CreatedAt,
		UpdatedAt:   report.UpdatedAt,
		ResolvedAt:  report.ResolvedAt,
	}

	h.sendSuccess(w, http.StatusOK, resp)
}

// ======================================================================
= Update Report Severity
// ======================================================================

// UpdateReportSeverity handles updating a report's severity.
// @Summary Update report severity
// @Description Updates the severity of a report (admin only)
// @Tags reports
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Report ID"
// @Param request body dto.UpdateReportSeverityRequest true "Severity update"
// @Success 200 {object} dto.ReportResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/reports/{id}/severity [put]
func (h *ReportHandler) UpdateReportSeverity(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	// Check admin role
	role, _ := middleware.GetUserRole(r.Context())
	if role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	reportID := vars["id"]
	if reportID == "" {
		h.sendError(w, http.StatusBadRequest, "Report ID required", nil)
		return
	}

	var req dto.UpdateReportSeverityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	report, err := h.reportService.UpdateReportSeverity(r.Context(), reportID, userID, req.Severity)
	if err != nil {
		h.handleServiceError(w, err, "Failed to update report severity")
		return
	}

	resp := &dto.ReportResponse{
		ID:          report.ID,
		ReporterID:  report.ReporterID,
		TargetID:    report.TargetID,
		TargetType:  report.TargetType,
		Reason:      report.Reason,
		Description: report.Description,
		Status:      report.Status,
		Severity:    report.Severity,
		ReviewerID:  report.ReviewerID,
		ReviewNotes: report.ReviewNotes,
		CreatedAt:   report.CreatedAt,
		UpdatedAt:   report.UpdatedAt,
		ResolvedAt:  report.ResolvedAt,
	}

	h.sendSuccess(w, http.StatusOK, resp)
}

// ======================================================================
= Assign Report
// ======================================================================

// AssignReport handles assigning a report to a reviewer.
// @Summary Assign report
// @Description Assigns a report to a reviewer (admin only)
// @Tags reports
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Report ID"
// @Param request body dto.AssignReportRequest true "Assignment details"
// @Success 200 {object} dto.ReportResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/reports/{id}/assign [post]
func (h *ReportHandler) AssignReport(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	// Check admin role
	role, _ := middleware.GetUserRole(r.Context())
	if role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	reportID := vars["id"]
	if reportID == "" {
		h.sendError(w, http.StatusBadRequest, "Report ID required", nil)
		return
	}

	var req dto.AssignReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	// Verify reviewer exists
	_, err = h.userService.GetUserByID(r.Context(), req.ReviewerID)
	if err != nil {
		h.sendError(w, http.StatusNotFound, "Reviewer user not found", nil)
		return
	}

	report, err := h.reportService.AssignReport(r.Context(), reportID, userID, req.ReviewerID)
	if err != nil {
		h.handleServiceError(w, err, "Failed to assign report")
		return
	}

	resp := &dto.ReportResponse{
		ID:          report.ID,
		ReporterID:  report.ReporterID,
		TargetID:    report.TargetID,
		TargetType:  report.TargetType,
		Reason:      report.Reason,
		Description: report.Description,
		Status:      report.Status,
		Severity:    report.Severity,
		ReviewerID:  report.ReviewerID,
		ReviewNotes: report.ReviewNotes,
		CreatedAt:   report.CreatedAt,
		UpdatedAt:   report.UpdatedAt,
		ResolvedAt:  report.ResolvedAt,
	}

	h.sendSuccess(w, http.StatusOK, resp)
}

// ======================================================================
= Resolve Report
// ======================================================================

// ResolveReport handles resolving a report.
// @Summary Resolve report
// @Description Resolves a report with action taken (admin/moderator only)
// @Tags reports
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Report ID"
// @Param request body dto.ResolveReportRequest true "Resolution details"
// @Success 200 {object} dto.ReportResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/reports/{id}/resolve [post]
func (h *ReportHandler) ResolveReport(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	// Check admin/moderator role
	role, _ := middleware.GetUserRole(r.Context())
	if role != "admin" && role != "moderator" {
		h.sendError(w, http.StatusForbidden, "Admin or moderator access required", nil)
		return
	}

	vars := mux.Vars(r)
	reportID := vars["id"]
	if reportID == "" {
		h.sendError(w, http.StatusBadRequest, "Report ID required", nil)
		return
	}

	var req dto.ResolveReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	report, err := h.reportService.ResolveReport(r.Context(), reportID, userID, req.Action, req.Notes)
	if err != nil {
		h.handleServiceError(w, err, "Failed to resolve report")
		return
	}

	// If action involves deleting tweet, perform it
	if req.Action == "delete_tweet" && report.TargetType == "tweet" {
		_ = h.tweetService.DeleteTweet(r.Context(), report.TargetID, userID)
	}

	resp := &dto.ReportResponse{
		ID:          report.ID,
		ReporterID:  report.ReporterID,
		TargetID:    report.TargetID,
		TargetType:  report.TargetType,
		Reason:      report.Reason,
		Description: report.Description,
		Status:      report.Status,
		Severity:    report.Severity,
		ReviewerID:  report.ReviewerID,
		ReviewNotes: report.ReviewNotes,
		CreatedAt:   report.CreatedAt,
		UpdatedAt:   report.UpdatedAt,
		ResolvedAt:  report.ResolvedAt,
	}

	h.sendSuccess(w, http.StatusOK, resp)
}

// ======================================================================
= Dismiss Report
// ======================================================================

// DismissReport handles dismissing a report.
// @Summary Dismiss report
// @Description Dismisses a report with notes (admin/moderator only)
// @Tags reports
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Report ID"
// @Param request body dto.DismissReportRequest true "Dismissal details"
// @Success 200 {object} dto.ReportResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/reports/{id}/dismiss [post]
func (h *ReportHandler) DismissReport(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.GetUserID(r.Context())
	if err != nil {
		h.sendError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	// Check admin/moderator role
	role, _ := middleware.GetUserRole(r.Context())
	if role != "admin" && role != "moderator" {
		h.sendError(w, http.StatusForbidden, "Admin or moderator access required", nil)
		return
	}

	vars := mux.Vars(r)
	reportID := vars["id"]
	if reportID == "" {
		h.sendError(w, http.StatusBadRequest, "Report ID required", nil)
		return
	}

	var req dto.DismissReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if err := req.Validate(); err != nil {
		h.sendValidationError(w, err)
		return
	}

	report, err := h.reportService.DismissReport(r.Context(), reportID, userID, req.Notes)
	if err != nil {
		h.handleServiceError(w, err, "Failed to dismiss report")
		return
	}

	resp := &dto.ReportResponse{
		ID:          report.ID,
		ReporterID:  report.ReporterID,
		TargetID:    report.TargetID,
		TargetType:  report.TargetType,
		Reason:      report.Reason,
		Description: report.Description,
		Status:      report.Status,
		Severity:    report.Severity,
		ReviewerID:  report.ReviewerID,
		ReviewNotes: report.ReviewNotes,
		CreatedAt:   report.CreatedAt,
		UpdatedAt:   report.UpdatedAt,
		ResolvedAt:  report.ResolvedAt,
	}

	h.sendSuccess(w, http.StatusOK, resp)
}

// ======================================================================
= Admin List Reports
// ======================================================================

// AdminListReports handles admin listing of all reports.
// @Summary Admin list reports
// @Description Lists all reports with pagination and filters (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Items per page (default 20, max 100)"
// @Param status query string false "Filter by status"
// @Param severity query string false "Filter by severity"
// @Param target_type query string false "Filter by target type"
// @Param search query string false "Search by reporter or target"
// @Success 200 {object} dto.ReportListResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/reports [get]
func (h *ReportHandler) AdminListReports(w http.ResponseWriter, r *http.Request) {
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
	severity := r.URL.Query().Get("severity")
	targetType := r.URL.Query().Get("target_type")
	search := r.URL.Query().Get("search")

	reports, nextCursor, total, err := h.reportService.AdminListReports(r.Context(), cursor, limit, status, severity, targetType, search)
	if err != nil {
		h.handleServiceError(w, err, "Failed to list reports")
		return
	}

	// Convert to response
	responses := make([]*dto.ReportAdminResponse, 0, len(reports))
	for _, rpt := range reports {
		reporter, _ := h.userService.GetUserByID(r.Context(), rpt.ReporterID)
		reviewer, _ := h.userService.GetUserByID(r.Context(), rpt.ReviewerID)
		responses = append(responses, &dto.ReportAdminResponse{
			ID:               rpt.ID,
			ReporterID:       rpt.ReporterID,
			TargetID:         rpt.TargetID,
			TargetType:       rpt.TargetType,
			Reason:           rpt.Reason,
			Description:      rpt.Description,
			Status:           rpt.Status,
			Severity:         rpt.Severity,
			ReviewerID:       rpt.ReviewerID,
			ReviewNotes:      rpt.ReviewNotes,
			CreatedAt:        rpt.CreatedAt,
			UpdatedAt:        rpt.UpdatedAt,
			ResolvedAt:       rpt.ResolvedAt,
			ReporterUsername: func() string {
				if reporter != nil {
					return reporter.Username
				}
				return ""
			}(),
			ReviewerUsername: func() string {
				if reviewer != nil {
					return reviewer.Username
				}
				return ""
			}(),
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

// ======================================================================
= Admin Get Report Stats
// ======================================================================

// AdminGetReportStats handles retrieving global report statistics.
// @Summary Admin get report stats
// @Description Retrieves global report statistics (admin only)
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param days query int false "Number of days to analyze (default 7, max 30)"
// @Success 200 {object} dto.ReportStatsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/reports/stats [get]
func (h *ReportHandler) AdminGetReportStats(w http.ResponseWriter, r *http.Request) {
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

	stats, err := h.reportService.AdminGetReportStats(r.Context(), days)
	if err != nil {
		h.handleServiceError(w, err, "Failed to get report stats")
		return
	}

	h.sendSuccess(w, http.StatusOK, stats)
}

// ======================================================================
= Admin Delete Report
// ======================================================================

// AdminDeleteReport handles admin deletion of a report.
// @Summary Admin delete report
// @Description Deletes a report (admin only)
// @Tags admin
// @Security BearerAuth
// @Param id path string true "Report ID"
// @Success 200 {object} dto.SuccessResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/admin/reports/{id} [delete]
func (h *ReportHandler) AdminDeleteReport(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, err := middleware.GetUserRole(r.Context())
	if err != nil || role != "admin" {
		h.sendError(w, http.StatusForbidden, "Admin access required", nil)
		return
	}

	vars := mux.Vars(r)
	reportID := vars["id"]
	if reportID == "" {
		h.sendError(w, http.StatusBadRequest, "Report ID required", nil)
		return
	}

	if err := h.reportService.AdminDeleteReport(r.Context(), reportID); err != nil {
		h.handleServiceError(w, err, "Failed to delete report")
		return
	}

	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Report deleted successfully",
	})
}

// ======================================================================
= Helper Methods
// ======================================================================

// sendSuccess writes a success response.
func (h *ReportHandler) sendSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.WithError(err).Error("Failed to encode success response")
	}
}

// sendError writes an error response.
func (h *ReportHandler) sendError(w http.ResponseWriter, status int, message string, details interface{}) {
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
func (h *ReportHandler) sendValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(dto.ValidationErrors); ok {
		h.sendError(w, http.StatusBadRequest, "Validation failed", ve.ToMap())
		return
	}
	h.sendError(w, http.StatusBadRequest, err.Error(), nil)
}

// handleServiceError maps service errors to HTTP responses.
func (h *ReportHandler) handleServiceError(w http.ResponseWriter, err error, defaultMsg string) {
	switch {
	case errors.Is(err, service.ErrReportNotFound):
		h.sendError(w, http.StatusNotFound, "Report not found", nil)
	case errors.Is(err, service.ErrReportDuplicate):
		h.sendError(w, http.StatusConflict, "Duplicate report", nil)
	case errors.Is(err, service.ErrReportAlreadyResolved):
		h.sendError(w, http.StatusBadRequest, "Report already resolved", nil)
	case errors.Is(err, service.ErrReportAlreadyDismissed):
		h.sendError(w, http.StatusBadRequest, "Report already dismissed", nil)
	case errors.Is(err, service.ErrInvalidReportStatus):
		h.sendError(w, http.StatusBadRequest, "Invalid report status", nil)
	case errors.Is(err, service.ErrInvalidReportSeverity):
		h.sendError(w, http.StatusBadRequest, "Invalid report severity", nil)
	case errors.Is(err, service.ErrInvalidReporterID):
		h.sendError(w, http.StatusBadRequest, "Invalid reporter ID", nil)
	case errors.Is(err, service.ErrInvalidTargetID):
		h.sendError(w, http.StatusBadRequest, "Invalid target ID", nil)
	case errors.Is(err, service.ErrInvalidTargetType):
		h.sendError(w, http.StatusBadRequest, "Invalid target type", nil)
	case errors.Is(err, service.ErrReportPermissionDenied):
		h.sendError(w, http.StatusForbidden, "Permission denied", nil)
	case errors.Is(err, service.ErrReportAlreadyAssigned):
		h.sendError(w, http.StatusBadRequest, "Report already assigned", nil)
	case errors.Is(err, service.ErrInvalidReviewerID):
		h.sendError(w, http.StatusBadRequest, "Invalid reviewer ID", nil)
	case errors.Is(err, service.ErrUserNotFound):
		h.sendError(w, http.StatusNotFound, "User not found", nil)
	case errors.Is(err, service.ErrTweetNotFound):
		h.sendError(w, http.StatusNotFound, "Tweet not found", nil)
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

// HealthCheck returns the health status of the report handler.
func (h *ReportHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.sendSuccess(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"component": "report_handler",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}