# Changelog

## [Unreleased]

### Added
- Support for Open Graph tags with caching and configurable options:
  - `OG_PASSTHROUGH`: A boolean to enable this whole system
  - `OG_EXPIRY_TIME`: A configurable cache expiration time
  - `OG_QUERY_DISTINCT`: A boolean (or regex pattern) to determine whether Anubis should treat the same page with different query parameters as distinct cache keys
