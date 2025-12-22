# WARP.md

This file provides guidance to WARP (warp.dev) when working with code in this repository.

## Project Overview

This is a Python-based Jira integration agent that uses Warp's coding capabilities to respond to Jira issues. The project is in early development with basic authentication scaffolding in place.

## Build System and Package Management

This project uses **uv** (https://docs.astral.sh/uv/) for dependency management and building. Do NOT use pip or other Python package managers.

### Common Commands

**Setup and dependencies:**
- `uv sync` - Install project dependencies
- `uv sync --extra dev` - Install with development dependencies (ruff, mypy)
- `uv python install 3.14` - Install Python 3.14 (required version)

**Running the application:**
- `uv run warp-jira-agent` - Run the main application entry point
- `uv run python -m warp_jira_agent.main` - Alternative way to run main module

**Code quality:**
- `uv run ruff format .` - Format all Python code
- `uv run ruff format --check .` - Check formatting without modifying files
- `uv run ruff check .` - Run linter
- `uv run mypy --install-types --non-interactive src/warp_jira_agent` - Run type checker

## Environment Configuration

The application requires three environment variables for Jira authentication:
- `JIRA_SERVER` - The Jira instance URL
- `JIRA_EMAIL` - User email for authentication
- `JIRA_API_KEY` - API key for authentication

This project uses **dotenvx** for encrypted environment variable management. Environment files:
- `.env.local` - Encrypted local environment variables (committed)
- `.env.keys` - Decryption keys (NOT committed to source control)

## Code Style and Quality Standards

### Ruff Configuration
- Line length: 100 characters
- Target Python version: 3.14
- Enabled linters: pycodestyle (E, W), pyflakes (F), isort (I), flake8-bugbear (B), flake8-comprehensions (C4), pyupgrade (UP)

### Mypy Configuration
Strict type checking is enabled with the following requirements:
- All functions must have type annotations (`disallow_untyped_defs`)
- No incomplete type definitions (`disallow_incomplete_defs`)
- Strict equality checks enabled
- No implicit optional types

When adding new code, ensure all functions have complete type annotations including parameter types and return types.

## Project Structure

```
src/warp_jira_agent/
├── __init__.py      # Package initialization with logging configuration
└── main.py          # Main entry point with Jira authentication logic
```

The project follows a simple structure:
- **src/warp_jira_agent/__init__.py** - Configures basic logging (INFO level) for the application
- **src/warp_jira_agent/main.py** - Contains the `main()` function which handles environment variable validation and Jira client initialization

## CI/CD

GitHub Actions workflow (`.github/workflows/ci.yml`) runs on all pushes and pull requests to main:
1. Format checking (no modifications)
2. Linting
3. Type checking

The workflow uses concurrent execution with cancellation of in-progress runs for the same ref.

## Development Workflow

1. Make sure uv is installed
2. Run `uv sync --extra dev` to install all dependencies
3. Before committing, run all quality checks locally:
   - Format: `uv run ruff format .`
   - Lint: `uv run ruff check .`
   - Type check: `uv run mypy --install-types --non-interactive src/warp_jira_agent`
4. All three checks must pass for CI to succeed
