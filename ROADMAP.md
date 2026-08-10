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

## v0.3 — Done

Planned (from README):

* Obsidian synchronization
* Markdown parsing
* Metadata updates

Landed:

* [x] `reel sync` — syncs an Obsidian note's checklist against Seerr, with `--dry-run`, `--yes`, `--retry` and `--json`
* [x] Markdown parsing — task lines, wikilinks and markdown links, `(YYYY)` year hints, trailing tags and block IDs, `%%reel:ignore%%`; fenced code blocks are not treated as tasks
* [x] Metadata updates — status written back as `[ ]` / `[🎬]` / `[↓]` / `[✓]` / `[✗]`, replacing only the marker rune so the rest of the note is byte-for-byte untouched
* [x] Bindings persisted in-vault at `.reel/sync.toml`, deterministically ordered, so state travels with the vault across machines
* [x] Title matching weighted by vote count and popularity — auto-resolves a dominant match, reports genuine collisions (Dune 1984 vs 2021) instead of guessing
* [x] `api.MediaStatus` — library status for media with no request record
* [x] `obsidian_notes` config key, so bare `reel sync` works

Deliberately not included: `--interactive` disambiguation. Ambiguous lines
are resolved by adding a year to the line rather than by a picker.

## v0.4

* Interactive TUI

## v0.5

* Download progress
* Jellyfin integration
* Notifications

## v1.0

A polished terminal-first media companion that unifies discovery, requesting, tracking, and personal media organization into a single, extensible tool.
