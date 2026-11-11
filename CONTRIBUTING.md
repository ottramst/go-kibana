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

## Acceptance tests

Acceptance tests (`TestAcc*`) run against a real Elasticsearch and Kibana. They are skipped unless
`KIBANA_ACC` is set. A Docker Compose file is provided (about 4 GB of memory);
`ELASTIC_STACK_VERSION` selects the image tags and defaults to the newest supported version.

```shell
docker compose up -d --wait
# ELASTIC_STACK_VERSION=8.19.25 docker compose up -d --wait

make testacc
# or a single test:
KIBANA_ACC=1 go test -v -run TestAccSpaces ./...

docker compose down -v
```

The tests log in as `elastic` / `changeme`. To run them against another Kibana, set `KIBANA_URL` and
either `KIBANA_API_KEY` or `KIBANA_USERNAME` and `KIBANA_PASSWORD`. Each test creates its own
uniquely named objects and deletes them afterwards.

Behavior that differs between Kibana versions must be covered for each supported version: run the suite
against every version in the test matrix before opening a pull request.

## Supported Kibana versions

CI tests the latest patch of the supported Kibana release lines (see the
[support policy](README.md#support-policy)). When Elastic ships a new patch or minor release, bump these together:

1. The matrix in `.github/workflows/test.yml`
2. The `ELASTIC_STACK_VERSION` default in `docker-compose.yml`
3. The spec versions in `GNUmakefile` (`KIBANA_SPEC_VERSIONS`), then run `make coverage`
4. The tested versions in `README.md` and `CLAUDE.md`

## Pull requests

- Keep each pull request focused on one change, and add or update tests for it.
- Use a [Conventional Commits](https://www.conventionalcommits.org/) title, e.g. `feat(spaces): add copy saved objects`;
  pull requests are squash-merged with that title. Mark breaking changes with `!`.
- Add an entry to `CHANGELOG.md` under `## Unreleased` (`BREAKING CHANGES`, `FEATURES`, `ENHANCEMENTS`,
  `BUG FIXES` or `NOTES`).
- CI must pass: the `CI gate` check covers lint, build and unit tests on every supported Go version, and the
  acceptance test matrix.

## Releases

Maintainers release by pushing a `vX.Y.Z` tag. Before tagging, move the `## Unreleased` entries in
`CHANGELOG.md` under a new `## vX.Y.Z` heading; the release workflow publishes that section as the
GitHub release notes and warms the Go module proxy so pkg.go.dev picks up the version.
