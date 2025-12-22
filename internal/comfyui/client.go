package comfyui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"comfy-tg-bot/internal/config"
)

// Client handles communication with the ComfyUI API
type Client struct {
	baseURL    string
	wsURL      string
	httpClient *http.Client
	workflow   *WorkflowManager
	logger     *slog.Logger
}

// NewClient creates a new ComfyUI client
func NewClient(cfg config.ComfyUIConfig, logger *slog.Logger) (*Client, error) {
	workflow, err := NewWorkflowManager(cfg.WorkflowPath)
	if err != nil {
		return nil, fmt.Errorf("load workflow: %w", err)
	}

	return &Client{
		baseURL: cfg.BaseURL,
		wsURL:   cfg.WebSocketURL,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		workflow: workflow,
		logger:   logger,
	}, nil
}

// GenerateImage is the main entry point for image generation
func (c *Client) GenerateImage(ctx context.Context, prompt string) ([]byte, error) {
	// Create execution monitor with unique client ID
	monitor := NewExecutionMonitor(c.wsURL, c.logger)

	// Prepare workflow
	workflow, err := c.workflow.PrepareWorkflow(prompt)
	if err != nil {
		return nil, fmt.Errorf("prepare workflow: %w", err)
	}

	// Queue the prompt
	promptID, err := c.QueuePrompt(ctx, workflow, monitor.GetClientID())
	if err != nil {
		return nil, fmt.Errorf("queue prompt: %w", err)
	}

	c.logger.Debug("prompt queued", "prompt_id", promptID)

	// Wait for completion
	if err := monitor.WaitForCompletion(ctx, promptID, nil); err != nil {
		return nil, fmt.Errorf("wait for completion: %w", err)
	}

	// Get history to find output
	history, err := c.GetHistory(ctx, promptID)
	if err != nil {
		return nil, fmt.Errorf("get history: %w", err)
	}

	// Find output image
	entry, ok := history[promptID]
	if !ok {
		return nil, fmt.Errorf("prompt not found in history")
	}

	// Find first image in outputs
	for _, output := range entry.Outputs {
		if len(output.Images) > 0 {
			img := output.Images[0]
			return c.GetImage(ctx, img.Filename, img.Subfolder, img.Type)
		}
	}

	return nil, fmt.Errorf("no output image found")
}

// QueuePrompt sends a prompt to ComfyUI
func (c *Client) QueuePrompt(ctx context.Context, workflow map[string]any, clientID string) (string, error) {
	req := PromptRequest{
		Prompt:   workflow,
		ClientID: clientID,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/prompt", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBody))
	}

	var promptResp PromptResponse
	if err := json.Unmarshal(respBody, &promptResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if promptResp.Error != "" {
		return "", fmt.Errorf("comfyui error: %s", promptResp.Error)
	}

	return promptResp.PromptID, nil
}

// GetHistory retrieves the execution history for a prompt
func (c *Client) GetHistory(ctx context.Context, promptID string) (HistoryResponse, error) {
	reqURL := fmt.Sprintf("%s/history/%s", c.baseURL, promptID)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	var history HistoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&history); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return history, nil
}

// GetImage downloads an image from ComfyUI
func (c *Client) GetImage(ctx context.Context, filename, subfolder, imgType string) ([]byte, error) {
	params := url.Values{}
	params.Set("filename", filename)
	if subfolder != "" {
		params.Set("subfolder", subfolder)
	}
	if imgType != "" {
		params.Set("type", imgType)
	}

	reqURL := fmt.Sprintf("%s/view?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// CheckHealth verifies ComfyUI is accessible
func (c *Client) CheckHealth(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/system_stats", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: status %d", resp.StatusCode)
	}

	return nil
}
