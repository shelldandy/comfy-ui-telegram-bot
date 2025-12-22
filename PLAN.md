# ComfyUI Telegram Bot Implementation Plan

## Overview
A Go-based Telegram bot that receives text prompts, generates images via ComfyUI, and returns both original and JPEG-compressed versions.

## Configuration
- ComfyUI: `localhost:8188`
- Workflow: User-provided JSON file
- Access: Whitelist by Telegram user ID
- Compression: JPEG at 80% quality

---

## Project Structure

```
comfy-tg-bot/
├── cmd/bot/main.go                 # Entry point, initialization, graceful shutdown
├── internal/
│   ├── config/config.go            # Viper-based config loading (YAML + env vars)
│   ├── telegram/
│   │   ├── bot.go                  # Bot setup, polling loop
│   │   ├── handlers.go             # /start, /help, /status, text message handler
│   │   └── middleware.go           # Whitelist authentication
│   ├── comfyui/
│   │   ├── client.go               # HTTP client (POST /prompt, GET /history, GET /view)
│   │   ├── websocket.go            # WebSocket for execution monitoring
│   │   ├── workflow.go             # Load workflow, replace {{PROMPT}} placeholder
│   │   └── types.go                # API request/response structs
│   └── image/processor.go          # PNG decode, JPEG encode at quality 80
├── configs/config.example.yaml     # Example config with documentation
├── go.mod
├── Makefile
├── Dockerfile
└── README.md
```

---

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/go-telegram-bot-api/telegram-bot-api/v5` | Telegram Bot API |
| `github.com/gorilla/websocket` | ComfyUI WebSocket client |
| `github.com/spf13/viper` | Configuration management |
| `log/slog` (stdlib) | Structured logging |
| `image/jpeg`, `image/png` (stdlib) | Image processing |

---

## Configuration File Format

```yaml
telegram:
  bot_token: ""  # Or set COMFY_BOT_TELEGRAM_BOT_TOKEN env var
  allowed_users:
    - 123456789
  polling_timeout: 60

comfyui:
  base_url: "http://localhost:8188"
  websocket_url: "ws://localhost:8188/ws"
  workflow_path: "./workflows/workflow.json"
  # Workflow should contain {{PROMPT}} placeholder where user text goes

image:
  jpeg_quality: 80
```

---

## Image Generation Flow

1. User sends text message to bot
2. Middleware checks if user ID is in whitelist
3. Bot loads workflow JSON, replaces `{{PROMPT}}` with user's text
4. Bot connects WebSocket to ComfyUI with unique client ID
5. Bot POSTs workflow to `/prompt`, receives `prompt_id`
6. Bot sends "Generating image..." message to user
7. Bot listens for WebSocket messages until execution completes
8. Bot GETs `/history/{prompt_id}` to find output image filename
9. Bot GETs `/view?filename=...` to download image bytes
10. Bot compresses image to JPEG (80% quality)
11. Bot sends original file as document + compressed as photo to user
12. Bot closes WebSocket

---

## Implementation Phases

### Phase 1: Foundation
1. `go mod init` with dependencies
2. `internal/config/config.go` - Viper config loading
3. `cmd/bot/main.go` - Basic structure with logging

### Phase 2: Telegram Bot
4. `internal/telegram/bot.go` - Initialize bot, start polling
5. `internal/telegram/middleware.go` - Whitelist check
6. `internal/telegram/handlers.go` - /start, /help commands

### Phase 3: ComfyUI Integration
7. `internal/comfyui/types.go` - API type definitions
8. `internal/comfyui/workflow.go` - Load and inject prompt
9. `internal/comfyui/client.go` - REST API client
10. `internal/comfyui/websocket.go` - Execution monitoring

### Phase 4: Image Generation Handler
11. Connect text message handler to ComfyUI client
12. `internal/image/processor.go` - JPEG compression
13. Send both image versions to user

### Phase 5: Polish
14. `/status` command - Check ComfyUI connectivity
15. Error handling and user-friendly messages
16. `configs/config.example.yaml`, `Makefile`, `README.md`

---

## Files to Create (in order)

1. `go.mod` - Module definition
2. `internal/config/config.go` - Configuration
3. `internal/comfyui/types.go` - Type definitions
4. `internal/comfyui/workflow.go` - Workflow handling
5. `internal/comfyui/client.go` - REST client
6. `internal/comfyui/websocket.go` - WebSocket client
7. `internal/image/processor.go` - Image compression
8. `internal/telegram/middleware.go` - Auth middleware
9. `internal/telegram/handlers.go` - Message handlers
10. `internal/telegram/bot.go` - Bot setup
11. `cmd/bot/main.go` - Entry point
12. `configs/config.example.yaml` - Example config
13. `Makefile` - Build commands
14. `README.md` - Documentation
