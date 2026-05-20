---
name: lcd-pair
description: >
  Set up a new LCD notification device for the first time. Use whenever the user
  wants to pair, add, register, configure, or set up a new LCD device, or when they
  mention having a new screen they need to connect Claude to. Triggers: "pair my new LCD",
  "add a screen", "set up my notification display", "I have a new device, help me connect it".
---

# LCD Device Pairing

One-time setup that records a device's ID and shared secret in local config.

## Config file

Path: `~/.config/autonomous-lcd.json`

```json
{
  "devices": [
    {
      "device_id": "lcd-a3f9c1",
      "label": "Desk LCD",
      "token": "<secret>",
      "last_known_ip": "192.168.1.42",
      "last_seen_at": "2026-05-18T10:30:00Z"
    }
  ],
  "default_device_id": "lcd-a3f9c1"
}
```

## Utilities

Use `python3` for all JSON config reads and writes (`jq` is not available by default on
macOS).

## Procedure

### 1. Collect device info

Ask the user for:

1. **Device ID** (required) — printed on a sticker on the device. Format: `lcd-<6 lowercase hex digits>` (e.g. `lcd-a3f9c1`). Validate the format before proceeding.
2. **Shared secret** (required) — a 32-character hex string shown on the LCD screen during pairing mode. **Never echo this back, never log it, never print it after receiving it.**
3. **Label** (optional) — friendly name like "Desk LCD" or "Kitchen LCD". Default to "My LCD" if not provided.

### 2. Discover the device

Find the device on the network to get its IP. Read `lcd-discover/SKILL.md` and follow
its full procedure:

1. Try cached IP from config (if device was previously paired).
2. mDNS browse for `_autonomous-lcd._tcp`.
3. UDP probe sweep on port 49152.

If the device cannot be found, report the failure and suggest the user check that the
device is powered on and connected to the same WiFi network. Stop here.

### 3. Verify the device

Once an IP is found, confirm it's the right device:

```bash
curl -s --max-time 3 http://<ip>:3000/status
```

Check that `device_id` in the response matches what the user provided. If not, the IP
belongs to a different device — continue discovery or report the mismatch.

### 4. Write config

Ensure the config directory exists:

```bash
mkdir -p ~/.config
```

Read `~/.config/autonomous-lcd.json` (create it if it doesn't exist).

- If the device ID already exists in the config, update its entry (re-pairing).
- If it's new, append to the `devices` array.
- If this is the first device, set `default_device_id` to this device's ID.
- Set `last_known_ip` to the discovered IP.
- Set `last_seen_at` to now (ISO 8601).

Write the file with permission `0600`:

```bash
chmod 600 ~/.config/autonomous-lcd.json
```

### 5. Send confirmation

Send a test notification to prove the full pipeline works:

```bash
curl -s --max-time 3 -X POST http://<ip>:3000/lcd \
  -H "Content-Type: application/json" \
  -H "X-Token: <secret>" \
  -H "X-Device-ID: <device_id>" \
  -d '{"text": "Paired with Claude", "color": "green", "title": "Setup"}'
```

- **200** — Pairing complete. Report success to the user.
- **401** — Secret is wrong. Ask the user to double-check it.
- **403** — Device ID mismatch. The IP belongs to a different device.
- **Other error** — Report the status code and suggest the user check the device.

### 6. Report

On success, tell the user:
- Device ID and label
- IP address
- That they can now use "notify my LCD" or "show X on my screen"

## Important

- **Never print, echo, or log the shared secret** after receiving it from the user.
- Config file must be permission `0600`. If it exists with looser permissions, fix them with `chmod 600` and warn the user.
- Re-pairing the same device ID is an update, not an error.
