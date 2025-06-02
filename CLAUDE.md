# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Development Commands

### Build and Run
- `go build ./cmd/ims` - Build the binary
- `go run ./cmd/ims` - Run the application directly
- `go run ./cmd/ims --help` - View CLI options

### Testing and Quality
- `go test ./...` - Run all tests
- `golangci-lint run` - Run linting (used in CI)

### Release
- Uses GoReleaser for multi-platform builds and releases
- Docker images are built automatically for `wyattjoh/ims` and `ghcr.io/wyattjoh/ims`

## Architecture Overview

This is an image manipulation service (IMS) built in Go that provides on-the-fly image transformations via HTTP API.

### Core Components

**Main Entry Point**: `cmd/ims/main.go` - CLI application using urfave/cli/v2

**Image Processing Pipeline** (`internal/image/`):
- `image.go` - Main processing pipeline: Decode → Transform → Encode
- `transform/` - Resize, crop, format conversion, orientation, blur
- `encoder/` - JPEG, PNG, GIF output with quality controls

**Provider System** (`internal/image/provider/`):
- Multiple backend support: filesystem, Google Cloud Storage, S3/Minio, HTTP origins, proxy mode
- Each provider implements the same interface for fetching source images

**Platform Layer** (`internal/platform/`):
- `providers/` - Middleware for provider selection and caching
- `signing/` - HMAC-SHA256 request signing for security

### Key Dependencies
- `github.com/disintegration/imaging` - Core image processing
- `github.com/urfave/negroni` - HTTP middleware stack
- OpenTracing with Jaeger for distributed tracing
- Prometheus metrics

### Backend Configuration
Backends are configured via `--backend` flag with different schemes:
- `gs://bucket` - Google Cloud Storage (requires GOOGLE_APPLICATION_CREDENTIALS)
- `s3://bucket` - S3/Minio (requires S3_* environment variables)
- `https://origin.com` - HTTP/HTTPS origin server
- `/local/path` - Local filesystem
- `:proxy:` - Proxy mode with URL signing

### Security Model
Optional request signing using `--signing-secret` with HMAC-SHA256:
- Query parameters are sorted and signed
- `--signing-with-path` includes URL path in signature
- Required for proxy mode to prevent abuse

### API
Query parameter-based transformations following Fastly Image API:
- `width`, `height`, `fit` - Resizing
- `crop` - Cropping in format `{width},{height}`
- `format` - Output format (jpeg, png, gif) with quality control
- `orient` - Rotation and flipping
- `blur` - Gaussian blur with sigma value
- `resize-filter` - Algorithm selection (lanczos default, box, linear, etc.)