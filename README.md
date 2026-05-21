# LCD Plugin for Claude Code

Display Claude Code usage and notifications on your ESP32 ST7789 LCD device.

## Install

```bash
claude --plugin-dir /path/to/lcd-skills
```

Or add to `~/.claude/settings.json`:

```json
{
  "plugins": ["/path/to/lcd-skills"]
}
```

## Setup

Tell Claude Code:

> "pair my LCD"

It will ask for your device ID (printed on the sticker, format: `lcd-XXXXXX`) and discover the device on your network.

## What you get

- **Auto-updating LCD** — Claude Code usage auto-updates on LCD every time Claude responds (via Stop hook, rate-limited to once per 60s).
- **On-demand usage** — `/lcd:usage` fetches and displays live usage immediately.
- **Notifications** — `/lcd:notify` or just say "notify my LCD when done" during any task.

## Slash commands

| Command | Description |
|---------|-------------|
| `/lcd:usage` | Fetch and display usage now |
| `/lcd:notify` | Send a notification to the LCD |

## Natural language triggers

- "pair my LCD" / "unpair my LCD"
- "show my usage on LCD"
- "notify my LCD" / "ping my display when done"
- "find my LCD" / "rescan for device"
- "LCD status"

## Requirements

- macOS (Keychain for OAuth token)
- Python 3 (stdlib only, no pip packages)
- ESP32 ST7789 LCD device on the same LAN
- Claude Code subscription (OAuth login, not API key)

## Files

```
lcd-skills/
├── plugin.json              Plugin manifest
├── SKILL.md                 Main skill definition
├── usage.md                 /lcd:usage slash command
├── notify.md                /lcd:notify slash command
├── README.md                This file
├── hooks/
│   └── hooks.json           Stop hook config (auto-update usage)
└── scripts/
    ├── lcd-usage-daemon.py  Fetch usage and send to LCD
    └── on-stop-usage.py     Hook script (called on Stop event)
```

## Uninstall

Tell Claude Code:

> "unpair my LCD"

To fully remove, also uninstall the plugin: `claude plugins uninstall lcd`
