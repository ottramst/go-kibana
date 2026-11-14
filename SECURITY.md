# Security Policy

## Supported versions

Security fixes are released as a patch of the latest v1.x minor version. Since v1 releases are
backward compatible, upgrading to the latest minor version is safe.

| Module version     | Supported          |
| ------------------ | ------------------ |
| latest v1.x minor  | :white_check_mark: |
| older v1.x minors  | :x:                |
| v0.x               | :x:                |

See the [support policy](README.md#support-policy) for the supported Go and Kibana versions.

## Reporting a vulnerability

Please **do not** open a public issue for security vulnerabilities.

Report them privately through GitHub's
[private vulnerability reporting](https://github.com/ottramst/go-kibana/security/advisories/new).
Include a description of the issue, the affected module and Kibana versions, and steps to
reproduce it if possible.

You can expect an acknowledgement within a week. Once the issue is confirmed, a fix is released as soon
as practical and the advisory is published afterwards, crediting the reporter unless they prefer otherwise.

Vulnerabilities in Kibana itself should be reported to Elastic, following
its [security policy](https://www.elastic.co/community/security).
