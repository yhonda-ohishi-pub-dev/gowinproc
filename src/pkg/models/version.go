package models

import "time"

// Version represents a version of a process binary
type Version struct {
	Tag         string    `json:"tag"`
	CommitSHA   string    `json:"commit_sha"`
	ReleaseURL  string    `json:"release_url"`
	AssetURL    string    `json:"asset_url"`
	AssetName   string    `json:"asset_name"`
	Size        int64     `json:"size"`
	PublishedAt time.Time `json:"published_at"`
	IsPrerelease bool     `json:"is_prerelease"`
}

// VersionInfo represents version information for a process
type VersionInfo struct {
	ProcessName    string    `json:"process_name"`
	CurrentVersion *Version  `json:"current_version"`
	LatestVersion  *Version  `json:"latest_version"`
	UpdateAvailable bool     `json:"update_available"`
	History        []Version `json:"history,omitempty"`
}

// UpdateRequest represents a request to update a process
type UpdateRequest struct {
	ProcessName string `json:"process_name"`
	Version     string `json:"version,omitempty"` // If empty, update to latest
	Force       bool   `json:"force"`
}

// UpdateStatus represents the status of an update operation
type UpdateStatus struct {
	ProcessName string  `json:"process_name"`
	Stage       string  `json:"stage"`
	Progress    float64 `json:"progress"`
	Message     string  `json:"message"`
	Error       string  `json:"error,omitempty"`
	Completed   bool    `json:"completed"`
}

// RollbackRequest represents a request to rollback a process
type RollbackRequest struct {
	ProcessName string `json:"process_name"`
	Version     string `json:"version,omitempty"` // If empty, rollback to previous version
}
