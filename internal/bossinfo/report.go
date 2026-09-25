package bossinfo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// reportURL is tarkov.dev's public intake for Goons sightings. The service
// stores the reporter's IP address and account ID with each report.
const reportURL = "https://manager.tarkov.dev/api/goons"

// Report is one Goons sighting: the map by the game's name (e.g. "bigmap"),
// the data set ("regular" or "pve"), when (the raid's start) and the EFT
// account that saw them.
type Report struct {
	MapNameID string
	Mode      string
	Time      time.Time
	AccountID string
}

// SendReport submits a Goons sighting. It shows in the maps data's
// goonReports once the data is next built (every ten minutes).
func (c *Client) SendReport(ctx context.Context, r Report) error {
	account, err := strconv.ParseInt(r.AccountID, 10, 32)
	if err != nil || account <= 0 {
		return errors.New("invalid account ID")
	}
	if r.MapNameID == "" || (r.Mode != "regular" && r.Mode != "pve") || r.Time.IsZero() {
		return errors.New("invalid Goons report")
	}
	body, _ := json.Marshal(map[string]any{"map": r.MapNameID, "gameMode": r.Mode, "timestamp": r.Time.UnixMilli(), "accountId": account})
	url := c.reportURL
	if url == "" {
		url = reportURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "MAYAK/0.1.0")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	answer, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Goons report: HTTP %d %s", resp.StatusCode, bytes.TrimSpace(answer))
	}
	var result struct {
		Status string `json:"status"`
	}
	if json.Unmarshal(answer, &result) != nil || result.Status != "success" {
		return fmt.Errorf("Goons report was not accepted: %s", bytes.TrimSpace(answer))
	}
	return nil
}
