#!/usr/bin/env python3
"""Hook: send usage to LCD when Claude stops. Rate-limited to once per 60s."""

import json
import os
import sys
import time

CONFIG_PATH = os.path.expanduser("~/.config/autonomous-lcd.json")
COOLDOWN_PATH = os.path.expanduser("~/.config/autonomous-lcd-hook.last")
COOLDOWN_SECONDS = 60


def should_run():
    try:
        last = float(open(COOLDOWN_PATH).read().strip())
        return (time.time() - last) >= COOLDOWN_SECONDS
    except Exception:
        return True


def mark_ran():
    with open(COOLDOWN_PATH, "w") as f:
        f.write(str(time.time()))


def main():
    json.load(sys.stdin)

    if not should_run():
        sys.exit(0)

    if not os.path.exists(CONFIG_PATH):
        sys.exit(0)

    plugin_root = os.environ.get("CLAUDE_PLUGIN_ROOT", os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
    daemon_script = os.path.join(plugin_root, "scripts", "lcd-usage-daemon.py")

    if not os.path.exists(daemon_script):
        sys.exit(0)

    mark_ran()

    import subprocess
    subprocess.Popen(
        [sys.executable, daemon_script],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )


if __name__ == "__main__":
    main()
