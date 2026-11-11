## Related issue

Fixes #<!-- issue number -->

## Description

<!-- What does this change, and why? Call out any design decisions, Kibana quirks or trade-offs. -->

## Checklist

- [ ] Unit tests added or updated, and `make test` passes
- [ ] Acceptance tests added or updated (`TestAcc...`), and `make testacc` passes against a local stack (`docker compose up -d --wait`)
- [ ] Tested against every supported Kibana version (`ELASTIC_STACK_VERSION`, see the matrix in `.github/workflows/test.yml`) where behavior differs
- [ ] New service methods carry a `// Kibana API: METHOD /path` annotation, and `make coverage` was run
- [ ] `make lint` passes
- [ ] `CHANGELOG.md` updated under `## Unreleased`
