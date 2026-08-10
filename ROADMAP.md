# Roadmap

Tracks actual progress against the milestones defined in README.md's "Milestones" section. See README.md for the full project vision.

## v0.1 — Done

* [x] Project setup
* [x] Configuration
* [x] Seerr authentication
* [x] Search
* [x] Request media

## v0.2 — Done

Planned (from README):

* Better output
* Status commands
* Trending media
* Request history

Landed so far:

* [x] `reel status` — status commands / request history
* [x] `reel trending` — trending media
* [x] `reel browse movies` / `reel browse tv`, with `--page` pagination
* [x] `--json` across the read commands (search, trending, browse movies/tv, status) — better output
* [x] `reel request --select`/`--type` — fully non-interactive requesting
* [x] `reel delete` — cancel/remove a request via `DELETE /request/{requestId}`

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
