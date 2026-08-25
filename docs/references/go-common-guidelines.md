# Go General Guidelines

Conventions that apply to every Go project in this workspace, library or
application. Load this document for every Go project, then load
`go-app-guidelines.md` additionally for applications.

## Module Basics

1. First path element of the module contains a dot, e.g.
   `gitlab.com/lyoneel/myapp` or `github.com/user/myapp`.
2. Package name equals the last element of the module path; mismatched
   names are legal but discouraged.
3. `go.mod` declares the lowest Go version actually required.
4. `go.sum` exists only when the module has external dependencies; a
   zero-dependency module ships without one.
5. Prefer the standard library over external dependencies. When only a
   small part of a library is needed, implement that functionality
   manually instead of bringing the whole library for a single function.
   External libraries are allowed but discouraged. For example, since
   Go 1.22, `net/http.ServeMux` matches method and path wildcards
   (`GET /items/{id}`), covering typical HTTP routing without a
   third-party router.
6. No `src/` directory: a Java pattern, never idiomatic in Go.

## Naming

1. Package names: lowercase, no underscores, short singular nouns.
2. Never name packages `util`, `common`, or `helpers`.

## Package Documentation

1. Every package has a `// Package <name> ...` comment on exactly one
   file (doc.go or the primary source file); pkg.go.dev uses it as the
   overview for published packages.
2. Exported identifiers carry doc comments starting with the identifier
   name.

## Makefile (required)

1. Every Go module ships a `Makefile` at the module root.
2. Required targets: `help`, `build`, `test`, `test-race`, `fmt`, `vet`,
   `lint`, `audit`, `clean`.
3. Application projects add `run` (required) and `dev` (optional, air
   auto-reload); both templates also provide `vuln`, `bench`, and
   `generate`, and the application template adds `deadcode` (requires a
   main package).
4. `help` is the default target (listed first, so naked `make` prints it)
   and is self-documenting: every target carries a `## Description`
   comment; `help` prints them with the standard grep/awk snippet in the
   template files.
5. Declare all targets in one `.PHONY:` line.
6. Set `SHELL := /bin/bash`.
7. Define project settings at the top (`NAME`, `MAIN_PKG`, `BINARY`,
   `PORT`) and tool variables (`GO`, `GOFMT`, `GOLINT`). `MAIN_PKG` is
   `./cmd/<name>` in a `cmd/` layout and `.` in a flat layout.
8. Binaries build to `build/$(NAME)`, never to the repository root;
   `build/` is gitignored and `clean` removes it.
9. Group targets with `# --- Section ---` comments in this order: Build,
   Run, Quality, Codegen, Clean. Omit empty sections.
10. Print a short echo message for non-trivial actions.
11. Guard optional tools with `command -v <tool>` and print the install
    command when missing.
12. For library modules, `build` is a compilation check
    (`go build ./...`); there is no binary and no `run` target.
13. Code-generation targets (sqlc, templ, wire) live under
    `# --- Codegen ---` and carry tool guards.
14. Keep recipes thin; move long logic into `scripts/`.
15. Do not export `GOPRIVATE` or `GONOSUMDB` for public module hosts
    such as `gitlab.com/lyoneel/*`; the default proxy and sumdb fetch and
    verify public modules. Export them only for genuinely private module
    hosts.
16. AGENTS.md documents the project's make targets.

### Optional Target Patterns

1. Cross-compilation and releases: a `release` target with `GOOS` and
   `GOARCH` set inline per command (they do not leak into the shell),
   and version injection via
   `go build -ldflags "-X main.version=$(VERSION)"` with
   `VERSION ?= dev` so callers override it: `make release VERSION=v1.2.0`.
2. Operations targets (`push`, `deploy`): gate them on helper targets,
   run quality checks first.

```makefile
confirm: ## Ask for confirmation before risky operations
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $$ans = y ]

no-dirty: ## Fail when the working tree has uncommitted changes
	@test -z "$$(git status --porcelain)"

push: confirm no-dirty test vet ## Push changes to the remote
	@git push
```

3. Vulnerability checks: `vuln` (govulncheck) needs network access to
   the vulnerability database, so it stays a standalone target. `audit`
   stays fully offline except for a 3s probe of `proxy.golang.org:443`;
   when online it runs `vuln`, otherwise it skips it with a note.
4. Very large Makefiles: split by concern into `make/*.mk` files and
   `include` them from the root Makefile.

## Documentation Files

1. `README.md` and `CHANGELOG.md` carry the workspace YAML frontmatter:
   `version`, `last-updated`, `title`, `description`.
2. `CHANGELOG.md` uses the date-build scheme:
   - `vMAJOR.MINOR.YYYYMMDD-N`: prerelease; N is the build number within
     the calendar day.
   - `vMAJOR.MINOR.YYYYMMDD`: stable release; no suffix, supersedes the
     prereleases of that date.
3. `README.md` is required in every project; type-specific content
   (module block and usage examples for libraries, build and run
   instructions for applications) is defined in the type-specific
   document.

## .gitignore

The template ships a standard `.gitignore` at its root: Go patterns
(from github/gitignore `Go.gitignore`), build output (`/build/`,
`/bin/`), and OS patterns for Windows, macOS, and Linux. Copy it into
new projects and extend per project.

## Baseline (all projects)

### Commands

1. `go build ./...`, `go vet ./...`, `go test ./...`, and `gofmt -l .`
   must pass before any commit; the Makefile targets wrap these.

### Linting

1. `gofmt` and `go vet` are the baseline.
2. golangci-lint is the standard meta-linter, configured in
   `.golangci.yml`; its default set covers `errcheck`, `govet`,
   `staticcheck`, `ineffassign`, and `unused`.

### Testing

1. Unit tests sit beside the code as `<source>_test.go`, table-driven.
2. Use an external test package (`package x_test`) to test only the
   public API and catch accidental exposure of internals.
3. Fixtures live under `testdata/`.
4. Integration tests carry a `//go:build integration` tag so plain
   `go test ./...` skips them; run with
   `go test -tags integration ./...`.
5. `Example` functions need an `// Output:` comment; without one they
   are compiled but never executed.

## Sources

1. Organizing a Go module (official): https://go.dev/doc/modules/layout
2. Effective Go (official): https://go.dev/doc/effective_go
3. Alex Edwards, A Time-Saving Makefile for Your Go Projects:
   https://www.alexedwards.net/blog/a-time-saving-makefile-for-your-go-projects
4. TutorialEdge, Makefiles for Go Developers:
   https://tutorialedge.net/golang/makefiles-for-go-developers/
5. mkgoprj, self-documenting help convention:
   https://github.com/nao1215/mkgoprj
6. golangci-lint: https://golangci-lint.run/
7. golang-standards/project-layout, scripts/ keeps the root Makefile
   thin: https://github.com/golang-standards/project-layout
8. Local practice: `projects/web/lanban/Makefile` (structured style,
   exported env, tool guards) and `projects/web/starcuts/Makefile`
   (build tags, per-OS dependency targets).
