# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.14.0] - 2026-11-21

Changes since the last release, v1.13.0.

### Added

- Official ArchiveBox REST API support using API-key (Bearer token) authentication: connection checks via `POST /api/v1/auth/check_api_token` and URL submission via `POST /api/v1/cli/add`.
- API Key field in settings, with a hint pointing to the ArchiveBox admin UI (`/admin/api/apitoken/`).
- Unit tests for the API client (`archivebox_test.go`).
- macOS (Darwin) and FreeBSD build/release targets in CI, plus a dedicated release workflow.
- Dependabot configuration for automated dependency updates.

### Changed

- Replaced web-login-form authentication and session/CSRF cookie handling with API keys. Existing users must create an API key in ArchiveBox and enter it in the app settings.
- Settings now take effect immediately without restarting the app.
- Instance URL validation accepts any `http(s)` URL; a port number is no longer required.
- Increased the URL-add request timeout to 60 seconds, since server-side archiving can take a while.
- Updated English and German UI strings and the README for API-key authentication.
- Updated Go (1.25 → 1.26), Fyne (2.8.1 → 2.8.2) and other dependencies.

### Fixed

- Fyne UI thread handling for background connection and archiving operations.
- `make release` command.

### Removed

- Borderless window preference.
- Username and password settings and the associated session-cookie/CSRF login logic.
