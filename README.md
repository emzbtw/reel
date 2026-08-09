# Project Vision (Draft)

## Overview

This project is a terminal-first companion for modern self-hosted media servers.

Rather than replacing Seerr, Radarr, Sonarr, or Jellyfin, it provides a unified interface for interacting with them from the command line, a TUI, and personal workflows such as Obsidian.

The long-term goal is to create a tool that feels as natural to use as `lazygit`, `btop`, or `git`—fast, keyboard-driven, scriptable, and enjoyable to use.

---

# Goals

* Provide a first-class CLI for media discovery and requests.
* Build a rich TUI for browsing and managing media.
* Integrate with Obsidian to turn notes into actionable media lists.
* Remain backend-agnostic where possible.
* Be easy to package and run on NixOS.
* Expose functionality through both human-friendly commands and automation-friendly interfaces.

---

# Philosophy

The browser should not be the only way to manage a media library.

Many self-hosters already spend much of their time in the terminal and in knowledge management tools such as Obsidian. This project aims to meet users where they already work instead of requiring them to open multiple web applications.

The CLI should be useful on its own.

The TUI should enhance the CLI rather than replace it.

Every feature available in the TUI should be built on reusable library code.

---

# Initial Backend

The first backend will be Seerr.

Initially the application will communicate with the Seerr API to:

* Search movies
* Search TV shows
* Submit requests
* View request status
* Check media availability
* Display trending and popular content

Future versions may support additional services without changing the user experience.

Potential future backends include:

* Jellyfin
* Plex
* Trakt
* Radarr
* Sonarr

---

# Planned Components

## CLI

The CLI will provide quick access to common operations.

Examples:

```text
reel search dune
reel request interstellar
reel status
reel trending
reel sync
```

The CLI should produce clean, readable output that works well in terminals and scripts.

---

## TUI

The TUI will provide an interactive experience inspired by modern terminal applications.

Possible features:

* Search as you type
* Browse trending media
* View artwork and metadata
* Keyboard-driven navigation
* Request media
* Monitor download progress
* View request history
* Open media in Jellyfin once available

---

## Obsidian Integration

Obsidian will become more than a note-taking application.

Example workflow:

A user writes:

```markdown
# Movies

- [ ] Arrival
- [ ] Heat
- [ ] Alien
```

Running a sync command will:

* Detect movie titles
* Search Seerr
* Submit requests
* Update note metadata
* Track request status

Example result:

```markdown
- [✓] Arrival
- [↓] Heat
- [🎬] Alien
```

The vault becomes a living media dashboard instead of a static wishlist.

---

# Long-Term Vision

The project should become a central hub for personal media management.

Possible future capabilities include:

* Unified search across multiple services
* Download monitoring
* Calendar of upcoming episodes
* Notifications when requests complete
* Watch history
* Trakt synchronization
* Import/export watchlists
* Home Assistant integration
* Discord and Telegram integrations
* Plugin architecture

---

# Technical Direction

Language:

* Go

Primary Libraries:

* cobra (CLI)
* net/http (HTTP)
* encoding/json (JSON)
* bubbletea (TUI)
* bubbles (TUI components)
* lipgloss (terminal styling)

Development Principles:

* Library-first architecture
* Small, composable modules
* Strong typing
* Good documentation
* Automated tests
* Frequent, reviewable commits

---

# Project Structure (Initial)

```text
reel/
├── cmd/
│   └── reel/
│       └── main.go
├── internal/
│   ├── api/
│   ├── cli/
│   ├── commands/
│   ├── config/
│   ├── models/
│   ├── obsidian/
│   └── tui/
├── docs/
├── go.mod
├── go.sum
├── README.md
├── ROADMAP.md
└── flake.nix
```

---

# Milestones

## v0.1

* Project setup
* Configuration
* Seerr authentication
* Search
* Request media

## v0.2

* Better output
* Status commands
* Trending media
* Request history

## v0.3

* Obsidian synchronization
* Markdown parsing
* Metadata updates

## v0.4

* Interactive TUI

## v0.5

* Download progress
* Jellyfin integration
* Notifications

## v1.0

A polished terminal-first media companion that unifies discovery, requesting, tracking, and personal media organization into a single, extensible tool.
