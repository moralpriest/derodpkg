# AGENTS.md

## Project Overview
- Go module (`github.com/moralpriest/derodpkg`)
- Library that wraps DERO daemon (`derod`) as an importable package
- Real API in `cmd/derodpkg.go`; `main.go` is a standalone example
- Built against Go 1.26 (as per go.mod)

## Installation
```bash
go install github.com/moralpriest/derodpkg@latest
```

## Key API (Struct-Based)
From `cmd/derodpkg.go`:
```go
type Daemon struct { ... }

func NewDaemon(initparams map[string]interface{}) (*Daemon, error)
func (d *Daemon) Initialize() error
func (d *Daemon) Start() error
func (d *Daemon) Stop() error
func (d *Daemon) Chain() *blockchain.Blockchain
func (d *Daemon) RPCServer() *derodrpc.RPCServer
```

## Usage Pattern
```go
params := map[string]interface{}{
    "--rpc-bind": "127.0.0.1:20202",
    "--testnet":  true,
}

d, err := derodpkg.NewDaemon(params)
if err != nil { log.Fatal(err) }
if err := d.Initialize(); err != nil { log.Fatal(err) }
if err := d.Start(); err != nil { log.Fatal(err) }
defer d.Stop()
```

## Build & Test
```bash
task build    # go build ./...
task test     # go test ./...
task vet      # go vet ./...
task lint     # golangci-lint run ./...
task tidy     # go mod tidy
task ci       # build + vet + test
```

## Coupling Notice
Strong dependency on `github.com/DEROFDN/derohe` (community-dev). Do not upgrade without careful validation.

## CI
GitHub Actions workflow in `.github/workflows/ci.yml` runs on push/PR to `dev` and `main`.

## Testing
- Unit tests in `cmd/derodpkg_test.go`
- No integration tests (daemon requires blockchain data)
