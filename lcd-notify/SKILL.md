---
name: lcd-notify
description: >
  Send a text notification to the user's LCD notification device. Use whenever the user
  asks to display, show, push, or send text to their LCD, screen, or notification device.
  Also use proactively at the end of long tasks (build, test, deploy) to ping the LCD so
  the user knows without checking the terminal. Triggers: "show this on my screen",
  "notify my LCD", "ping my display when done", "let me know on the device".
---

# LCD Notification

Send a text notification to a paired LCD device over the LAN.

## Config file

Path: `~/.config/autonomous-lcd.json` — must exist with at least one paired device.
If no config or no devices, tell the user to run **lcd-pair** first.

## Utilities

Use `python3` for JSON config reads and writes (`jq` is not available by default on macOS).

## Procedure

### 1. Load config and pick device

Read `~/.config/autonomous-lcd.json`. Determine target device:
- Use the device ID the user specified, OR
- Use `default_device_id` from config, OR
- Error if neither is available.

Get the device's `token` and `last_known_ip` from the config.

### 2. Prepare the message

Build the JSON payload:

```json
{
  "text": "<message>",
  "size": 2,
  "color": "blue",
  "title": "Claude"
}
```

Parameters:
- **text** (required) — the notification message. Max 500 characters. Strip any markdown formatting (`**bold**` → `bold`, etc.) to plain text. Warn if over 165 characters (may not fit on screen at size 2).
- **size** (optional) — 1 to 4, default 2.
- **color** (optional) — header bar color, default "blue". Options: red, green, blue, yellow, white.
- **title** (optional) — shown in header bar, default "Claude".

### 3. Send notification

```bash
RESPONSE=$(curl -s -w "\n%{http_code}" --max-time 3 \
  -X POST http://<last_known_ip>:3000/lcd \
  -H "Content-Type: application/json" \
  -H "X-Token: <token>" \
  -H "X-Device-ID: <device_id>" \
  -d '<json_payload>')
HTTP_CODE=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | sed '$d')
```

### 4. Handle response

- **200** — Success. Update `last_seen_at` in config. Report to user that the message was delivered.
- **400** — Malformed payload. Check the JSON.
- **401** — Token wrong. Tell user to re-pair the device (run **lcd-pair**).
- **403** — Device ID mismatch. The cached IP belongs to a different device. Run rediscovery (see step 5).
- **413** — Text too long (>500 chars). Truncate to 497 characters + `"..."` and retry once.
- **Connection error / timeout** — Cached IP is stale. Run rediscovery (see step 5).

### 5. Rediscovery and retry (on failure only)

If the request fails due to connection error, timeout, or 403:

1. Read `lcd-discover/SKILL.md` and follow its procedure to find the device's current IP.
2. Update config with the new IP.
3. Retry the notification **once** with the new IP.
4. If retry also fails, report the error clearly.

### 6. Dry run

If the user asks for a dry run or test without sending, show the payload that would be
sent (with the token masked as `***`) and the target URL — but do not execute the curl.

## Important

- Never print or log the device token in any output.
- Keep this skill lean — it's the most frequently used one.
- If the cached IP is stale, this skill triggers rediscovery automatically.
