# syntax=docker/dockerfile:1.4
FROM golang:1.25-trixie

# Install system packages.
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates git python3 python3-jira

# Install dotenvx (requires curl).
RUN curl -sfS https://dotenvx.sh/install.sh | sh

# Copy over scripts for the agent to use.
COPY scripts/ /usr/local/bin/

# Copy over configuration.
COPY --from=config .env.local /config/.env.local
