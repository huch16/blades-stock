## Build & Development

- **Build the desktop app**: `make build` (uses `wails build -clean -skipbindings`).
- **Run unit tests**: `make test` (excludes integration tests).
- **Run integration tests** (requires Chrome): `make test-integration`.
- **Lint**: `make lint` (runs `go vet`, `staticcheck`, and `golangci-lint`).
- **CI**: GitHub Actions workflow `.github/workflows/ci.yml` runs on pushes/pull‑requests, executing the above steps and publishing a coverage artifact.

Refer to this guide for local development workflow and CI integration.
