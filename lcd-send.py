import json, urllib.request, sys

with open("/Users/huynguyen/.config/autonomous-lcd.json") as f:
    cfg = json.load(f)

device = cfg["devices"][0]
ip = device["last_known_ip"]
device_id = device["device_id"]
token = device.get("token", "")

raw = sys.argv[1] if len(sys.argv) > 1 else sys.stdin.read()
payload = raw.encode() if isinstance(raw, str) else json.dumps(raw).encode()

req = urllib.request.Request(
    f"http://{ip}:3000/lcd",
    data=payload,
    headers={"Content-Type": "application/json", "X-Device-ID": device_id, "X-Token": token},
    method="POST",
)

try:
    with urllib.request.urlopen(req, timeout=3) as resp:
        print(f"HTTP {resp.status}")
        print(resp.read().decode())
except Exception as e:
    print(f"Error: {e}")
