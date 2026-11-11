# Contributing

Thanks for your interest in improving go-kibana. Bug reports, feature requests and pull requests are
all welcome. By participating you agree to follow the [Code of Conduct](.github/CODE_OF_CONDUCT.md).

## Reporting issues

Use the [issue templates](https://github.com/ottramst/go-kibana/issues/new/choose). Always include the
go-kibana, Go and Kibana versions. Report security vulnerabilities privately, as described in
[SECURITY.md](SECURITY.md).

Planned work is tracked in [milestones](https://github.com/ottramst/go-kibana/milestones): each epic
issue lists its endpoints as sub-issues. Comment on a sub-issue before starting on it, so work isn't
duplicated.

## Development setup

Requirements: [Go](https://go.dev/doc/install) (1.26 or later) and
[golangci-lint](https://golangci-lint.run/) (the version pinned in `.github/workflows/lint.yml`).

```shell
make build   # build the package
make test    # unit tests
make lint    # golangci-lint
make fmt     # gofmt -s
```

## Pull requests

- Keep each pull request focused on one change, and add or update tests for it.
- Use a [Conventional Commits](https://www.conventionalcommits.org/) title, e.g. `feat(spaces): add copy saved objects`;
  pull requests are squash-merged with that title. Mark breaking changes with `!`.
- Add an entry to `CHANGELOG.md` under `## Unreleased` (`BREAKING CHANGES`, `FEATURES`, `ENHANCEMENTS`,
  `BUG FIXES` or `NOTES`).
- CI must pass: the `CI gate` check covers lint, build and unit tests on every supported Go version.

## Releases

Maintainers release by pushing a `vX.Y.Z` tag. Before tagging, move the `## Unreleased` entries in
`CHANGELOG.md` under a new `## vX.Y.Z` heading; the release workflow publishes that section as the
GitHub release notes and warms the Go module proxy so pkg.go.dev picks up the version.
