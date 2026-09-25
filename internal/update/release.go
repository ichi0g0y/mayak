// Package update keeps MAYAK current from its GitHub Releases: it finds the
// latest release, downloads the archive built for this OS and CPU, verifies
// it against the release's checksums, unpacks it beside the running program
// and restarts into it.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// Repository is the GitHub repository the releases are published in.
const Repository = "ichi0g0y/mayak"

// ChecksumsAsset is the release asset that lists the SHA-256 of every
// archive ("<hex>  <file name>" per line, as sha256sum writes it).
const ChecksumsAsset = "SHA256SUMS.txt"

// Release is a published GitHub release.
type Release struct {
	Tag         string    `json:"tag_name"`
	Name        string    `json:"name"`
	Notes       string    `json:"body"`
	URL         string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	Assets      []Asset   `json:"assets"`
}

// Asset is one downloadable file of a release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// Version is the release's version without the tag's "v".
func (r Release) Version() string { return strings.TrimPrefix(r.Tag, "v") }

// Asset returns the release's asset of that name.
func (r Release) Asset(name string) (Asset, bool) {
	for _, asset := range r.Assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return Asset{}, false
}

// ArchiveName is the release asset built for an OS and CPU:
// Mayak-windows-amd64.zip, Mayak-darwin-arm64.tar.gz, Mayak-linux-amd64.tar.gz.
func ArchiveName(goos, goarch string) string {
	if goos == "windows" {
		return "Mayak-" + goos + "-" + goarch + ".zip"
	}
	return "Mayak-" + goos + "-" + goarch + ".tar.gz"
}

// Archive returns the release's archive for this OS and CPU.
func (r Release) Archive() (Asset, bool) { return r.Asset(ArchiveName(runtime.GOOS, runtime.GOARCH)) }

// Client reads releases from the GitHub API.
type Client struct {
	HTTP       *http.Client
	UserAgent  string
	Repository string
	// APIBase is the GitHub API root; tests point it at a local server.
	APIBase string
}

// NewClient returns a client for Repository with a 20 second timeout.
func NewClient(userAgent string) *Client {
	return &Client{HTTP: &http.Client{Timeout: 20 * time.Second}, UserAgent: userAgent, Repository: Repository, APIBase: "https://api.github.com"}
}

// Latest returns the newest published release that is not a draft or
// pre-release, as GitHub's "latest" release.
func (c *Client) Latest(ctx context.Context) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.APIBase+"/repos/"+c.Repository+"/releases/latest", nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", c.UserAgent)
	response, err := c.HTTP.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return Release{}, fmt.Errorf("no release has been published")
	}
	if response.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("GitHub: %s", response.Status)
	}
	const maxResponseBytes = 1 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return Release{}, err
	}
	if len(body) > maxResponseBytes {
		return Release{}, fmt.Errorf("GitHub: response too large")
	}
	var release Release
	if err := json.Unmarshal(body, &release); err != nil {
		return Release{}, fmt.Errorf("GitHub: %w", err)
	}
	if release.Tag == "" {
		return Release{}, fmt.Errorf("GitHub: release without a tag")
	}
	return release, nil
}

// Checksums downloads and parses the release's ChecksumsAsset: file name to
// lower-case hex SHA-256.
func (c *Client) Checksums(ctx context.Context, release Release) (map[string]string, error) {
	asset, ok := release.Asset(ChecksumsAsset)
	if !ok {
		return nil, fmt.Errorf("release %s has no %s", release.Tag, ChecksumsAsset)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	response, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", ChecksumsAsset, response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return nil, err
	}
	return ParseChecksums(string(body)), nil
}

// ParseChecksums reads sha256sum output: "<hex>  <name>" or "<hex> *<name>".
func ParseChecksums(text string) map[string]string {
	sums := make(map[string]string)
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || len(fields[0]) != 64 {
			continue
		}
		name := strings.TrimPrefix(strings.Join(fields[1:], " "), "*")
		sums[name] = strings.ToLower(fields[0])
	}
	return sums
}
