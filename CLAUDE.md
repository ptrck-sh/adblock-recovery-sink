# Claude Instructions

Follow `AGENTS.md` first. It contains the shared repository policy for human and automated contributors.

When working in this repository:

- Prefer existing CI, pre-commit, Renovate, and alint conventions over introducing a new structure.
- Keep the default template generic; put container, Helm, and OpenTofu/Terragrunt requirements in their respective alint profiles.
- Use tagged remote policy references in examples so real repos can be updated by Renovate.
- Before proposing broad policy changes, inspect representative local repos and verify the rule will not force unrelated layouts into every project.
- Report validation results clearly, including any alint warnings that remain intentional.
