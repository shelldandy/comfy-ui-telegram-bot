package comfyui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

const PromptPlaceholder = "{{PROMPT}}"

// WorkflowManager handles loading and modifying workflow templates
type WorkflowManager struct {
	templatePath string
	template     []byte
	mu           sync.RWMutex
}

// NewWorkflowManager creates a new workflow manager and loads the template
func NewWorkflowManager(templatePath string) (*WorkflowManager, error) {
	wm := &WorkflowManager{
		templatePath: templatePath,
	}

	if err := wm.Load(); err != nil {
		return nil, err
	}

	return wm, nil
}

// Load reads and validates the workflow template
func (wm *WorkflowManager) Load() error {
	data, err := os.ReadFile(wm.templatePath)
	if err != nil {
		return fmt.Errorf("read workflow file: %w", err)
	}

	// Validate it's valid JSON
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("invalid workflow JSON: %w", err)
	}

	// Check for placeholder
	if !strings.Contains(string(data), PromptPlaceholder) {
		return fmt.Errorf("workflow must contain %s placeholder", PromptPlaceholder)
	}

	wm.mu.Lock()
	wm.template = data
	wm.mu.Unlock()

	return nil
}

// PrepareWorkflow creates a workflow with the user's prompt
func (wm *WorkflowManager) PrepareWorkflow(userPrompt string) (map[string]any, error) {
	wm.mu.RLock()
	templateCopy := make([]byte, len(wm.template))
	copy(templateCopy, wm.template)
	wm.mu.RUnlock()

	// Sanitize the prompt for JSON embedding
	sanitized := sanitizeForJSON(userPrompt)

	// Replace placeholder
	modified := strings.ReplaceAll(string(templateCopy), PromptPlaceholder, sanitized)

	// Parse and validate result
	var workflow map[string]any
	if err := json.Unmarshal([]byte(modified), &workflow); err != nil {
		return nil, fmt.Errorf("prompt created invalid JSON: %w", err)
	}

	return workflow, nil
}

// sanitizeForJSON escapes special characters for safe JSON string embedding
func sanitizeForJSON(s string) string {
	// Use json.Marshal to properly escape the string
	escaped, err := json.Marshal(s)
	if err != nil {
		// Fallback to basic escaping
		s = strings.ReplaceAll(s, "\\", "\\\\")
		s = strings.ReplaceAll(s, "\"", "\\\"")
		s = strings.ReplaceAll(s, "\n", "\\n")
		s = strings.ReplaceAll(s, "\r", "\\r")
		s = strings.ReplaceAll(s, "\t", "\\t")
		return s
	}

	// Remove surrounding quotes from json.Marshal output
	return string(escaped[1 : len(escaped)-1])
}

// Reload reloads the workflow template from disk
func (wm *WorkflowManager) Reload() error {
	return wm.Load()
}
