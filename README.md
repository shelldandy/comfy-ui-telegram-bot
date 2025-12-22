# ComfyUI Telegram Bot

A Go-based Telegram bot that generates images using ComfyUI. Send a text prompt and receive both the original PNG and a compressed JPEG preview.

## Features

- Text-to-image generation via ComfyUI
- Whitelist-based access control
- Returns both PNG (original) and JPEG (compressed preview)
- Per-user request limiting (one generation at a time per user)
- Graceful shutdown handling

## Requirements

- Go 1.21+
- ComfyUI running and accessible
- Telegram Bot token (from @BotFather)

## Setup

1. Copy the example config:
   ```bash
   cp configs/config.example.yaml config.yaml
   ```

2. Edit `config.yaml`:
   - Set your Telegram bot token
   - Add allowed user IDs
   - Set the path to your workflow JSON file

3. Prepare your workflow:
   - Export your ComfyUI workflow as API format JSON
   - Add `{{PROMPT}}` placeholder where the user's prompt should be inserted

4. Build and run:
   ```bash
   make build
   ./bin/comfy-tg-bot
   ```

## Docker Deployment

1. Copy environment template:
   ```bash
   cp .env.example .env
   ```

2. Edit `.env` with your bot token and allowed user IDs

3. Place your `workflow.json` in the project root

4. Build and run:
   ```bash
   make docker-build
   make docker-run
   ```

5. View logs:
   ```bash
   make docker-logs
   ```

The container uses `host.docker.internal` to connect to ComfyUI running on your host machine.

## Configuration

Configuration can be set via:
- YAML file (`config.yaml` in current directory or `configs/`)
- Environment variables (prefix: `COMFY_BOT_`)

### Environment Variables

| Variable | Description |
|----------|-------------|
| `COMFY_BOT_TELEGRAM_BOT_TOKEN` | Telegram bot API token |
| `COMFY_BOT_TELEGRAM_ALLOWED_USERS` | Comma-separated user IDs |
| `COMFY_BOT_COMFYUI_BASE_URL` | ComfyUI HTTP URL |
| `COMFY_BOT_COMFYUI_WORKFLOW_PATH` | Path to workflow JSON |

## Workflow Setup

Your workflow JSON must contain the `{{PROMPT}}` placeholder. Example structure:

```json
{
  "3": {
    "class_type": "KSampler",
    "inputs": { ... }
  },
  "6": {
    "class_type": "CLIPTextEncode",
    "inputs": {
      "text": "{{PROMPT}}",
      "clip": ["4", 0]
    }
  }
}
```

## Commands

- `/start` - Welcome message
- `/help` - Usage instructions
- `/status` - Check ComfyUI server status

## License

MIT
