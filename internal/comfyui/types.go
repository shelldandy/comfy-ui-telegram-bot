package comfyui

import "encoding/json"

// PromptRequest is sent to POST /prompt
type PromptRequest struct {
	Prompt   map[string]any `json:"prompt"`
	ClientID string         `json:"client_id"`
}

// PromptResponse is returned from POST /prompt
type PromptResponse struct {
	PromptID   string                 `json:"prompt_id"`
	Number     int                    `json:"number"`
	NodeErrors map[string][]NodeError `json:"node_errors,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

// NodeError contains error information for a specific node
type NodeError struct {
	Type      string `json:"type"`
	Message   string `json:"message"`
	Details   string `json:"details"`
	ExtraInfo any    `json:"extra_info"`
}

// HistoryResponse is returned from GET /history/{prompt_id}
type HistoryResponse map[string]HistoryEntry

// HistoryEntry contains execution history for a single prompt
type HistoryEntry struct {
	Prompt  []any                 `json:"prompt"`
	Outputs map[string]NodeOutput `json:"outputs"`
	Status  ExecutionStatus       `json:"status"`
}

// NodeOutput contains output data from a node
type NodeOutput struct {
	Images []ImageOutput `json:"images,omitempty"`
}

// ImageOutput describes an output image
type ImageOutput struct {
	Filename  string `json:"filename"`
	Subfolder string `json:"subfolder"`
	Type      string `json:"type"`
}

// ExecutionStatus indicates the status of an execution
type ExecutionStatus struct {
	StatusStr string `json:"status_str"`
	Completed bool   `json:"completed"`
}

// WSMessage represents a WebSocket message from ComfyUI
type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// ExecutingData is the data payload for "executing" messages
type ExecutingData struct {
	Node     *string `json:"node"`
	PromptID string  `json:"prompt_id"`
}

// ProgressData is the data payload for "progress" messages
type ProgressData struct {
	Value    int    `json:"value"`
	Max      int    `json:"max"`
	PromptID string `json:"prompt_id"`
}

// SystemStats is returned from GET /system_stats
type SystemStats struct {
	System struct {
		OS             string `json:"os"`
		PythonVersion  string `json:"python_version"`
		EmbeddedPython bool   `json:"embedded_python"`
	} `json:"system"`
	Devices []DeviceInfo `json:"devices"`
}

// DeviceInfo contains information about a compute device
type DeviceInfo struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	Index          int    `json:"index"`
	VRAMTotal      int64  `json:"vram_total"`
	VRAMFree       int64  `json:"vram_free"`
	TorchVRAMTotal int64  `json:"torch_vram_total"`
	TorchVRAMFree  int64  `json:"torch_vram_free"`
}
