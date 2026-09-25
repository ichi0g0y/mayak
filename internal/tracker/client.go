package tracker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.tarkovtracker.org"

type Mode string

const (
	ModePVP      Mode = "pvp"
	ModePVE      Mode = "pve"
	ModeSeasonal Mode = "seasonal"
)

type TokenInfo struct {
	Success     bool     `json:"success"`
	Permissions []string `json:"permissions"`
	Token       string   `json:"token"`
	Owner       string   `json:"owner"`
	Note        string   `json:"note"`
	GameMode    string   `json:"gameMode"`
}

type Progress struct {
	Success bool `json:"success"`
	Data    struct {
		HideoutModules []struct {
			ID       string `json:"id"`
			Complete bool   `json:"complete"`
		} `json:"hideoutModulesProgress"`
		Tasks []struct {
			ID       string `json:"id"`
			Complete bool   `json:"complete"`
			Failed   bool   `json:"failed"`
		} `json:"tasksProgress"`
		DisplayName string `json:"displayName"`
		PlayerLevel int    `json:"playerLevel"`
	} `json:"data"`
	Meta struct {
		GameMode string `json:"gameMode"`
	} `json:"meta"`
}

type Client struct {
	http    *http.Client
	baseURL string
}

func New() *Client {
	return &Client{http: &http.Client{Timeout: 15 * time.Second}, baseURL: defaultBaseURL}
}

func (c *Client) TokenInfo(ctx context.Context, token string) (TokenInfo, error) {
	var result TokenInfo
	if err := c.do(ctx, http.MethodGet, "/token", token, nil, &result); err != nil {
		return result, err
	}
	if !result.Success {
		return result, errors.New("TarkovTracker rejected the token")
	}
	return result, nil
}

func (c *Client) Progress(ctx context.Context, token string) (Progress, error) {
	var result Progress
	if err := c.do(ctx, http.MethodGet, "/progress", token, nil, &result); err != nil {
		return result, err
	}
	if !result.Success {
		return result, errors.New("TarkovTracker returned unsuccessful progress")
	}
	return result, nil
}

func (c *Client) SetTask(ctx context.Context, token, taskID, state string) error {
	if !validTaskID(taskID) {
		return errors.New("invalid EFT task ID")
	}
	switch state {
	case "completed", "failed", "uncompleted":
	default:
		return errors.New("invalid TarkovTracker task state")
	}
	return c.do(ctx, http.MethodPost, "/progress/task/"+url.PathEscape(taskID), token, map[string]string{"state": state}, nil)
}

type TaskUpdate struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

func (c *Client) SetTasks(ctx context.Context, token string, updates []TaskUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	for _, update := range updates {
		if !validTaskID(update.ID) {
			return errors.New("invalid EFT task ID")
		}
		switch update.State {
		case "completed", "failed", "uncompleted":
		default:
			return errors.New("invalid TarkovTracker task state")
		}
	}
	return c.do(ctx, http.MethodPost, "/progress/tasks", token, updates, nil)
}

func (c *Client) do(ctx context.Context, method, path, token string, body any, target any) error {
	if strings.TrimSpace(token) == "" {
		return errors.New("TarkovTracker token is required")
	}
	var encoded []byte
	var err error
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	for attempt := 0; attempt < 2; attempt++ {
		request, requestErr := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(encoded))
		if requestErr != nil {
			return requestErr
		}
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Accept", "application/json")
		request.Header.Set("User-Agent", "MAYAK/0.1.0")
		if body != nil {
			request.Header.Set("Content-Type", "application/json")
		}
		response, requestErr := c.http.Do(request)
		if requestErr != nil {
			if attempt == 0 && ctx.Err() == nil {
				continue
			}
			return requestErr
		}
		responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		_ = response.Body.Close()
		if readErr != nil {
			return readErr
		}
		if response.StatusCode == http.StatusTooManyRequests && attempt == 0 {
			delay := retryDelay(response.Header.Get("Retry-After"))
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			var apiError struct {
				Error string `json:"error"`
			}
			_ = json.Unmarshal(responseBody, &apiError)
			if apiError.Error == "" {
				apiError.Error = strings.TrimSpace(string(responseBody))
			}
			if apiError.Error == "" {
				apiError.Error = response.Status
			}
			return fmt.Errorf("TarkovTracker API: %s", apiError.Error)
		}
		if target != nil && len(responseBody) > 0 {
			if err := json.Unmarshal(responseBody, target); err != nil {
				return fmt.Errorf("decode TarkovTracker response: %w", err)
			}
		}
		return nil
	}
	return errors.New("TarkovTracker request failed")
}

func retryDelay(value string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || seconds < 1 {
		return time.Second
	}
	if seconds > 30 {
		seconds = 30
	}
	return time.Duration(seconds) * time.Second
}

func validTaskID(value string) bool {
	if len(value) != 24 {
		return false
	}
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

func ModeForToken(token string) (Mode, bool) {
	switch {
	case strings.HasPrefix(strings.ToUpper(token), "PVP_"):
		return ModePVP, true
	case strings.HasPrefix(strings.ToUpper(token), "PVE_"):
		return ModePVE, true
	case strings.HasPrefix(strings.ToUpper(token), "SZN_"):
		return ModeSeasonal, true
	default:
		return "", false
	}
}

func HasPermissions(info TokenInfo, required ...string) bool {
	available := make(map[string]bool, len(info.Permissions))
	for _, permission := range info.Permissions {
		available[strings.ToUpper(permission)] = true
	}
	for _, permission := range required {
		if !available[strings.ToUpper(permission)] {
			return false
		}
	}
	return true
}
