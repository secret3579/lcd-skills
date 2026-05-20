---
name: lcd-discover
description: >
  Discover a paired LCD device on the local network. Use whenever the user wants
  to find, locate, scan for, or refresh the network address of their LCD notification
  device, or whenever another LCD skill reports that a device's cached IP is stale.
  Triggers: "find my LCD", "rescan for the screen", "where is the device on the network",
  "my LCD stopped responding".
---

# LCD Device Discovery

Find a paired LCD device on the current LAN and update its cached IP in
`~/.config/autonomous-lcd.json`.

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
macOS). Example read:

```bash
python3 -c "
import json, pathlib
cfg = json.loads(pathlib.Path('$HOME/.config/autonomous-lcd.json').read_text())
print(json.dumps(cfg, indent=2))
"
```

Example update (set a field on a device entry):

```bash
python3 -c "
import json, pathlib, datetime
p = pathlib.Path('$HOME/.config/autonomous-lcd.json')
cfg = json.loads(p.read_text())
for d in cfg['devices']:
    if d['device_id'] == '<device_id>':
        d['last_known_ip'] = '<new_ip>'
        d['last_seen_at'] = datetime.datetime.now(datetime.timezone.utc).isoformat()
p.write_text(json.dumps(cfg, indent=2))
"
```

## Procedure

### 0. Pre-check

Read `~/.config/autonomous-lcd.json`. If the file does not exist or `devices` is empty, tell
the user to run **lcd-pair** first and stop.

Determine the target `device_id`: use the one the user provided, or fall back to
`default_device_id`.

### 1. Cache check (fast path)

Try the cached `last_known_ip` with a 500 ms timeout:

```bash
curl -s --max-time 0.5 http://<last_known_ip>:3000/status
```

Parse the JSON response. If `device_id` in the response matches the target, the device
is still at this IP. Update `last_seen_at` to now in the config and report success.
Done.

### 2. mDNS service browse

If cache check fails, use the macOS built-in `dns-sd`:

```bash
timeout 3 dns-sd -B _autonomous-lcd._tcp 2>&1
```

This lists services advertising `_autonomous-lcd._tcp`. For each service name found, resolve it:

```bash
timeout 3 dns-sd -L "<service-name>" _autonomous-lcd._tcp 2>&1
```

Look for TXT record containing `device_id=<target>`. Extract the **hostname** from the
"can be reached at" line (e.g. `lcd-a3f9c1.local.`).

Then resolve the hostname to an IP address:

```bash
timeout 3 dns-sd -G v4 <hostname> 2>&1
```

Parse the IPv4 address from the output line containing `Add` (last field on that line).

If found, update `last_known_ip` and `last_seen_at` in config. Report success.

**Note:** `dns-sd` on macOS runs interactively and does not exit on its own. Always
wrap with `timeout`. Parse the output line by line.

### 3. UDP probe sweep (fallback)

If mDNS finds nothing (multicast may be blocked), do a unicast UDP scan.

**Get the local subnet:**

```bash
ifconfig | grep "inet " | grep -v 127.0.0.1
```

Pick the active interface (typically the one with a 192.168.x.x or 10.x.x.x address).
Derive the /24 subnet from it.

**Safety check:** If the subnet is larger than /22 (more than 1022 hosts), ask the user
for confirmation before scanning.

**Probe all candidate IPs** on UDP port 49152 with payload `AUTONOMOUS_LCD_PROBE?`. Use `python3`
for reliable UDP (macOS `nc -u` may not capture replies correctly):

```bash
python3 << 'PYEOF'
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

print(json.dumps({"error": "not_found", "probed": len(candidates)}), file=sys.stderr)
sys.exit(1)
PYEOF
```

Each reply is JSON:

```json
{"device_id": "lcd-a3f9c1", "http_port": 3000, "model": "esp32-st7789-1.14"}
```

Match by `device_id`. If found, update config and report success.

### 4. Failure

If all stages fail, report clearly:
- Which stages ran (cache / mDNS / UDP)
- How many IPs were probed in the UDP sweep
- Checklist for the user: Is the device powered on? Is it on the same WiFi network?
  Is the device's HTTP server enabled?

## Output

On success, report:
- Device ID
- IP address found
- Which stage found it (cache / mDNS / UDP)

Always update the config file with the new IP and timestamp on success.

## Important

- Never print or log the device token.
- Never scan a subnet larger than /22 without explicit user confirmation.
