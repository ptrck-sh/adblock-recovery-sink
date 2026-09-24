+++
title = "Contributing"
description = "Development, profiles, commit messages, and security reporting."
weight = 13
+++

Install the project tool versions with `mise`, then run the Go test suite:

```sh
mise install
go test ./...
```

The manual smoke harness is in `test/smoke`; the end-to-end test that CI runs against the built binary is `test/e2e/run.sh`. Both use Go, `uv`, `certutil`, and Playwright browsers. See [Compatibility](@/compatibility.md) for the harness commands and scope.

To add a profile, create a directory with `profile.yaml` and its response body files. Follow the schema in [Configuration](@/configuration.md), keep the host and path scope narrow, and add tests for route matching and expected response behaviour.

Use Conventional Commits. Keep changes small and avoid comments in code, templates, configuration, and CI. Report security concerns privately through the [security policy](https://gitlab.com/ptrck-sh/adblock-recovery-sink/-/blob/main/SECURITY.md).
