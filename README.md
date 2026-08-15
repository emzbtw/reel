# Project Vision

## Overview

This project is a terminal-first companion for Seerr, the self-hosted media request and discovery service that fronts Radarr, Sonarr, and Jellyfin.

Rather than replacing Seerr's own web UI, it provides a command line, a TUI, and Obsidian integration for the same discovery and requesting workflow.

The long-term goal is to create a tool that feels as natural to use as `lazygit`, `btop`, or `nom`—fast, keyboard-driven, scriptable, and enjoyable to use.

---

# Installation

## Via Nix flake

reel is packaged as a Nix flake.

Build and run directly:

    nix build
    ./result/bin/reel --help

Or install into your profile:

    nix profile install github:emzbtw/reel

## Configuration

reel needs a Seerr instance to talk to. Create a config file at
`~/.config/reel/config.toml` (respects `$XDG_CONFIG_HOME`):

    seerr_url = "https://your-seerr-instance"
    seerr_api_key = "your-seerr-api-key"

Find your API key in Seerr under Settings → General. Alternatively,
set `REEL_SEERR_URL` and `REEL_SEERR_API_KEY` as environment
variables — these override the config file.

For Obsidian sync, also set `obsidian_notes` to a list of note paths
in the same config file.

---

# Goals

* Provide a first-class CLI for media discovery and requests.
* Build a rich TUI for browsing and managing media.
* Integrate with Obsidian to turn notes into actionable media lists.
* Be easy to package and run on NixOS.
* Expose functionality through both human-friendly commands and automation-friendly interfaces.

---

# Philosophy

The browser should not be the only way to manage a media library.

Many self-hosters already spend much of their time in the terminal and in knowledge management tools such as Obsidian. This project aims to meet users where they already work instead of requiring them to open multiple web applications.

The TUI is the default way in: running `reel` with no arguments opens it.

The CLI should still be useful on its own, for scripting and one-off lookups.

---

# Backend

The application communicates with the Seerr API to:

* Search movies
* Search TV shows
* Submit requests
* View request status
* Check media availability
* Display trending and popular content

---

# Components

## CLI

The CLI provides quick access to common operations.

Examples:

```text
reel search dune
reel request interstellar
reel status
reel trending
reel sync
```

The CLI produces clean, readable output that works well in terminals and scripts.

---

## TUI

The TUI provides an interactive experience inspired by modern terminal applications.

Run `reel` with no arguments to launch it:

```text
reel
```

When output isn't a terminal (piped, redirected, or in CI), bare `reel` prints
the command help instead, so it stays safe to use in scripts.

Features:

* Browse Discover movies and TV shows, with pagination
* Search Seerr, with results replacing the browse list
* Keyboard-driven navigation
* Request media, with a confirmation step
* View and cancel existing requests

---

## Obsidian Integration

Obsidian becomes more than a note-taking application.

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
- [✗] Some Declined Movie
```

A line can also be marked `[x]`/`[X]` to pause reel from writing to it without losing tracking, or given a trailing `%%reel:ignore%%` comment to opt it out permanently.

The vault becomes a living media dashboard instead of a static wishlist.

---

# Long-Term Vision

The project should become a central hub for personal media management.

Possible future capabilities include:

* Unified search across multiple services
* Calendar of upcoming episodes
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
│   ├── config/
│   ├── models/
│   ├── obsidian/
│   └── tui/
├── flake.lock
├── flake.nix
├── go.mod
├── go.sum
├── LICENSE
├── Makefile
├── README.md
└── ROADMAP.md
```

---

# Development

Rebuild the `./reel` binary after pulling changes (`make build`) — a stale binary silently no-ops instead of reflecting new code.

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

## v1.0

A polished terminal-first media companion that unifies discovery, requesting, tracking, and personal media organization into a single, extensible tool.
