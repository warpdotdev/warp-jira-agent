FROM golang:1.25 AS build-stage

WORKDIR /usr/src/app

# Copy dependencies for caching
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -v -o /usr/local/bin/warp-jira-agent

FROM debian:trixie-slim AS run-stage

ARG CHANNEL
ARG TARGETARCH

RUN export DEBIAN_FRONTEND=noninteractive && \
    apt-get update \
    && apt-get install -y --no-install-recommends \
    ca-certificates wget gnupg \
    && wget -qO- https://cli.github.com/packages/githubcli-archive-keyring.gpg | gpg --dearmor -o /usr/share/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=${TARGETARCH} signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" > /etc/apt/sources.list.d/github-cli.list \
    && apt-get update \
    && apt-get install -y gh \
    && if [ "$CHANNEL" = "dev" ]; then \
        wget --output-document=warp-cli.deb "https://staging.warp.dev/download/cli?os=linux&package=deb&channel=dev&arch=${TARGETARCH}"; \
    else \
        wget --output-document=warp-cli.deb "https://app.warp.dev/download/cli?os=linux&package=deb&arch=${TARGETARCH}"; \
    fi \
    && dpkg -i warp-cli.deb || (apt-get update && apt-get install -f -y) \
    && dpkg -i warp-cli.deb \
    && rm -rf /var/lib/apt/lists /var/cache/apt/archives warp-cli.deb

WORKDIR /app
COPY --from=build-stage /usr/local/bin/warp-jira-agent /usr/bin/warp-jira-agent

ENTRYPOINT ["/usr/bin/warp-jira-agent"]
