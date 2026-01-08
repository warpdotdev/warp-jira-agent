FROM python:3.14-slim AS builder
COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/

# Install and build the app and dependencies into /app.
WORKDIR /app

# 1) Install dependencies (without pulling in the app code).
RUN --mount=type=cache,target=/root/.cache/uv \
    --mount=type=bind,source=uv.lock,target=uv.lock \
    --mount=type=bind,source=pyproject.toml,target=pyproject.toml \
    uv sync --locked --no-install-project --no-editable

# 2) Copy the app into the image.
COPY . /app
RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --locked --no-editable

FROM python:3.14-slim

WORKDIR /app

# Install system packages
RUN apt-get update && apt-get install -y curl && rm -rf /var/lib/apt/lists/*

# Install dotenvx
RUN curl -sfS https://dotenvx.sh/install.sh | sh

# Copy the environment, but not the source code
COPY --from=builder --chown=app:app /app/.venv /app/.venv

# Run the application
CMD ["dotenvx", "run", "--", "/app/.venv/bin/warp-jira-agent"]