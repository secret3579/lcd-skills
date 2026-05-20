# Claude Code Build Spec — LCD Device Skills

> **Purpose of this file.** This is a build spec for Claude Code. Read it end-to-end before writing any code. It describes three Claude skills you will create that let any Claude session discover, pair with, and send notifications to a personal ST7789 LCD device on the local network. The architecture decisions in this spec are settled — do not re-litigate them. Focus on producing high-quality skills that follow the contracts described here.

---

## 1. Background

The user has an embedded device with these properties:

- **MCU**: ESP-family (ESP32 or similar), WiFi-enabled.
- **Display**: 1.14″ 135×240 IPS SPI LCD, ST7789 driver.
- **Existing transport**: MQTT to a global broker, used by other features and the user's mobile app. **Do not touch the MQTT path.** The device will continue to use it for non-Claude features.

The Claude-side integration uses a **separate, parallel path on the LAN**: an HTTP server on the device and an MCP server on the developer's machine. The cloud HTTP path through the existing backend is also available, but the skills built here target the **LAN HTTP path** specifically — that is the path with zero per-call cost and confirmed delivery on the local network.

Three skills are needed, each scoped to one job:

1. **`lcd-discover`** — find the device on the LAN given a `device_id`.
2. **`lcd-pair`** — establish trust between Claude Code and a specific device (shared secret + device ID, written to local config).
3. **`lcd-notify`** — send a text notification to a paired device.

These are separate skills, not one combined skill, because:

- Discovery is expensive (subnet scan) and should only run when needed.
- Pairing is a one-time setup that shouldn't reload every conversation.
- Notification is the hot path and must be lean.

Separate skills also give the user clean triggering: "discover my LCD" loads only `lcd-discover`, not the others.

---

## 2. Architecture summary (read before coding)

```
Claude session
  ↓ (skill invokes local subprocess / MCP tool)
Local Python helper on developer machine
  ↓ HTTP POST over LAN
  ↓   http://<ip>:3000/lcd
  ↓   Headers: X-Token, X-Device-ID
ESP device (HTTP server on port 3000)
  ↓ HTTP 200 OK on success
ST7789 LCD renders the message
```

### Why this design

- **No internet required** — works fully on the office LAN, on a flight, behind any firewall.
- **No per-call server cost** — does not touch the production backend.
- **Confirmed delivery** — HTTP status tells Claude whether the message reached the screen.
- **No BLE** — Bluetooth SIG licensing is out of scope for this project.
- **No hostnames** — `.local` is blocked on many enterprise networks and creates collisions in shared offices. Devices are addressed by IP, identified by `device_id`.

### Discovery: mDNS service browse → unicast UDP scan

The discover skill uses a **two-stage fallback**:

1. **Primary — mDNS service browse.** Query service type `_autonomous-lcd._tcp` on multicast UDP port 5353. Read each device's IP, port, and TXT records. Match by `device_id`. Fast (<200 ms) when it works.
2. **Fallback — active unicast UDP scan.** When mDNS returns nothing (multicast blocked, AP isolation), enumerate every host IP in the local subnet, send a unicast UDP probe to each on port 49152, and match the reply by `device_id`. Slower (~0.5–1.5 s for a /24) but works on restrictive networks. Cache the result.

Neither stage uses hostnames. Matching is always by `device_id`.

### Auth and safety

- **Shared secret** in the `X-Token` HTTP header. Prevents accidental writes from unrelated devices on the LAN.
- **Device ID check** in the `X-Device-ID` header. Even if the wrong IP is reached, the firmware returns 403 on ID mismatch — your message will never accidentally land on a colleague's screen.

---

## 3. Contracts you can rely on

The firmware exposes the following endpoints. These are part of the spec — they are stable. Build skills against them.

### HTTP — display notification

```
POST http://<device-ip>:3000/lcd
Headers:
  Content-Type: application/json
  X-Token: <shared secret from pairing>
  X-Device-ID: <device_id>

Body:
{
  "text": "Build #2451 passed in 3m 12s",
  "size": 2,                  // optional, 1–4, default 2
  "color": "green",           // optional, header bar color
  "title": "CI"               // optional, shown in header bar
}

Responses:
  200 OK         — message rendered
  400 Bad Request — malformed JSON or missing required fields
  401 Unauthorized — X-Token mismatch
  403 Forbidden  — X-Device-ID mismatch
  413 Payload Too Large — text too long (>500 chars)
```

### HTTP — status / health check

```
GET http://<device-ip>:3000/status
Headers:
  X-Device-ID: <device_id>   // optional

Response 200:
{
  "device_id": "lcd-a3f9c1",
  "model": "esp32-st7789-1.14",
  "uptime_seconds": 12453,
  "free_heap_bytes": 138240,
  "wifi_rssi": -54
}
```

### UDP — discovery probe

```
Send (UDP unicast to <candidate-ip>:49152):
  Payload (bytes): "AUTONOMOUS_LCD_PROBE?"

Receive (UDP unicast from device:49152):
  Payload (JSON):
  {
    "device_id": "lcd-a3f9c1",
    "http_port": 3000,
    "model": "esp32-st7789-1.14"
  }
```

### mDNS service advertisement

```
Service type: _autonomous-lcd._tcp
Port: 3000
TXT records:
  device_id=lcd-a3f9c1
  model=esp32-st7789-1.14
  version=1.0.0
```

### Device ID format

`lcd-<6 lowercase hex digits>` derived from the last 3 bytes of the device's WiFi MAC. Example: `lcd-a3f9c1`. The user receives this ID on a sticker affixed to the device — it's how they know which physical screen they own.

### Local config file

All three skills read and write `~/.config/autonomous-lcd.json`. Schema:

```json
{
  "devices": [
    {
      "device_id": "lcd-a3f9c1",
      "label": "Desk LCD",
      "token": "abc123def456...",
      "last_known_ip": "192.168.1.42",
      "last_seen_at": "2026-05-18T10:30:00Z"
    }
  ],
  "default_device_id": "lcd-a3f9c1"
}
```

`last_known_ip` is a cache — always validate it with a quick HEAD or `/status` GET before trusting it. If validation fails, fall through to discovery.

---

## 4. Skills to build

Each skill below has a name, description (for triggering), required behavior, and a sketch of what its SKILL.md and bundled scripts should contain. Follow the skill-creator conventions (`/mnt/skills/examples/skill-creator/SKILL.md`).

### Skill 1: `lcd-discover`

**Purpose**: find a paired device on the current LAN and update its cached IP.

**Description (for SKILL.md frontmatter)**:
> Discover a paired LCD device on the local network. Use whenever the user wants to find, locate, scan for, or refresh the network address of their LCD notification device, or whenever another LCD skill reports that a device's cached IP is stale. Triggers include phrases like "find my LCD", "rescan for the screen", "where is the device on the network", "my LCD stopped responding". This skill should also run automatically as a sub-step when notifications fail with a connection error.

**Behavior**:

1. Read `~/.config/autonomous-lcd.json`. If no devices configured, tell the user to run pairing first and stop.
2. Try the cached `last_known_ip` first — quick GET to `/status` with a 500 ms timeout. If the response's `device_id` matches, update `last_seen_at` and return the IP. Done.
3. If the cache miss, run **stage 1: mDNS service browse**. Listen for `_autonomous-lcd._tcp` services for ~2 seconds. For each found service, read the TXT records and match `device_id`. If found, update cache and return.
4. If mDNS finds nothing, run **stage 2: unicast UDP probe sweep**:
   - Read the local subnet via `netifaces` (preferred) or by parsing `ip route` / `ifconfig` output.
   - Build the candidate list (skip `.0`, `.255`, and the local machine's own IP).
   - Send the probe `b"AUTONOMOUS_LCD_PROBE?"` to each candidate on port 49152 in parallel using `asyncio` (no more than 50 concurrent sockets to avoid IDS triggers).
   - Listen for replies for ~2 seconds. Parse JSON, match by `device_id`.
   - If found, update cache and return.
5. If both stages fail, report clearly: which stage(s) ran, how many candidates were probed, and what the user should check (device powered on, on the same LAN, etc.).

**Bundled scripts**:

- `scripts/discover.py` — does the actual discovery, prints JSON to stdout.
- `scripts/lib/mdns_browse.py` — uses `zeroconf` library.
- `scripts/lib/udp_scan.py` — async UDP probe sweep.
- `scripts/lib/config.py` — reads/writes the config file.

**Important behaviors**:

- Always print structured output (JSON) on success so the calling skill can parse it.
- Log every step clearly so the user can debug network issues.
- Never scan a subnet larger than /22 without explicit user confirmation — protects against accidentally scanning a corporate /16.

---

### Skill 2: `lcd-pair`

**Purpose**: one-time setup that records a device's ID and shared secret in local config.

**Description (for SKILL.md frontmatter)**:
> Set up a new LCD notification device for the first time. Use whenever the user wants to pair, add, register, configure, or set up a new LCD device, or when they mention having a new screen they need to connect Claude to. Triggers include "pair my new LCD", "add a screen", "set up my notification display", "I have a new device, help me connect it". This skill should run before any notification can be sent to a brand-new device.

**Behavior**:

1. Prompt the user for the **device ID** (printed on the sticker, format `lcd-<6 hex>`).
2. Prompt for the **shared secret** (printed inside the device or shown on first boot — implementation depends on user's firmware; default expectation is a 32-character hex string the user reads off the LCD screen during a pairing mode).
3. Optionally prompt for a friendly label (e.g. "Desk LCD", "Kitchen LCD").
4. Run a discovery (delegate to `lcd-discover` logic or invoke the script directly) to verify the device is reachable.
5. Once verified, write the device to `~/.config/autonomous-lcd.json`. If this is the first device, set it as `default_device_id`.
6. Send a confirmation notification to the device: `"✓ Paired with Claude"`. This proves the full pipeline works end-to-end.
7. Report success or any failure clearly.

**Bundled scripts**:

- `scripts/pair.py` — interactive pairing flow.
- Reuses `scripts/lib/config.py` and `scripts/lib/http_send.py` from the other skills.

**Important behaviors**:

- Never log or print the shared secret after writing it to config.
- File permissions on the config: `0600` (user-only). Refuse to use a config with looser permissions, and offer to fix them.
- Treat re-pairing the same device ID as an update, not an error — overwrite the entry.

---

### Skill 3: `lcd-notify`

**Purpose**: the hot path — send a notification to a paired device.

**Description (for SKILL.md frontmatter)**:
> Send a text notification to the user's LCD notification device. Use whenever the user asks Claude to display, show, push, or send text to their LCD, screen, or notification device. Also use proactively at the end of long tasks — when a build, test run, deploy, code-generation, or other long-running operation completes, offer to ping the LCD so the user knows without checking the terminal. Triggers include "show this on my screen", "notify my LCD", "ping my display when done", "let me know on the device". Prefer this skill over plain text echoes whenever the user has a paired device and the message is genuinely a notification (status, completion, alert) rather than conversational output.

**Behavior**:

1. Read `~/.config/autonomous-lcd.json`. If no devices, tell the user to pair one first.
2. Determine target device: explicit `device_id` argument if provided, else `default_device_id`, else error.
3. Validate the message: max 500 characters; warn (don't fail) if longer than what fits comfortably (`size=2` shows ~165 characters).
4. Build the JSON payload from the message text plus optional `size`, `color`, `title` parameters.
5. POST to `http://<last_known_ip>/lcd` with `X-Token` and `X-Device-ID` headers. Timeout: 3 seconds.
6. On success (200), update `last_seen_at` and return.
7. On connection error or timeout, invoke `lcd-discover` logic to refresh the IP, then retry **once**. If retry succeeds, update the cache. If retry fails, report clearly with the underlying error.
8. On 401, prompt the user to re-pair — the secret is wrong.
9. On 403, report a device ID mismatch — the cached IP belongs to the wrong device, run discovery.

**Bundled scripts**:

- `scripts/notify.py` — main entry point, takes args for text, size, color, title, device-id.
- `scripts/lib/http_send.py` — POST helper with retry-after-discovery logic.
- Reuses `scripts/lib/config.py`.

**Important behaviors**:

- Keep the SKILL.md compact — this is the hot path and should load fast.
- Do not include explanatory prose about how the discovery fallback works in this SKILL.md; defer to `lcd-discover` for that. Just say "if the cached IP is stale, this skill will trigger rediscovery."
- Support a `--dry-run` flag for testing payload formatting without actually sending.
- When called with text containing newlines or formatting hints (markdown bold etc.), strip down to plain text the LCD can render.

---

## 5. Build sequence

Build the skills in this order. Each step is a checkpoint — verify it works before moving on.

### Step 1 — Skeleton and shared library

1. Create the three skill folders: `lcd-discover/`, `lcd-pair/`, `lcd-notify/`.
2. In each, create `SKILL.md` with just the frontmatter (name + description) and a one-line body. This establishes triggering before the implementation is ready.
3. Create a `_shared/` directory (or duplicate inside each — whichever fits the user's installation pattern better) with `config.py`, `http_send.py`, `mdns_browse.py`, `udp_scan.py`. Implement these as plain Python modules with clear function signatures and docstrings. Add Python type hints throughout.

### Step 2 — `lcd-discover` first

Implement discovery before notification, because notification depends on it. Test the script directly with `python scripts/discover.py --device-id lcd-XXXXXX` — make sure both stages work in isolation. Mock the network for unit tests; integration testing requires a real device.

### Step 3 — `lcd-pair`

Implement pairing. Test the full flow: device ID input → discovery → secret input → write config → confirmation notification. The end-to-end test for this is sending the "✓ Paired with Claude" message and seeing it on the physical screen.

### Step 4 — `lcd-notify`

Implement the notification skill last. By now the device is paired and discoverable; this skill is mostly a thin wrapper around `http_send.py` plus the retry-after-discovery logic.

### Step 5 — Tests

For each skill, create `evals/evals.json` with 3–5 realistic prompts (per the skill-creator skill's guidance). Do not write assertions yet; just prompts the user might say. Examples for `lcd-notify`:

- "Ping my LCD when the build finishes."
- "Show 'Hello from Claude' on my screen."
- "Notify my device that the deploy is complete."

### Step 6 — Packaging

If the `present_files` tool is available, package each skill (per skill-creator) and present the `.skill` files to the user for installation.

---

## 6. Code style and conventions

- **Python 3.10+** for all scripts. Use `asyncio`, `dataclasses`, modern type hints.
- **Dependencies**: keep minimal. Allowed: `zeroconf`, `netifaces`, `httpx`, `pydantic`. Pin versions in a per-skill `requirements.txt`.
- **Logging**: use the `logging` module, not `print`, except for the final JSON payload on stdout. Default level INFO, support `-v` for DEBUG.
- **Errors**: raise specific exceptions (`DeviceNotFoundError`, `AuthenticationError`, `DeviceIDMismatchError`) and catch them at the top level for clean user-facing messages.
- **No secrets in logs.** Token values, even partial, never appear in any log output.
- **File permissions**: config file is `0600`. Skills refuse to run with looser permissions.
- **Timeouts everywhere**: HTTP `3s` default, UDP probe `2s`, mDNS browse `2s`. Never block indefinitely.

---

## 7. What success looks like

- The user can plug in a brand-new device, scan a sticker, run "pair my LCD", and have it working in under a minute.
- "Ping my LCD when the build finishes" reliably reaches the screen, even after the device's DHCP lease has changed.
- All three skills work without internet access, without per-call server costs, without OAuth or token-refresh complexity.
- Failure modes are clear: the user always knows whether the problem is network, auth, device offline, or wrong device ID — not just "request failed."
- The skills compose: `lcd-notify` calls `lcd-discover` transparently when the cache is stale; `lcd-pair` calls `lcd-discover` to verify reachability during setup.

---

## 8. Out of scope

These are explicitly not part of this build:

- Firmware development. Assume the firmware contracts in section 3 are already implemented on the device.
- Cloud HTTP path through the production backend (separate spec, separate skills if ever needed).
- BLE. Off the table due to licensing.
- Multi-screen layouts, image rendering, animations. Text only for the first version.
- A GUI installer. Pairing is interactive via the chat session; no separate app.

---

When you're ready to start, confirm you've read this whole spec, then begin with Step 1 of section 5.
