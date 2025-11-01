package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/yhonda-ohishi-pub-dev/gowinproc/src/pkg/models"
)

// Client handles GitHub API interactions
type Client struct {
	token      string
	httpClient *http.Client
}

// NewClient creates a new GitHub client
func NewClient(token string) *Client {
	return &Client{
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// githubRelease represents a GitHub release response
type githubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

// GetLatestRelease fetches the latest release for a repository
func (c *Client) GetLatestRelease(repo string) (*models.Version, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github API error: %s - %s", resp.Status, string(body))
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return c.convertRelease(&release), nil
}

// GetRelease fetches a specific release by tag
func (c *Client) GetRelease(repo, tag string) (*models.Version, error) {
	// Special case: "latest" means get the latest release
	if tag == "latest" {
		return c.GetLatestRelease(repo)
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", repo, tag)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github API error: %s - %s", resp.Status, string(body))
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return c.convertRelease(&release), nil
}

// ListReleases lists all releases for a repository
func (c *Client) ListReleases(repo string, limit int) ([]models.Version, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=%d", repo, limit)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github API error: %s - %s", resp.Status, string(body))
	}

	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	versions := make([]models.Version, 0, len(releases))
	for i := range releases {
		versions = append(versions, *c.convertRelease(&releases[i]))
	}

	return versions, nil
}

// DownloadAsset downloads a release asset to the specified path
func (c *Client) DownloadAsset(assetURL, destPath string, progressCallback func(downloaded, total int64)) error {
	req, err := http.NewRequest("GET", assetURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	}
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download asset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	// Create destination directory
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create destination file
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Download with progress tracking
	total := resp.ContentLength
	downloaded := int64(0)

	buffer := make([]byte, 32*1024) // 32KB buffer
	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := out.Write(buffer[:n]); writeErr != nil {
				return fmt.Errorf("failed to write file: %w", writeErr)
			}
			downloaded += int64(n)
			if progressCallback != nil {
				progressCallback(downloaded, total)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}
	}

	return nil
}

// convertRelease converts a GitHub release to our Version model
func (c *Client) convertRelease(release *githubRelease) *models.Version {
	version := &models.Version{
		Tag:          release.TagName,
		ReleaseURL:   release.HTMLURL,
		PublishedAt:  release.PublishedAt,
		IsPrerelease: release.Prerelease,
	}

	// Find the first Windows executable asset
	for _, asset := range release.Assets {
		if filepath.Ext(asset.Name) == ".exe" {
			version.AssetURL = asset.BrowserDownloadURL
			version.AssetName = asset.Name
			version.Size = asset.Size
			break
		}
	}

	return version
}
