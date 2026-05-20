#!/bin/bash
set -e

DEVICE_ID="lcd-a3f9c1"
HTTP_PORT="3000"
UDP_PORT="49152"
CONFIG="$HOME/.config/autonomous-lcd.json"
MOCK_PID=""

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

pass() { echo -e "  ${GREEN}PASS${NC} $1"; }
fail() { echo -e "  ${RED}FAIL${NC} $1"; exit 1; }
step() { echo -e "\n${YELLOW}[$1]${NC} $2"; }

cleanup() {
    [ -n "$MOCK_PID" ] && kill "$MOCK_PID" 2>/dev/null
    rm -f "$CONFIG" "$MOCK_LOG"
    echo -e "\n${YELLOW}[cleanup]${NC} Mock device stopped, test config removed"
}
trap cleanup EXIT

# ─── Start mock device ───────────────────────────────────────────────
step "0/4" "Starting mock device..."
cd "$(dirname "$0")"

if [ ! -f ./lcd-mock ]; then
    echo "  Building lcd-mock..."
    go build -o lcd-mock .
fi

MOCK_LOG=$(mktemp)
DEVICE_ID="$DEVICE_ID" HTTP_PORT="$HTTP_PORT" UDP_PORT="$UDP_PORT" \
    ./lcd-mock > "$MOCK_LOG" 2>&1 &
MOCK_PID=$!
sleep 1

if ! kill -0 "$MOCK_PID" 2>/dev/null; then
    fail "Mock device failed to start (port in use?)"
fi
pass "Mock device running (PID $MOCK_PID)"

# ─── Step 1: Pair ────────────────────────────────────────────────────
step "1/4" "Pair — discover device + create config"

mkdir -p ~/.config

# Verify device is reachable
STATUS=$(curl -s --max-time 1 http://localhost:$HTTP_PORT/status)
FOUND_ID=$(echo "$STATUS" | python3 -c "import sys,json; print(json.load(sys.stdin)['device_id'])")

if [ "$FOUND_ID" != "$DEVICE_ID" ]; then
    fail "GET /status returned device_id=$FOUND_ID, expected $DEVICE_ID"
fi
pass "Device reachable, device_id matches"

# Write config
python3 -c "
import json, datetime, pathlib, os

cfg = {
    'devices': [{
        'device_id': '$DEVICE_ID',
        'label': 'Test LCD',
        'last_known_ip': '127.0.0.1',
        'last_seen_at': datetime.datetime.now(datetime.timezone.utc).isoformat()
    }],
    'default_device_id': '$DEVICE_ID'
}

p = pathlib.Path('$CONFIG')
p.write_text(json.dumps(cfg, indent=2))
os.chmod(p, 0o600)
"
pass "Config written to $CONFIG"

# Send confirmation notification
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-time 3 \
    -X POST http://127.0.0.1:$HTTP_PORT/lcd \
    -H "Content-Type: application/json" \
    -d '{"text": "Paired with Claude", "color": "green", "title": "Setup"}')

if [ "$HTTP_CODE" = "200" ]; then
    pass "Confirmation notification sent (HTTP $HTTP_CODE)"
else
    fail "Confirmation notification failed (HTTP $HTTP_CODE)"
fi

# ─── Step 2: Discover — cache + UDP ─────────────────────────────────
step "2/4" "Discover — cache check + UDP probe"

CACHED_IP=$(python3 -c "
import json, pathlib
cfg = json.loads(pathlib.Path('$CONFIG').read_text())
for d in cfg['devices']:
    if d['device_id'] == '$DEVICE_ID':
        print(d['last_known_ip'])
        break
")

STATUS=$(curl -s --max-time 0.5 http://$CACHED_IP:$HTTP_PORT/status 2>/dev/null || echo '{}')
FOUND_ID=$(echo "$STATUS" | python3 -c "import sys,json; print(json.load(sys.stdin).get('device_id',''))" 2>/dev/null)

if [ "$FOUND_ID" = "$DEVICE_ID" ]; then
    pass "Cache hit — device at $CACHED_IP confirmed"
else
    fail "Cache check failed"
fi

UDP_REPLY=$(python3 -c "
import socket, json
s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
s.settimeout(2)
s.sendto(b'AUTONOMOUS_LCD_PROBE?', ('127.0.0.1', $UDP_PORT))
data, _ = s.recvfrom(1024)
print(data.decode())
s.close()
" 2>/dev/null)

FOUND_ID=$(echo "$UDP_REPLY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('device_id',''))")

if [ "$FOUND_ID" = "$DEVICE_ID" ]; then
    pass "UDP probe reply: $UDP_REPLY"
else
    fail "UDP probe returned wrong device_id"
fi

# ─── Step 3: Notify ──────────────────────────────────────────────────
step "3/4" "Notify — send notification"

RESPONSE=$(curl -s -w "\n%{http_code}" --max-time 3 \
    -X POST http://127.0.0.1:$HTTP_PORT/lcd \
    -H "Content-Type: application/json" \
    -d '{"text": "Build #42 passed. All tests green.", "color": "green", "title": "CI"}')

HTTP_CODE=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    pass "Notification delivered (HTTP $HTTP_CODE)"
else
    fail "Notification failed (HTTP $HTTP_CODE): $BODY"
fi

# ─── Step 4: Notify — error case ─────────────────────────────────────
step "4/4" "Notify — error handling"

# 400: missing text
CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-time 3 \
    -X POST http://127.0.0.1:$HTTP_PORT/lcd \
    -H "Content-Type: application/json" \
    -d '{"color": "red"}')
[ "$CODE" = "400" ] && pass "400 on missing text" || fail "Expected 400, got $CODE"

# ─── Summary ─────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}All tests passed.${NC} Full flow: pair → discover (cache + UDP) → notify → error handling"

echo ""
echo -e "${YELLOW}[LCD Output]${NC} Notifications rendered on mock device:"
grep -E "╔|║|╠|╚|size=" "$MOCK_LOG" || true
