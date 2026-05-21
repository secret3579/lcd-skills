# LCD Plugin — Setup Guide

Turn your ESP32 LCD into a live Claude Code usage monitor.

---

## What you need

- An ESP32 ST7789 LCD device
- Your computer and the device on the same WiFi network
- Claude Code (with OAuth login, not API key)

---

## Install

Open your terminal and run:

```bash
claude plugins marketplace add https://github.com/secret3579/lcd-skills
```

```bash
claude plugins install lcd
```

Then **restart Claude Code** (exit and reopen).

---

## Pair your device

1. Power on your LCD device
2. Open Claude Code and type:

```
pair my LCD
```

3. Claude will scan your network and find the device
4. A **4-digit code** will appear on the LCD screen
5. Type that code into Claude Code

That's it — your device is paired.

---

## What happens next

Once paired, the LCD automatically shows your Claude Code usage every time Claude responds. No action needed.

You can also:

- Say `notify my LCD` to send a custom message to the screen
- Type `/lcd:usage` to refresh the usage display immediately
- Say `unpair my LCD` to disconnect the device

---

## Update the plugin

```bash
claude plugins update lcd
```

Restart Claude Code after updating.

---

## Uninstall

```bash
claude plugins uninstall lcd
```
