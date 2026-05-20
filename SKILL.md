---
name: lcd
description: >
  Interact with the user's LCD notification device (ST7789, ESP32) over LAN.
  Supports discovery, pairing, and sending notifications with plain text,
  rich layout (positioned text, shapes, progress bars), and buzzer sounds.
  Triggers: "find my LCD", "pair my LCD", "show on my screen", "notify my LCD",
  "ping my display when done", "send to my screen", "rescan for device".
---

# LCD Skill

Discover, pair, and send notifications to an ST7789 LCD device on the local network.

## Config

Path: `~/.config/autonomous-lcd.json`

```json
{
  "devices": [
    {
      "device_id": "lcd-bd4a14",
      "label": "My LCD",
      "last_known_ip": "192.168.1.42",
      "last_seen_at": "2026-05-18T10:30:00Z"
    }
  ],
  "default_device_id": "lcd-bd4a14"
}
```

Device ID format: `lcd-<6 lowercase hex digits>` (from last 3 bytes of MAC address).

---

## 1. Discover

Find a paired device on the LAN and update its cached IP.

**When to use:** "find my LCD", "rescan", "LCD stopped responding", or when a notification fails with connection error/timeout.

### 1.1 Cache check (fast path)

```bash
curl -s --max-time 0.5 http://<last_known_ip>:3000/status
```

If `device_id` in response matches → update `last_seen_at`, done.

### 1.2 mDNS service browse

`dns-sd` on macOS runs interactively and never exits. Run it in background and kill after 3 seconds:

```bash
dns-sd -B _autonomous-lcd._tcp &
PID=$!
sleep 3
kill $PID 2>/dev/null
wait $PID 2>/dev/null
```

For each service found, resolve hostname then IP:

```bash
dns-sd -L "<service-name>" _autonomous-lcd._tcp &
PID=$!; sleep 3; kill $PID 2>/dev/null; wait $PID 2>/dev/null

dns-sd -G v4 <hostname> &
PID=$!; sleep 3; kill $PID 2>/dev/null; wait $PID 2>/dev/null
```

Match by `device_id` in TXT records. Parse IP from the `Add` line. If found → update config, done.

### 1.3 UDP probe sweep (fallback)

If mDNS finds nothing (multicast may be blocked):

```bash
python3 -c '
import socket, json, sys, concurrent.futures

TARGET = "<device_id>"
SUBNET = "<subnet_prefix>"  # e.g. "192.168.1"

def probe(ip):
    try:
        s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        s.settimeout(1)
        s.sendto(b"AUTONOMOUS_LCD_PROBE?", (ip, 49152))
        data, _ = s.recvfrom(1024)
        return json.loads(data), ip
    except Exception:
        return None, None
    finally:
        s.close()

candidates = [f"{SUBNET}.{i}" for i in range(1, 255)]
with concurrent.futures.ThreadPoolExecutor(max_workers=50) as pool:
    futures = [pool.submit(probe, ip) for ip in candidates]
    for f in concurrent.futures.as_completed(futures):
        info, ip = f.result()
        if info and info.get("device_id") == TARGET:
            print(json.dumps({"ip": ip, **info}))
            sys.exit(0)

print(json.dumps({"error": "not_found"}), file=sys.stderr)
sys.exit(1)
'
```

Get subnet from: `ifconfig | grep "inet " | grep -v 127.0.0.1`

Never scan a subnet larger than /22 without user confirmation.

### 1.4 Failure

Report which stages ran, how many IPs probed, and suggest: device powered on? Same WiFi?

---

## 2. Pair

One-time setup to register a new device.

**When to use:** "pair my LCD", "add a screen", "set up device".

### Procedure

1. Ask for **device ID** (format: `lcd-<6 hex>`, on sticker).
2. Ask for **label** (optional, default "My LCD").
3. Run discovery (section 1) to find the device IP.
4. Verify via `GET /status` that `device_id` matches.
5. Write to `~/.config/autonomous-lcd.json`:
   - New device → append to `devices[]`
   - Existing device ID → update entry
   - First device → set as `default_device_id`
   - Set file permission `0600`
6. Send confirmation notification: `{"text": "Paired with Claude", "color": "green", "play_sound": 20}`
7. Report success: device ID, label, IP.

Re-pairing same device ID is an update, not an error.

---

## 3. Notify

Send a notification to a paired device. This is the most frequently used operation.

**When to use:** "show on my screen", "notify my LCD", "ping my display when done". Also use proactively at end of long tasks (build, test, deploy).

### 3.1 Pick device

- Use device ID the user specified, OR
- Use `default_device_id` from config, OR
- Error if neither available. If no config or no devices → tell user to pair first.

### 3.2 Build payload

**Legacy text mode** — simple notification:

```json
{
  "text": "Build passed in 3m 12s",
  "size": 2,
  "color": "green",
  "play_sound": 20
}
```

- `text` (required) — max 500 chars, strip markdown to plain text
- `size` (optional) — 1 to 4, default 2
- `color` (optional) — `red`, `green`, `blue`, `yellow`, `white`, or hex `#RRGGBB`
- `play_sound` — always include **20** (`claude_style`) unless user specifies otherwise

**Rich layout mode** — positioned items:

```json
{
  "play_sound": 20,
  "items": [
    { "type": "text", "text": "Title", "x": 12, "y": 0, "width": 196, "size": 4, "color": "#ffffff" },
    { "type": "rect", "x": 6, "y": 34, "width": 208, "height": 72, "radius": 10, "color": "#171d18" },
    { "type": "progress", "x": 18, "y": 78, "width": 184, "height": 14, "radius": 4, "value": 50, "color": "#9bad67", "bg_color": "#2f2f2f" }
  ]
}
```

When `items[]` has at least one valid item, rich layout is used and `text` is ignored.

**Coordinate system:**
- Viewport: **220 x 117** pixels (inner content area, not full 240x135 screen)
- Origin `(0, 0)` = top-left of content area
- Last usable point: `(219, 116)`

**Limits:**
- Max **6** items (extra ignored)
- Item text max **95** chars
- `value`: 0..100, `radius`: 0..50, `stroke_width`: 1..32

### 3.3 Send

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

### 3.4 Handle response

| Status | Action |
|--------|--------|
| **200** | Success. Update `last_seen_at` in config. |
| **400** | Malformed payload. Check JSON. |
| **403** | Device ID mismatch. Run rediscovery. |
| **413** | Text >500 chars. Truncate to 497 + `"..."`, retry once. |
| **Connection error/timeout** | Cached IP stale. Run rediscovery (section 1), retry once. |

### 3.5 Dry run

If user asks for dry run, show the payload and target URL without sending.

---

## 4. Rich Layout Reference

### Item types

**text** — Render text at position

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `text` | string | *(required)* | Text content (max 95 chars) |
| `size` | int 1-4 | 2 | Font size |
| `align` | string | `left` | `left`, `center`, `right` |
| `width` | int | 0 | Layout width (0 = remaining) |
| `color` | string | `blue` | Text color |

**rect** — Rectangle or rounded rectangle

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `width` | int | *(required)* | Width |
| `height` | int | *(required)* | Height |
| `radius` | int | 0 | Corner radius |
| `filled` | bool | true | Fill or outline |

**progress** — Progress bar

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `width` | int | *(required)* | Width |
| `height` | int | *(required)* | Height |
| `value` | int 0-100 | *(required)* | Percentage |
| `radius` | int | 0 | Corner radius |
| `bg_color` | string | `#303030` | Track color |

**line** — Line between two points

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `x2` | int | *(required)* | End X |
| `y2` | int | *(required)* | End Y |
| `stroke_width` | int | 1 | Thickness |

**circle** — Circle (set width = height)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `width` | int | *(required)* | Diameter |
| `height` | int | *(required)* | = width |
| `filled` | bool | true | Fill or outline |

**ellipse** — Same fields as circle, different width/height.

**arc** — Arc stroke over angle range

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `width` | int | *(required)* | Bounding width |
| `height` | int | *(required)* | Bounding height |
| `start_angle` | int | 0 | Start angle |
| `end_angle` | int | 360 | End angle |
| `stroke_width` | int | 1 | Arc thickness |

**triangle** — Triangle from 3 points

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `x2` | int | *(required)* | Point 2 X |
| `y2` | int | *(required)* | Point 2 Y |
| `x3` | int | *(required)* | Point 3 X |
| `y3` | int | *(required)* | Point 3 Y |
| `filled` | bool | true | Fill or outline |

**polygon** — Polygon from point list

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `points` | string | *(required)* | `x1,y1; x2,y2; x3,y3 ...` (max 95 chars) |
| `filled` | bool | true | Fill or outline |

**ring** — Circular outline over angle range

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `width` | int | *(required)* | Bounding width |
| `height` | int | *(required)* | Bounding height |
| `start_angle` | int | 0 | Start angle |
| `end_angle` | int | 360 | End angle (360 = full ring) |
| `stroke_width` | int | 1 | Ring thickness |

**pie** — Filled wedge shape

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `width` | int | *(required)* | Bounding width |
| `height` | int | *(required)* | Bounding height |
| `start_angle` | int | 0 | Start angle |
| `end_angle` | int | 360 | End angle |

### Common fields (all item types)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `type` | string | `text` | Item type |
| `x` | int | 0 | X position in viewport |
| `y` | int | 0 | Y position in viewport |
| `color` | string | root `color` | Item color |
| `bg_color` | string | `#303030` | Background (progress) |

---

## 5. Buzzer Sound Presets

`play_sound` can be included in any request (both legacy and rich mode). Plays once when payload is accepted. Requires `text` or `items` — cannot be sent standalone.

| Index | Name | Style |
|-------|------|-------|
| 0 | *(muted)* | Sound off |
| 1 | `triple_ping` | Sharp high triple ping |
| 2 | `lift_chime` | Smooth rising chime |
| 3 | `bright_fanfare` | Bright success fanfare |
| 4 | `micro_success` | Short positive triad |
| 5 | `soft_drop` | Soft descending confirmation |
| 6 | `double_tick` | Quick double tick and resolve |
| 7 | `alert_fall` | Short alert drop |
| 8 | `clean_pop` | Modern light popup sound |
| 9 | `confirm_arc` | Rising confirm arc |
| 10 | `status_ready` | Compact ready cue |
| 11 | `soft_descend` | Calm descending tone |
| 12 | `gentle_open` | Friendly open cue |
| 13 | `focus_ping` | Clean focused ping |
| 14 | `tap_rise` | Tap then rise |
| 15 | `resolve_down` | Downward resolve |
| 16 | `signal_up` | Fast upward signal |
| 17 | `digital_bloom` | Slightly synthetic bloom |
| 18 | `notify_peak` | Peak notification tone |
| 19 | `warning_soft` | Soft warning fall |
| 20 | `claude_style` | Crisp bright code-style notification |

Default notification sound: **20** (`claude_style`).

---

## 6. Tested Examples

Copy-paste ready payloads for quick testing.

### Example 1 — Header + Current (5-hour usage)

```json
{
  "play_sound": 20,
  "items": [
    { "type": "text", "text": "👾 Usage", "x": 0, "y": 0, "width": 220, "align": "center", "size": 4, "color": "#e8dcc8" },
    { "type": "rect", "x": 6, "y": 30, "width": 208, "height": 82, "radius": 10, "color": "#1e2a22" },
    { "type": "text", "text": "41%", "x": 18, "y": 36, "width": 100, "size": 4, "color": "#e8dcc8" },
    { "type": "text", "text": "Current", "x": 100, "y": 44, "width": 105, "align": "right", "size": 1, "color": "#a09888" },
    { "type": "progress", "x": 18, "y": 72, "width": 184, "height": 12, "radius": 6, "value": 41, "color": "#6b8f4e", "bg_color": "#3a3a3a" },
    { "type": "text", "text": "Resets in 1h 56m", "x": 18, "y": 92, "width": 180, "size": 1, "color": "#9a9488" }
  ]
}
```

### Example 2 — Weekly (7-day usage) + status

```json
{
  "play_sound": 20,
  "items": [
    { "type": "rect", "x": 6, "y": 0, "width": 208, "height": 82, "radius": 10, "color": "#1e2a22" },
    { "type": "text", "text": "48%", "x": 18, "y": 6, "width": 100, "size": 4, "color": "#e8dcc8" },
    { "type": "text", "text": "Weekly", "x": 100, "y": 14, "width": 105, "align": "right", "size": 1, "color": "#a09888" },
    { "type": "progress", "x": 18, "y": 42, "width": 184, "height": 12, "radius": 6, "value": 48, "color": "#6b8f4e", "bg_color": "#3a3a3a" },
    { "type": "text", "text": "Resets in 1d 11h", "x": 18, "y": 62, "width": 180, "size": 1, "color": "#9a9488" },
    { "type": "text", "text": "* Baking...", "x": 0, "y": 92, "width": 220, "align": "center", "size": 2, "color": "#d4845a" }
  ]
}
```

### Example 3 — Simple text notification

```json
{
  "text": "Build passed in 3m 12s",
  "size": 2,
  "color": "green",
  "play_sound": 20
}
```

### Example 4 — Shape demo

```json
{
  "play_sound": 20,
  "items": [
    { "type": "circle", "x": 10, "y": 12, "width": 28, "height": 28, "color": "#7cb342", "filled": true },
    { "type": "ellipse", "x": 52, "y": 10, "width": 48, "height": 30, "color": "#ffffff", "filled": false, "stroke_width": 2 },
    { "type": "line", "x": 8, "y": 64, "x2": 206, "y2": 64, "color": "#4fc3f7", "stroke_width": 3 },
    { "type": "arc", "x": 118, "y": 10, "width": 38, "height": 38, "color": "#ffb300", "stroke_width": 4, "start_angle": 0, "end_angle": 270 },
    { "type": "triangle", "x": 22, "y": 84, "x2": 44, "y2": 112, "x3": 4, "y3": 112, "color": "#ef5350", "filled": true },
    { "type": "pie", "x": 166, "y": 82, "width": 36, "height": 36, "color": "#66bb6a", "start_angle": 0, "end_angle": 120 }
  ]
}
```

---

## 7. Proactive / Scheduled Notifications

Send notifications at regular intervals without user interaction. Useful for dashboards, monitoring, or periodic status updates.

**When to use:** "send every 5 minutes", "update my LCD every hour", "keep my screen updated", "monitor and push to LCD", "show my usage on LCD".

### 7.1 Claude Code Usage Monitor

Fetch real usage data from the Claude Code API and display on LCD. No external scripts needed — uses the local OAuth token directly.

**Get OAuth token:**

```python
import json, os, subprocess

def get_token():
    # Linux / WSL: plaintext credentials file
    cred_path = os.path.expanduser("~/.claude/.credentials.json")
    if os.path.exists(cred_path):
        with open(cred_path) as f:
            data = json.load(f)
        for v in _find_strings(data):
            if v.startswith("sk-ant-oat"):
                return v

    # macOS: stored in login Keychain
    try:
        kc = subprocess.run(
            ["security", "find-generic-password", "-s", "Claude Code-credentials", "-w"],
            capture_output=True, text=True
        )
        if kc.returncode == 0:
            data = json.loads(kc.stdout.strip())
            for v in _find_strings(data):
                if v.startswith("sk-ant-oat"):
                    return v
    except Exception:
        pass
    return None

def _find_strings(obj):
    if isinstance(obj, str):
        yield obj
    elif isinstance(obj, dict):
        for v in obj.values():
            yield from _find_strings(v)
    elif isinstance(obj, list):
        for v in obj:
            yield from _find_strings(v)
```

**Fetch usage and build payload:**

```python
import urllib.request
from datetime import datetime, timezone

def fetch_usage(token):
    req = urllib.request.Request(
        "https://api.anthropic.com/api/oauth/usage",
        headers={
            "Authorization": f"Bearer {token}",
            "anthropic-beta": "oauth-2025-04-20",
        },
    )
    with urllib.request.urlopen(req, timeout=15) as resp:
        return json.loads(resp.read())

def time_left(iso_str):
    if not iso_str:
        return "N/A"
    diff = datetime.fromisoformat(iso_str) - datetime.now(timezone.utc)
    total_sec = int(diff.total_seconds())
    if total_sec <= 0:
        return "now"
    h = total_sec // 3600
    m = (total_sec % 3600) // 60
    if h > 24:
        return f"{h // 24}d {h % 24}h"
    return f"{h}h {m}m"

token = get_token()
usage = fetch_usage(token)
pct_5h = int(usage["five_hour"]["utilization"])
pct_7d = int(usage["seven_day"]["utilization"])
reset_5h = time_left(usage["five_hour"]["resets_at"])
reset_7d = time_left(usage["seven_day"]["resets_at"])

# Section 1: Header + Current (5-hour)
section1 = {
    "play_sound": 20,
    "items": [
        { "type": "text", "text": "👾 Usage", "x": 0, "y": 0, "width": 220, "align": "center", "size": 4, "color": "#e8dcc8" },
        { "type": "rect", "x": 6, "y": 30, "width": 208, "height": 82, "radius": 10, "color": "#1e2a22" },
        { "type": "text", "text": f"{pct_5h}%", "x": 18, "y": 36, "width": 100, "size": 4, "color": "#e8dcc8" },
        { "type": "text", "text": "Current", "x": 100, "y": 44, "width": 105, "align": "right", "size": 1, "color": "#a09888" },
        { "type": "progress", "x": 18, "y": 72, "width": 184, "height": 12, "radius": 6, "value": pct_5h, "color": "#6b8f4e", "bg_color": "#3a3a3a" },
        { "type": "text", "text": f"Resets in {reset_5h}", "x": 18, "y": 92, "width": 180, "size": 1, "color": "#9a9488" }
    ]
}

# Section 2: Weekly (7-day) + status
section2 = {
    "play_sound": 20,
    "items": [
        { "type": "rect", "x": 6, "y": 0, "width": 208, "height": 82, "radius": 10, "color": "#1e2a22" },
        { "type": "text", "text": f"{pct_7d}%", "x": 18, "y": 6, "width": 100, "size": 4, "color": "#e8dcc8" },
        { "type": "text", "text": "Weekly", "x": 100, "y": 14, "width": 105, "align": "right", "size": 1, "color": "#a09888" },
        { "type": "progress", "x": 18, "y": 42, "width": 184, "height": 12, "radius": 6, "value": pct_7d, "color": "#6b8f4e", "bg_color": "#3a3a3a" },
        { "type": "text", "text": f"Resets in {reset_7d}", "x": 18, "y": 62, "width": 180, "size": 1, "color": "#9a9488" },
        { "type": "text", "text": "* Baking...", "x": 0, "y": 92, "width": 220, "align": "center", "size": 2, "color": "#d4845a" }
    ]
}
```

**Rate limit:** The usage API rate-limits hard. Do NOT poll faster than once per minute. Recommended interval: **5 minutes**.

### 7.2 Background Loop Pattern

Run a background process that sends at a fixed interval:

```python
import json, urllib.request, time, os

INTERVAL = 300  # seconds (5 minutes)

with open(os.path.expanduser("~/.config/autonomous-lcd.json")) as f:
    cfg = json.load(f)
device = cfg["devices"][0]
ip = device["last_known_ip"]
device_id = device["device_id"]

def send(payload):
    try:
        req = urllib.request.Request(
            f"http://{ip}:3000/lcd",
            data=json.dumps(payload).encode(),
            headers={"Content-Type": "application/json", "X-Device-ID": device_id},
            method="POST",
        )
        with urllib.request.urlopen(req, timeout=3) as resp:
            return resp.status
    except Exception:
        return None

first = True
while True:
    payload = build_payload()  # replace with actual payload logic
    if first:
        payload["play_sound"] = 20
        first = False
    send(payload)
    time.sleep(INTERVAL)
```

### Guidelines

- Use `play_sound` only on the **first** send or when status changes — avoid buzzing every interval.
- Always wrap `send()` in try/except so timeouts don't kill the loop.
- For one-shot delayed sends, use `time.sleep(delay)` before a single send instead of a loop.
- If the user asks to stop, kill the background process.
- Recommended intervals: 60s minimum. The usage API enforces at least 60s between calls.

---

## Important

- V1: no authentication — only `X-Device-ID` header required (no `X-Token`).
- Always include `"play_sound": 20` unless user specifies otherwise.
- Never print or log device tokens.
- Config file permission should be `0600`.
- If cached IP is stale, run rediscovery automatically.
- Never scan a subnet larger than /22 without user confirmation.
