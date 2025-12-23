# ComfyUI Telegram Bot

A Go-based Telegram bot that generates images using ComfyUI. Send a text prompt and receive both the original PNG and a compressed JPEG preview.

## Features

- Text-to-image generation via ComfyUI
- Whitelist-based access control
- Admin user with dynamic user/group approval/rejection
- Group chat support via @mention
- Returns both PNG (original) and JPEG (compressed preview)
- Per-user settings for image delivery preferences
- Per-user request limiting (one generation at a time per user)
- Graceful shutdown handling

## Requirements

- Go 1.24+
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
| `COMFY_BOT_TELEGRAM_ALLOWED_USERS` | Comma-separated user IDs (optional if `ADMIN_USER` is set) |
| `COMFY_BOT_TELEGRAM_ADMIN_USER` | Admin user ID for approving new users (optional if `ALLOWED_USERS` is set) |
| `COMFY_BOT_COMFYUI_BASE_URL` | ComfyUI HTTP URL |
| `COMFY_BOT_COMFYUI_WORKFLOW_PATH` | Path to workflow JSON |
| `COMFY_BOT_SETTINGS_DATABASE_PATH` | Path to SQLite database for user settings (default: `data/settings.db`) |
| `COMFY_BOT_SETTINGS_SEND_ORIGINAL` | Default setting for sending original PNG (default: `true`) |
| `COMFY_BOT_SETTINGS_SEND_COMPRESSED` | Default setting for sending compressed JPEG (default: `true`) |

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
- `/settings` - Configure image delivery preferences (toggle original PNG / compressed JPEG)
- `/status` - Check ComfyUI server status
- `/revoke <user_id>` - (Admin only) Revoke a user's access
- `/revokegroup <group_id>` - (Admin only) Revoke a group's access

## Admin User Approval

When `ADMIN_USER` is configured, the bot supports dynamic user approval:

1. When an unauthorized user messages the bot, the admin receives a notification with **Approve** / **Reject** buttons
2. If approved, the user is added to the database and can use the bot immediately
3. If rejected, the user is notified and their request is removed
4. The admin can later revoke access using `/revoke <user_id>`

Approved users are stored in the SQLite database and have the same permissions as users in `ALLOWED_USERS`. Users in `ALLOWED_USERS` (from config) cannot be revoked - only dynamically approved users can be revoked.

## Group Chat Support

The bot can be added to Telegram groups with the following behavior:

### Setup for Groups

Before the bot can receive mentions in groups, you must disable Privacy Mode:

1. Open @BotFather on Telegram
2. Send `/mybots` and select your bot
3. Go to **Bot Settings** → **Group Privacy**
4. Set to **Disable** (should show "Privacy mode is disabled")

> **Note:** If you change this setting after the bot is already in a group, you must remove and re-add the bot to that group.

### How to Use in Groups

1. Add the bot to a group
2. Mention the bot with a prompt: `@botusername a beautiful sunset over mountains`
3. The bot will generate and reply with a compressed JPEG image

### Group Authorization

Groups require admin approval before the bot will respond:

1. When someone mentions the bot in an unapproved group, the admin receives a notification with **Approve** / **Reject** buttons
2. If approved, the group receives a confirmation message and can start using the bot
3. If rejected, the request is simply removed
4. The admin can later revoke access using `/revokegroup <group_id>`

### Group vs Private Chat Differences

| Feature | Private Chat | Group Chat |
|---------|--------------|------------|
| Trigger | Any text message | `@botusername` mention only |
| Commands | Supported | Ignored |
| Image output | Per-user settings (PNG/JPEG) | Compressed JPEG only |
| Response style | Direct message | Reply to original message |

## License

MIT
