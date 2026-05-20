# LCD Device Protocol Specification

> This document is for the hardware team. It describes the full protocol that the ESP32 firmware must implement to communicate with clients on the LAN.

---

## 1. Overview

The device must expose 3 interfaces:

| Interface          | Purpose                              | Port            |
| ------------------ | ------------------------------------ | --------------- |
| HTTP Server        | Receive notifications + health check | 3000            |
| UDP Listener       | Reply to discovery probes            | 49152           |
| mDNS Advertisement | Advertise service on LAN             | 5353 (standard) |

---

## 2. Device ID

**Format:** `lcd-<6 lowercase hex characters>`

**How to generate:** Take the last 3 bytes of the WiFi MAC address, convert to lowercase hex.

**Example:** MAC = `AA:BB:CC:A3:F9:C1` → Device ID = `lcd-a3f9c1`

The device ID is **fixed** and does not change across reboots. Print it on a sticker attached to the device.

---

## 3. HTTP Server (port 3000)

### 3.1. POST /lcd — Display notification

Client sends text to be displayed on the LCD screen.

**Request:**

```
POST /lcd HTTP/1.1
Content-Type: application/json
X-Device-ID: <device_id>
```

note

```json
{
  "text": "Build passed in 3m 12s",
  "size": 2,
  "color": "green",
  "title": "CI"
}
```

**Body fields:**

| Field   | Required | Type      | Description                                                                                 |
| ------- | -------- | --------- | ------------------------------------------------------------------------------------------- |
| `text`  | Yes (legacy mode) | string | Content to display. Max 500 chars. Required when rich layout not used. |
| `size`  | No | int (1-4) | Font size. Default: 2. Also default size for text items in rich mode. |
| `color` | No | string | Accent bar color (legacy) or default item color (rich). Default: `blue`. |
| `title` | No | string | Parsed but not shown by current UI theme. |
| `items` | Yes (rich mode) | array | List of items to render (max 6). See section 3.1.1. |
| `play_sound` | No | int (0-20) | Play buzzer preset once when payload is accepted. |

**Response:**

| Status                  | When                                                | Body                                     |
| ----------------------- | --------------------------------------------------- | ---------------------------------------- |
| `200 OK`                | Text rendered on LCD successfully                   | `{"status": "ok"}`                       |
| `400 Bad Request`       | Malformed JSON or missing `text` field              | `{"error": "missing field: text"}`       |
| `403 Forbidden`         | `X-Device-ID` does not match the device's actual ID | `{"error": "device id mismatch"}`        |
| `413 Payload Too Large` | `text` exceeds 500 characters                       | `{"error": "text too long", "max": 500}` |

**Processing order (check header before parsing body to save RAM on ESP32):**

1. Check `X-Device-ID` header matches device ID. If mismatch → 403.
2. Parse JSON body. If invalid → 400.
3. Check `text` field exists and is not empty. If missing → 400.
4. Check `text` ≤ 500 characters. If exceeded → 413.
5. If `items` array present → render structured layout. Else render `text` → 200.

---

### 3.1.1. Rich Layout Mode

Instead of plain `text`, the client can send an `items[]` array for rich rendering with precise positioning.

If `items[]` contains at least one valid item, the firmware renders `items[]` and ignores the legacy `text` field.

**Coordinate system:** The renderer draws inside an inner content area:
- Origin: `(0, 0)` top-left of content area
- Usable area: `220 x 117` pixels (not the full `240 x 135` screen)
- Bottom-right usable point: `(219, 116)`

**Limits:**
- Maximum **6** items in `items[]` (extra items ignored)
- Item `text` max length: **95** characters
- `polygon.points` max string length: **95** characters
- `play_sound`: preset index `0..20`
- Colors: named (`red`, `green`, `blue`, `yellow`, `white`) or hex `#RRGGBB` (invalid falls back to `blue`)
- `value`: clamped `0..100`, `radius`: clamped `0..50`, `stroke_width`: clamped `1..32`

#### Common item fields

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `type` | No | string | `text` | Unknown types treated as text if `text` present |
| `color` | No | string | root `color` | Main item color |
| `bg_color` | No | string | `#303030` | Background color (used by `progress`) |
| `x` | No | int | `0` | X in 220x117 viewport |
| `y` | No | int | `0` | Y in 220x117 viewport |
| `width` | No | int | `0` | For text: 0 = remaining width. Clamped to 0..240 |
| `height` | No | int | `0` | Clamped to 0..135 |
| `stroke_width` | No | int | `1` | Clamped to 1..32 |
| `filled` | No | bool | `true` | Fill-capable shapes |
| `start_angle` | No | int | `0` | For `arc`, `ring`, `pie` |
| `end_angle` | No | int | `360` | For `arc`, `ring`, `pie`. 360 = full shape |

#### Item types

**text** — Render text at position

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `text` | Yes | string | Text content |
| `size` | No | int (1-4) | Font size (1=small, 2=medium, 3=large, 4=extra large) |
| `align` | No | string | `left`, `center`, `right` |
| `width` | No | int | Text layout width |

**rect** — Rectangle or rounded rectangle

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `width` | Yes | int | Width |
| `height` | Yes | int | Height |
| `radius` | No | int | Corner radius |
| `filled` | No | bool | `true` for filled, `false` for outline |

**progress** — Progress bar

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `width` | Yes | int | Width |
| `height` | Yes | int | Height |
| `value` | Yes | int (0-100) | Progress percentage |
| `radius` | No | int | Corner radius |
| `bg_color` | No | string | Background track color |

**line** — Line between two points

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `x2` | Yes | int | End point X |
| `y2` | Yes | int | End point Y |
| `stroke_width` | No | int | Line thickness |

**circle** — Circle

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `width` | Yes | int | Diameter |
| `height` | Yes | int | Should match `width` |
| `filled` | No | bool | Filled or outlined |
| `stroke_width` | No | int | Border thickness |

**ellipse** — Ellipse (same fields as circle, different width/height)

**arc** — Arc stroke over angle range

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `width` | Yes | int | Bounding box width |
| `height` | Yes | int | Bounding box height |
| `start_angle` | Yes | int | Start angle |
| `end_angle` | Yes | int | End angle |
| `stroke_width` | No | int | Arc thickness |

**triangle** — Triangle from 3 points

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `x2` | Yes | int | Point 2 X |
| `y2` | Yes | int | Point 2 Y |
| `x3` | Yes | int | Point 3 X |
| `y3` | Yes | int | Point 3 Y |
| `filled` | No | bool | Filled or outlined |

**polygon** — Polygon from point list

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `points` | Yes | string | Format: `x1,y1; x2,y2; x3,y3 ...` |
| `filled` | No | bool | Filled or outlined |
| `stroke_width` | No | int | Border thickness |

**ring** — Circular/elliptical outline over angle range

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `width` | Yes | int | Bounding box width |
| `height` | Yes | int | Bounding box height |
| `start_angle` | Yes | int | Start angle |
| `end_angle` | Yes | int | End angle (360 for full ring) |
| `stroke_width` | No | int | Ring thickness |

**pie** — Filled wedge shape

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `width` | Yes | int | Bounding box width |
| `height` | Yes | int | Bounding box height |
| `start_angle` | Yes | int | Start angle |
| `end_angle` | Yes | int | End angle (360 for full pie) |

#### Example — Compact usage card

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

#### Example — Shape demo

```json
{
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

#### Example — LCD plus buzzer

```json
{
  "play_sound": 20,
  "items": [
    { "type": "text", "text": "Build passed", "x": 12, "y": 18, "width": 180, "size": 3, "color": "#ffffff" },
    { "type": "text", "text": "Deploy complete", "x": 12, "y": 52, "width": 180, "size": 1, "color": "#9ad0a5" }
  ]
}
```

---

### 3.1.2. Buzzer Sound Presets

`play_sound` can be included in any `POST /lcd` request (both legacy and rich mode). It plays once when the payload is accepted. Requires `text` or `items` — cannot be sent standalone.

| Index | Name | Style |
|-------|------|-------|
| `0` | *(muted)* | Sound off |
| `1` | `triple_ping` | Sharp high triple ping |
| `2` | `lift_chime` | Smooth rising chime |
| `3` | `bright_fanfare` | Bright success fanfare |
| `4` | `micro_success` | Short positive triad |
| `5` | `soft_drop` | Soft descending confirmation |
| `6` | `double_tick` | Quick double tick and resolve |
| `7` | `alert_fall` | Short alert drop |
| `8` | `clean_pop` | Modern light popup sound |
| `9` | `confirm_arc` | Rising confirm arc |
| `10` | `status_ready` | Compact ready cue |
| `11` | `soft_descend` | Calm descending tone |
| `12` | `gentle_open` | Friendly open cue |
| `13` | `focus_ping` | Clean focused ping |
| `14` | `tap_rise` | Tap then rise |
| `15` | `resolve_down` | Downward resolve |
| `16` | `signal_up` | Fast upward signal |
| `17` | `digital_bloom` | Slightly synthetic bloom |
| `18` | `notify_peak` | Peak notification tone |
| `19` | `warning_soft` | Soft warning fall |
| `20` | `claude_style` | Crisp bright code-style notification |

**Recommended for Claude Code notifications:** `4`, `8`, `13`, `16`, `20`

**Notes:**
- If a sound is already playing, new `play_sound` waits until the player is idle
- `play_sound` requires `text` or `items` in the same request

---

### 3.2. GET /status — Health check

Client checks if the device is alive and retrieves basic information.

**Request:**

```
GET /status HTTP/1.1
X-Device-ID: <device_id>   (optional)
```

**No `X-Token` required** — this endpoint is public on the LAN for discovery purposes.

**Response 200:**

```json
{
  "device_id": "lcd-a3f9c1",
  "model": "esp32-st7789-1.14",
  "uptime_seconds": 12453,
  "free_heap_bytes": 138240,
  "wifi_rssi": -54
}
```

| Field             | Type   | Description                                                  |
| ----------------- | ------ | ------------------------------------------------------------ |
| `device_id`       | string | Device ID                                                    |
| `model`           | string | Model identifier. Format: `esp32-st7789-1.14`                |
| `uptime_seconds`  | int    | Seconds since last boot                                      |
| `free_heap_bytes` | int    | Free RAM (bytes)                                             |
| `wifi_rssi`       | int    | WiFi signal strength (dBm, negative, closer to 0 = stronger) |

---

## 4. UDP Discovery (port 49152)

The device listens for UDP on port **49152**. When a probe is received, reply immediately.

### Flow:

```
Client                          Device
  │                               │
  │──── UDP "AUTONOMOUS_LCD_PROBE?" ────────→│  (unicast to IP:49152)
  │                               │
  │←──── UDP JSON reply ──────────│  (unicast reply)
  │                               │
```

### Probe (client → device):

- **Payload:** exactly 21 bytes ASCII: `AUTONOMOUS_LCD_PROBE?`
- **Port:** 49152
- **Protocol:** UDP unicast

### Reply (device → client):

The device must only reply if the received payload is **exactly** `AUTONOMOUS_LCD_PROBE?` (21 bytes, case-sensitive).

**Payload (JSON):**

```json
{
  "device_id": "lcd-a3f9c1",
  "http_port": 3000,
  "model": "esp32-st7789-1.14"
}
```

| Field       | Type   | Description                          |
| ----------- | ------ | ------------------------------------ |
| `device_id` | string | Device ID                            |
| `http_port` | int    | Port the HTTP server is listening on |
| `model`     | string | Model identifier                     |

### Important notes:

- Reply via **unicast** to the client's IP:port (source address from the UDP packet).
- **Do not reply** if the payload does not match `AUTONOMOUS_LCD_PROBE?` — prevents abuse.
- No auth required for discovery — only public information is returned (device ID, port, model). Never return the token or secret.
- Reply within **< 100ms** to prevent client timeout.

---

## 5. mDNS Service Advertisement

The device advertises its service via mDNS (Multicast DNS) so clients can find it without scanning the entire subnet.

### Service registration:

| Parameter    | Value                             |
| ------------ | --------------------------------- |
| Service type | `_autonomous-lcd._tcp`            |
| Service name | `<device_id>` (e.g. `lcd-a3f9c1`) |
| Port         | `3000`                            |
| Domain       | `local`                           |

### TXT records:

```
device_id=lcd-a3f9c1
model=esp32-st7789-1.14
version=1.0.0
```

| Key         | Description               |
| ----------- | ------------------------- |
| `device_id` | Device ID                 |
| `model`     | Model identifier          |
| `version`   | Firmware version (semver) |

### Notes:

- Register mDNS immediately after WiFi connects.
- Unregister when WiFi disconnects.
- mDNS may be blocked on some enterprise networks — that is why the UDP fallback in section 4 exists.

---

## 6. Authentication

**V1: no authentication.** The device accepts any request with a valid `X-Device-ID`. This is sufficient for initial testing and development.

Token-based auth (`X-Token` header) and challenge-response will be added in v2. See section 10 (Security Roadmap) for details.

---

## 7. Rendering on LCD (ST7789 135x240)

This section is a suggestion — the hardware team decides the exact rendering details.

### Suggested layout:

```
┌──────────────────────────┐
│  ██ Title           ██   │  ← Header bar (color = color field)
├──────────────────────────┤
│                          │
│   Text content here      │  ← Body text (size = size field)
│   wraps to multiple      │
│   lines as needed        │
│                          │
└──────────────────────────┘
     135px wide × 240px tall
```

### Estimated character capacity:

| Size | Approximate characters that fit |
| ---- | ------------------------------- |
| 1    | ~330 characters                 |
| 2    | ~165 characters                 |
| 3    | ~80 characters                  |
| 4    | ~40 characters                  |

Text longer than the display area: truncate or scroll — up to the team to decide.

---

## 8. Startup sequence

```
1. Boot ESP32
2. Connect WiFi
3. Start HTTP server (port 3000)
4. Start UDP listener (port 49152)
5. Register mDNS service (_autonomous-lcd._tcp)
6. (Optional) Display device_id + IP on LCD so the user can see it
7. Ready — wait for requests
```

---

## 9. Error handling

| Situation              | What the device should do                                          |
| ---------------------- | ------------------------------------------------------------------ |
| WiFi disconnected      | Attempt reconnect. Unregister mDNS. Show status on LCD.            |
| Malformed HTTP request | Return appropriate status code (400/403/413). Do not crash.        |
| Invalid UDP probe      | Ignore silently, do not reply.                                     |
| Low RAM                | Must still respond to `/status`. May reject `/lcd` with 503.       |
| Concurrent requests    | Process sequentially (serial). No concurrency needed — only 1 LCD. |

---

## 10. Security — Roadmap (not required for v1)

The current auth (shared secret via `X-Token`) is sufficient for v1 — prevents accidental
cross-talk between devices on the same LAN.

Security improvements for v2:

- **Shared secret (`X-Token`)**: device stores a secret, client sends it via header, device validates. Prevents unauthorized writes.
- **Challenge-response**: replace `X-Token` plaintext with `HMAC-SHA256(token, nonce+body)` — token never goes over the network.
- **Mutual auth**: add `GET /verify?nonce=x` so the client can verify the device is genuine before sending data.
- **Replay protection**: single-use nonce, expires after 30 seconds.

---

## 11. Summary — Implementation checklist

- [ ] HTTP `POST /lcd` — receive JSON, validate, render, return correct status codes
- [ ] HTTP `GET /status` — return JSON with device info
- [ ] UDP port 49152 — receive `AUTONOMOUS_LCD_PROBE?`, reply with JSON
- [ ] mDNS — advertise `_autonomous-lcd._tcp` with TXT records
- [ ] Device ID — generate from last 3 bytes of MAC, format `lcd-xxxxxx`
- [ ] `X-Device-ID` header check on POST /lcd (403 on mismatch)
