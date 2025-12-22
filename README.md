# Warp Jira Agent

Use Warp's coding agent to respond to Jira issues.

## Development

### Setup

This project requires [uv](https://docs.astral.sh/uv/) to build.

### Running Locally

Use both `dotenvx` and `uv` to run with local environment variables:

```sh
$ dotenvx run -f .env.local -- uv run warp-jira-agent
```

### Formatting and Linting

* Format with `uv run ruff format .`
* Lint with `uv run ruff check .`
* Typecheck with `uv run mypy --install-types --non-interactive src/warp_jira_agent`