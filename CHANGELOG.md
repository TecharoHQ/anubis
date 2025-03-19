# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

- Docker images are now built with the timestamp set to the commit timestamp
- The README now points to TecharoHQ/anubis instead of Xe/x
- Images are built using ko instead of `docker buildx build`
  [#13](https://github.com/TecharoHQ/anubis/pull/13)

## 1.12.1

- Phrasing in the `<noscript>` warning was replaced from its original placeholder text to
  something more suitable for general consumption
  ([fd6903a](https://github.com/TecharoHQ/anubis/commit/fd6903aeed315b8fddee32890d7458a9271e4798)).
- Footer links on the check page now point to Techaro's brand
  ([4ebccb1](https://github.com/TecharoHQ/anubis/commit/4ebccb197ec20d024328d7f92cad39bbbe4d6359))
- Anubis was imported from [Xe/x](https://github.com/Xe/x).
