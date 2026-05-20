---
name: lcd-notify
description: >
  Send a notification to the user's LCD device. Supports plain text, rich layout
  (positioned text, shapes, progress bars), and buzzer sounds. Use whenever the user
  asks to display, show, push, or send content to their LCD, screen, or notification device.
  Also use proactively at the end of long tasks (build, test, deploy) to ping the LCD so
  the user knows without checking the terminal. Triggers: "show this on my screen",
  "notify my LCD", "ping my display when done", "let me know on the device".
---

# LCD Notification

Send a notification to a paired LCD device over the LAN.

## Config file

Path: `~/.config/autonomous-lcd.json` — must exist with at least one paired device.
If no config or no devices, tell the user to run **lcd-pair** first.

## Procedure

### 1. Load config and pick device

```python
import json
with open("~/.config/autonomous-lcd.json") as f:
    cfg = json.load(f)
```

Determine target device:
- Use the device ID the user specified, OR
- Use `default_device_id` from config, OR
- Error if neither is available.

Get the device's `last_known_ip` and `device_id` from the config.

### 2. Choose rendering mode

**Legacy text mode** — simple text notification:

```json
{
  "text": "Build passed in 3m 12s",
  "size": 2,
  "color": "green"
}
```

- `text` (required) — max 500 chars. Strip markdown to plain text.
- `size` (optional) — 1 to 4, default 2.
- `color` (optional) — accent color. Options: `red`, `green`, `blue`, `yellow`, `white`, or hex `#RRGGBB`. Default: `blue`.
- `title` (optional) — parsed but not shown by current UI theme.

**Rich layout mode** — positioned items with shapes and progress bars:

```json
{
  "items": [
    { "type": "text", "text": "Usage", "x": 12, "y": 0, "width": 196, "size": 4, "color": "#ffffff" },
    { "type": "rect", "x": 6, "y": 34, "width": 208, "height": 72, "radius": 10, "color": "#171d18" },
    { "type": "text", "text": "29%", "x": 18, "y": 44, "width": 72, "size": 4, "color": "#ffffff" },
    { "type": "progress", "x": 18, "y": 78, "width": 184, "height": 14, "radius": 4, "value": 29, "color": "#9bad67", "bg_color": "#2f2f2f" },
    { "type": "text", "text": "Current load", "x": 110, "y": 48, "width": 90, "align": "right", "size": 1, "color": "#d7d7d7" },
    { "type": "text", "text": "Resets in 2h 22m", "x": 18, "y": 98, "width": 160, "size": 1, "color": "#d7d7d7" }
  ]
}
```

When `items[]` has at least one valid item, rich layout is used and `text` is ignored.

**Coordinate system:**
- Viewport: `220 x 117` pixels (not the full 240x135 screen)
- Origin `(0, 0)` = top-left of content area
- Last usable point: `(219, 116)`

**Limits:**
- Max **6** items (extra ignored)
- Item text max **95** chars
- `value`: 0..100, `radius`: 0..50, `stroke_width`: 1..32

**Item types:** `text`, `rect`, `progress`, `line`, `circle`, `ellipse`, `arc`, `triangle`, `polygon`, `ring`, `pie`

See `DEVICE-PROTOCOL.md` section 3.1.1 for full field reference per item type.

**Buzzer sound** — add `play_sound` at root level (works with both modes):

```json
{
  "text": "Build passed",
  "play_sound": 20
}
```

- `play_sound` (optional) — preset index 0..20. Default notification sound: **20** (`claude_style`).
- Requires `text` or `items` in the same request — cannot be sent standalone.

### 3. Send notification

Use inline `python3` to send the request:

```python
import json, urllib.request

payload = json.dumps(<json_payload>).encode()
req = urllib.request.Request(
    "http://<last_known_ip>:3000/lcd",
    data=payload,
    headers={
        "Content-Type": "application/json",
        "X-Device-ID": "<device_id>",
    },
    method="POST",
)
with urllib.request.urlopen(req, timeout=3) as resp:
    print(f"HTTP {resp.status}")
    print(resp.read().decode())
```

### 4. Handle response

- **200** — Success. Update `last_seen_at` in config.
- **400** — Malformed payload. Check the JSON.
- **403** — Device ID mismatch. Run rediscovery (step 5).
- **413** — Text too long (>500 chars). Truncate to 497 + `"..."` and retry once.
- **Connection error / timeout** — Cached IP is stale. Run rediscovery (step 5).

### 5. Rediscovery and retry (on failure only)

If the request fails due to connection error, timeout, or 403:

1. Read `lcd-discover/SKILL.md` and follow its procedure to find the device's current IP.
2. Update config with the new IP.
3. Retry the notification **once** with the new IP.
4. If retry also fails, report the error clearly.

### 6. Dry run

If the user asks for a dry run or test without sending, show the payload that would be
sent and the target URL — but do not execute the request.

## Important

- Never print or log the device token in any output.
- Keep this skill lean — it's the most frequently used one.
- If the cached IP is stale, this skill triggers rediscovery automatically.
- Default buzzer sound is **20** (`claude_style`) unless user specifies otherwise.
