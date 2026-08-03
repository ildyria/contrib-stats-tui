# How to build

Build the `contributors` binary from source.

## Prerequisites

- **Go 1.26+** installed and on your `PATH`.
- A **`git`** binary on your `PATH` (used both to build version info and at runtime).

## Build into `./bin`

```sh
make build
```

This drops a binary at `./bin/contributors`. Under the hood it runs:

```sh
go build -ldflags "-X <module>/internal/gitstats.Version=<version>" -o bin/contributors .
```

The version string is derived from `git describe --tags --always --dirty`, so
the compiled binary reports a meaningful `Version`. The same value also keys the
on-disk cache, so a rebuild transparently invalidates any stale cache.

## Install into `$GOBIN`

To install the binary onto your `PATH` instead of `./bin`:

```sh
make install
```

This runs `go install` with the same version ldflags, placing the binary in
`$GOBIN` (or `$GOPATH/bin`).

## Other useful targets

| Target        | What it does                                             |
| ------------- | ------------------------------------------------------- |
| `make fmt`    | Format the code (`gofmt -w -s .`).                       |
| `make vet`    | Run `go vet ./...`.                                      |
| `make test`   | Run tests (set `TEST_REPO=/path/to/repo` for integration tests). |
| `make tidy`   | Tidy module dependencies (`go mod tidy`).               |
| `make clean`  | Remove build artifacts (`./bin`).                       |
| `make help`   | List all available targets.                             |

## Build without make

If you prefer not to use `make`:

```sh
go build -o bin/contributors .
```

The version ldflags are optional; without them the binary reports `Version=dev`.
