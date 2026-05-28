# Vibe Desk Display — Setup Guide

Turn your VibeDesk into a live Claude Code usage monitor.

---

## What you need

- A VibeDesk device, set up via the mobile app
- Your computer and VibeDesk on the same WiFi network
- Claude Code (with OAuth login, not API key)

---

## Install

Open your terminal and run:

```bash
claude plugins marketplace add https://github.com/autonomous-ai/vibe-desk-display
```

```bash
claude plugins install vibe-desk-display
```

Then **restart Claude Code** (exit and reopen).

---

## Pair your device

1. Make sure your VibeDesk is set up and connected to the same WiFi as your computer
2. Open Claude Code and type:

```
pair my display
```

3. Claude will scan your network and find the device
4. A **4-digit code** will appear on the display screen
5. Type that code into Claude Code

That's it — your device is paired.

---

## What happens next

Once paired, the display automatically shows your Claude Code usage every time Claude responds. No action needed.

You can also:

- Say `notify my display` to send a custom message to the screen
- Type `/vibe-desk-display:usage` to refresh the usage display immediately
- Say `unpair my display` to disconnect the device

---

## Update the plugin

```bash
claude plugins update vibe-desk-display
```

Restart Claude Code after updating.

---

## Uninstall

```bash
claude plugins uninstall vibe-desk-display
```
