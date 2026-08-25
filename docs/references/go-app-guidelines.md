# Go Application Guidelines

Conventions for non-library Go projects: binaries, CLIs, servers, daemons.
Apply when the project's purpose is a `main` package. These summarise
community consensus (official go.dev layout doc,
golang-standards/project-layout, and widely followed practice). They are
conventions, not rules; small projects need none of this beyond a flat root.

## Scope

1. An application has no exported API for other modules.
2. Code that another module should import belongs in a separate library
   module, not in `pkg/` inside the application.

## Layout by Size

1. Single small binary: flat layout, `package main` files at the module
   root next to `go.mod`.
2. Small binary with supporting packages: root `main` files plus
   supporting packages under `internal/`.
3. Larger project, server, or more than one binary: the server layout,
   `cmd/<binary>/` per executable plus `internal/` for all logic, with
   non-Go assets (configs, scripts, Dockerfile, CI) at the root.

```text
myapp/
  cmd/
    myapp/
      main.go
  internal/
    user/
      user.go
      user_test.go
  configs/
  scripts/
  go.mod
  README.md
```

### Optional Directories (add only when needed)

| Directory   | Purpose                                                    |
|-------------|------------------------------------------------------------|
| `api/`      | API specs: OpenAPI, protobuf, JSON schema                  |
| `configs/`  | Default config files or templates                          |
| `scripts/`  | Build, install, and analysis scripts                       |
| `build/`    | Packaging (Docker, deb, rpm) and CI configuration          |
| `deployments/` | Deployment manifests (compose, k8s, helm, terraform)    |
| `test/`     | External integration tests and test data                   |
| `docs/`     | Design and user documents                                  |
| `examples/` | Example programs                                           |
| `tools/`    | Project-supporting tools                                   |
| `assets/`   | Images, logos, and other repository assets                 |

## Main Package Rules

1. `cmd/` subdirectory name equals the executable name.
2. `main.go` stays thin: flag parsing, dependency wiring, and a call to
   `run()`. Target ~15 lines, ceiling 50; beyond that, logic has leaked
   in and belongs in `internal/`.
3. Use the `run() error` pattern; `os.Exit` appears only in `main`.

```go
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

## Boundaries and Packages

1. `internal/` is the default home for logic: the `go` toolchain blocks
   imports of `internal/` packages from other modules, so it is the only
   boundary that is enforced.
2. `pkg/` is an optional signal, not a mechanism: it is not enforced by
   the toolchain, is contested in the community, and is omitted by many
   major projects. Use it only for code deliberately shared across repos.
3. Group `internal/` packages by domain, with at most one edge layer for
   transport (HTTP, gRPC).

## Installation

1. Install from the module root (flat layout):
   `go install <module>@latest`.
2. Install from a `cmd/` layout:
   `go install <module>/cmd/<binary>@latest`.
3. Multiple binaries: one `cmd/<name>/` directory each, sharing
   `internal/`. Multiple `go.mod` files only for independent release
   cycles, never for tidiness.

## Shared Conventions

Module basics, naming, package documentation, the Makefile standard,
documentation files, `.gitignore`, and the baseline (commands, linting,
testing) are defined in `go-common-guidelines.md`.

## Sources

1. Organizing a Go module (official): https://go.dev/doc/modules/layout
2. Standard Go Project Layout (community):
   https://github.com/golang-standards/project-layout

Contested areas, stated as splits rather than rules: `pkg/` usage,
`vendor/` committing, and Makefile presence all vary across well-regarded
projects; follow whichever choice the project already makes.
