# Vibe Desk Display for Claude Code

Turn your VibeDesk into a live Claude Code usage monitor. Usage auto-updates every time Claude responds — no dashboard, no browser, just a glance at your desk.

## Quick Start

```bash
claude plugins marketplace add https://github.com/autonomous-ai/vibe-desk-display
claude plugins install vibe-desk-display
```

Restart Claude Code, then type `pair my display` and follow the on-screen instructions.

See the full [Setup Guide](GUIDE.md) for details.

## Features

- **Live usage display** — 5-hour and 7-day usage auto-updates on display after every Claude response
- **Notifications** — send custom messages to the screen ("notify my display when done")
- **OTP pairing** — no sticker reading, just enter the code shown on screen
- **Zero dependencies** — Python 3 stdlib only, no pip install needed

## Commands

| Command | Description |
|---------|-------------|
| `/vibe-desk-display:usage` | Refresh usage display now |
| `/vibe-desk-display:notify` | Send a notification |

Or use natural language: "show my usage on display", "notify my display", "unpair my display"

## Requirements

- macOS
- Python 3
- A VibeDesk device on the same WiFi
- Claude Code with OAuth login

## Update / Uninstall

```bash
claude plugins update vibe-desk-display      # pull latest
claude plugins uninstall vibe-desk-display   # remove plugin
```
