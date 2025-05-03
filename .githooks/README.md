# Git Hooks

This directory contains Git hooks used to enforce coding standards and processes in the BennWallet project.

## Available Hooks

- `commit-msg`: Validates that commit messages follow the [Conventional Commits](https://www.conventionalcommits.org/) format

## How to Enable Git Hooks

Run the following command to tell Git to use the hooks from this directory:

```bash
git config core.hooksPath .githooks
```

## Conventional Commits Format

All commits should follow this format:

```
<type>(<scope>): <description>
```

Where:
- `type` is one of: feat, fix, docs, style, refactor, perf, test, build, ci, chore
- `scope` is optional and identifies the section of the codebase (e.g., auth, ynab, ui)
- `description` is a short description of the change

For more details, see the commit template at `.github/COMMIT_TEMPLATE.md` 