# Agent Instructions

This repository follows the homelab project baseline. Treat these files as durable project policy, not as generated boilerplate.

## Working Rules

- Keep changes small and scoped to the repository type.
- Preserve `README.md`, `AGENTS.md`, `CLAUDE.md`, `.gitlab-ci.yml`, `.pre-commit-config.yaml`, `.yamllint.yaml`, `.alint.yml`, `.gitignore`, and `mise.toml` unless the repository has a documented exception.
- Use tagged `ci-templates` components and tagged `project-template-default` alint policies for stable repositories.
- Keep technology-specific requirements in alint profiles instead of adding unrelated files to every repo.
- Do not commit secrets, `.env` files, `mise.local.toml`, kubeconfigs, Terraform/OpenTofu state, generated caches, or dependency directories.

## Validation

Prose in Markdown files is linted with `proselint` through pre-commit. Keep exceptions intentional and local.

Run these before opening a merge request when repository structure or policy changes:

```sh
pre-commit run --all-files
alint check
```

For narrow application changes, run the smallest relevant test or lint command from the repo documentation.

## Repository Types

Use the base policy everywhere. Add exactly the profile that matches the repo shape:

- Container image repos extend `.alint/profiles/container.yml` and keep `Dockerfile` at the repository root.
- Helm chart repos extend `.alint/profiles/helm-chart.yml` and keep `Chart.yaml` plus `values.yaml` at the repository root.
- OpenTofu or Terragrunt repos extend `.alint/profiles/opentofu-terragrunt.yml` and must not track state or variable secret files.
