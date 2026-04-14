# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.1] - 2026-04-14

### Changed
- LLM request tracking now generates `RequestID` with `ling-chatimpl-` prefix.
- LLM handlers (OpenAI) complete/error paths integrate request tracking so `RequestID`/`LatencyMs` are populated on `QueryResponse`.

### Added
- Signal-based LLM usage emission (`LLMUsage`) and lightweight signal payloads for session/message lifecycle.
- Example wiring in `usage_example_fixed.go` using `utils.Sig().Connect(...)` to persist chat/session/message/usage records without a service layer.

### Added
- Version management system
- Makefile for build automation
- Code quality checks with golangci-lint

## [0.1.0] - 2026-04-12

### Added
- Bootstrap module for database initialization
- Captcha module with image and click verification
- Censor module for content moderation (text, image, audio, video)
- Logger module with zap integration
- Media module for media processing
- Middleware module with security features
- Response module for HTTP responses
- Task module for task scheduling
- Token module for JWT authentication
- Utils module with various utility functions
