package comfyui

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// ProgressCallback is called when progress updates are received
type ProgressCallback func(current, total int)

// ExecutionMonitor monitors a single prompt execution via WebSocket
type ExecutionMonitor struct {
	wsURL    string
	logger   *slog.Logger
	clientID string
}

// NewExecutionMonitor creates a new execution monitor with a unique client ID
func NewExecutionMonitor(wsURL string, logger *slog.Logger) *ExecutionMonitor {
	return &ExecutionMonitor{
		wsURL:    wsURL,
		logger:   logger,
		clientID: uuid.New().String(),
	}
}

// GetClientID returns the client ID for use in prompt submission
func (m *ExecutionMonitor) GetClientID() string {
	return m.clientID
}

// WaitForCompletion waits for a specific prompt to complete
// Returns nil on success, error on failure or context cancellation
func (m *ExecutionMonitor) WaitForCompletion(ctx context.Context, promptID string, progressCb ProgressCallback) error {
	url := fmt.Sprintf("%s?clientId=%s", m.wsURL, m.clientID)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, url, nil)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}
	defer conn.Close()

	// Set up read deadline management
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		return nil
	})

	// Start ping ticker
	pingTicker := time.NewTicker(10 * time.Second)
	defer pingTicker.Stop()

	// Channel for read results
	msgCh := make(chan WSMessage)
	errCh := make(chan error)

	// Read goroutine
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				errCh <- err
				return
			}

			var msg WSMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				m.logger.Debug("failed to unmarshal ws message", "error", err)
				continue
			}
			msgCh <- msg
		}
	}()

	for {
		select {
		case <-ctx.Done():
			// Send close frame before returning
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			return ctx.Err()

		case <-pingTicker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return fmt.Errorf("ping failed: %w", err)
			}

		case err := <-errCh:
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				return fmt.Errorf("websocket closed unexpectedly")
			}
			return fmt.Errorf("websocket read: %w", err)

		case msg := <-msgCh:
			// Reset read deadline on any message
			conn.SetReadDeadline(time.Now().Add(30 * time.Second))

			switch msg.Type {
			case "executing":
				var data ExecutingData
				if err := json.Unmarshal(msg.Data, &data); err != nil {
					continue
				}

				if data.PromptID == promptID && data.Node == nil {
					// Execution complete
					m.logger.Debug("execution complete", "prompt_id", promptID)
					return nil
				}

			case "progress":
				var data ProgressData
				if err := json.Unmarshal(msg.Data, &data); err != nil {
					continue
				}

				if data.PromptID == promptID && progressCb != nil {
					progressCb(data.Value, data.Max)
				}

			case "execution_error":
				return fmt.Errorf("comfyui execution error: %s", string(msg.Data))
			}
		}
	}
}
