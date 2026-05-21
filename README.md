# LCD Plugin for Claude Code

Turn your ESP32 LCD into a live Claude Code usage monitor. Usage auto-updates every time Claude responds — no dashboard, no browser, just a glance at your desk.

## Quick Start

```bash
claude plugins marketplace add https://github.com/secret3579/lcd-skills
claude plugins install lcd
```

Restart Claude Code, then type `pair my LCD` and follow the on-screen instructions.

See the full [Setup Guide](GUIDE.md) for details.

## Features

- **Live usage display** — 5-hour and 7-day usage auto-updates on LCD after every Claude response
- **Notifications** — send custom messages to the screen ("notify my LCD when done")
- **OTP pairing** — no sticker reading, just enter the code shown on screen
- **Zero dependencies** — Python 3 stdlib only, no pip install needed

## Commands

| Command | Description |
|---------|-------------|
| `/lcd:usage` | Refresh usage display now |
| `/lcd:notify` | Send a notification |

Or use natural language: "show my usage on LCD", "notify my LCD", "unpair my LCD"

## Requirements

- macOS
- Python 3
- ESP32 ST7789 LCD device on the same WiFi
- Claude Code with OAuth login

## Update / Uninstall

```bash
claude plugins update lcd      # pull latest
claude plugins uninstall lcd   # remove plugin
```
