package webhook

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/internal/update"
)

// Handler handles webhook requests
type Handler struct {
	updateManager *update.Manager
	secret        string // Optional webhook secret for validation
}

// NewHandler creates a new webhook handler
func NewHandler(updateMgr *update.Manager, secret string) *Handler {
	return &Handler{
		updateManager: updateMgr,
		secret:        secret,
	}
}

// GitHubWebhookPayload represents a GitHub webhook payload
type GitHubWebhookPayload struct {
	Action      string `json:"action"`
	Release     Release `json:"release"`
	Repository  Repository `json:"repository"`
}

type Release struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Prerelease  bool   `json:"prerelease"`
	Draft       bool   `json:"draft"`
}

type Repository struct {
	FullName string `json:"full_name"`
	Name     string `json:"name"`
}

// HandleGitHubWebhook handles GitHub webhook requests
func (h *Handler) HandleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Validate webhook signature if secret is set
	// TODO: Implement GitHub webhook signature validation

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer r.Body.Close()

	// Parse webhook payload
	var payload GitHubWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	// Only process release events
	if payload.Action != "published" && payload.Action != "released" {
		log.Printf("Ignoring webhook action: %s", payload.Action)
		h.writeJSON(w, http.StatusOK, map[string]string{
			"message": "ignored",
			"action":  payload.Action,
		})
		return
	}

	// Ignore drafts and prereleases
	if payload.Release.Draft || payload.Release.Prerelease {
		log.Printf("Ignoring draft/prerelease: %s", payload.Release.TagName)
		h.writeJSON(w, http.StatusOK, map[string]string{
			"message": "ignored draft/prerelease",
		})
		return
	}

	log.Printf("Received GitHub webhook for %s: %s",
		payload.Repository.FullName, payload.Release.TagName)

	// Find matching process by repository
	// TODO: Need to match repository to process name
	// For now, trigger update for all processes that match the repository

	response := map[string]interface{}{
		"message":    "webhook received",
		"repository": payload.Repository.FullName,
		"version":    payload.Release.TagName,
	}

	h.writeJSON(w, http.StatusAccepted, response)
}

// CloudflareWebhookPayload represents a custom webhook from github-webhook-worker
type CloudflareWebhookPayload struct {
	ProcessName string `json:"process_name"`
	Repository  string `json:"repository"`
	Version     string `json:"version"`
	Action      string `json:"action"` // "update" or "rollback"
}

// HandleCloudflareWebhook handles webhooks from github-webhook-worker
func (h *Handler) HandleCloudflareWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer r.Body.Close()

	// Parse webhook payload
	var payload CloudflareWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	log.Printf("Received Cloudflare webhook for %s: action=%s, version=%s",
		payload.ProcessName, payload.Action, payload.Version)

	// Handle action
	switch payload.Action {
	case "update":
		// Trigger update
		if err := h.updateManager.UpdateProcess(payload.ProcessName, payload.Version, false); err != nil {
			h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to trigger update: %v", err))
			return
		}

		h.writeJSON(w, http.StatusAccepted, map[string]interface{}{
			"message": "update triggered",
			"process": payload.ProcessName,
			"version": payload.Version,
		})

	case "rollback":
		// Trigger rollback
		if err := h.updateManager.RollbackProcess(payload.ProcessName, payload.Version); err != nil {
			h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to trigger rollback: %v", err))
			return
		}

		h.writeJSON(w, http.StatusAccepted, map[string]interface{}{
			"message": "rollback triggered",
			"process": payload.ProcessName,
			"version": payload.Version,
		})

	default:
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown action: %s", payload.Action))
	}
}

// writeJSON writes a JSON response
func (h *Handler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response
func (h *Handler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]interface{}{
		"error": message,
	})
}
