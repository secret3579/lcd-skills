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
| `text`  | Yes      | string    | Content to display. Max 500 characters.                                                     |
| `size`  | No       | int (1-4) | Font size. Default: 2.                                                                      |
| `color` | No       | string    | Header bar color. Valid values: `red`, `green`, `blue`, `yellow`, `white`. Default: `blue`. |
| `title` | No       | string    | Text shown in header bar. Default: none.                                                    |

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

### 3.1.1. Structured Layout Format

Instead of plain `text`, the client can send a `items` array for rich rendering with precise positioning.

**Request:**

```json
{
  "items": [
    {
      "type": "text",
      "text": "Usage",
      "x": 0,
      "y": 5,
      "width": 240,
      "align": "center",
      "size": 3,
      "color": "#ff4081"
    },
    {
      "type": "text",
      "text": "29%",
      "x": 18,
      "y": 42,
      "width": 100,
      "align": "left",
      "size": 4,
      "color": "#00e5ff"
    },
    {
      "type": "text",
      "text": "Current",
      "x": 120,
      "y": 50,
      "width": 120,
      "align": "right",
      "size": 1,
      "color": "#ffea00"
    },
    {
      "type": "progress",
      "value": 29,
      "x": 18,
      "y": 76,
      "width": 184,
      "height": 14,
      "radius": 4,
      "color": "#76ff03",
      "bg_color": "#2f2f2f"
    },
    {
      "type": "text",
      "text": "Resets in 2h 22m",
      "x": 18,
      "y": 98,
      "width": 200,
      "align": "left",
      "size": 1,
      "color": "#ff9100"
    }
  ]
}
```

**Example 2 — Weekly usage with status:**

```json
{
  "items": [
    {
      "type": "text",
      "text": "4%",
      "x": 18,
      "y": 5,
      "width": 100,
      "align": "left",
      "size": 4,
      "color": "#00e5ff"
    },
    {
      "type": "text",
      "text": "Weekly",
      "x": 120,
      "y": 13,
      "width": 120,
      "align": "right",
      "size": 1,
      "color": "#ffea00"
    },
    {
      "type": "progress",
      "value": 4,
      "x": 18,
      "y": 40,
      "width": 184,
      "height": 14,
      "radius": 4,
      "color": "#76ff03",
      "bg_color": "#2f2f2f"
    },
    {
      "type": "text",
      "text": "Resets in 6d 19h",
      "x": 18,
      "y": 58,
      "width": 200,
      "align": "left",
      "size": 1,
      "color": "#ff9100"
    },
    {
      "type": "text",
      "text": "* Baking...",
      "x": 0,
      "y": 78,
      "width": 240,
      "align": "center",
      "size": 2,
      "color": "#ff4081"
    }
  ]
}
```

**Component types:**

| Type | Description | Fields |
|------|-------------|--------|
| `text` | Render text at position | `text`, `x`, `y`, `width`, `align`, `size`, `color` |
| `progress` | Render progress bar | `value` (0-100), `x`, `y`, `width`, `height`, `radius`, `color`, `bg_color` |

**Common fields:**

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Component type (`text`, `progress`) |
| `x` | int | X position in pixels |
| `y` | int | Y position in pixels |
| `width` | int | Width in pixels |
| `color` | string | Hex color (e.g. `#9bad67`) |

**Text-specific fields:**

| Field | Type | Description |
|-------|------|-------------|
| `text` | string | Content to display |
| `align` | string | `left`, `center`, `right`. Default: `left` |
| `size` | int (1-4) | Font size. Default: 2 |

**Progress-specific fields:**

| Field | Type | Description |
|-------|------|-------------|
| `value` | int (0-100) | Progress percentage |
| `height` | int | Bar height in pixels |
| `radius` | int | Corner radius in pixels. Default: 0 |
| `bg_color` | string | Background/empty color (hex) |

**Notes:**
- When `items` is present, `text`/`size`/`color`/`title` fields are ignored.
- Components render in array order (first = bottom, last = top).
- Screen size: 135x240 pixels.

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
