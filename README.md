# Warp Jira Agent

Use Warp's coding agent to respond to Jira issues using [Ambient Agents](https://docs.warp.dev/ambient-agents/ambient-agents-overview).

## Usage

### Install dependencies

You can build and run the Jira agent with either `uv` or Docker. Either will install any other dependencies as needed.

### Configure the agent

The Jira agent subscriber expects the following environment variables to be set:
* `JIRA_SERVER`: URL of your Jira server, e.g. `https://myco.atlassian.net`
* `JIRA_EMAIL`: email address of the Jira user to authenticate as
* `JIRA_API_KEY`: an API key tied to the Jira user email. See https://support.atlassian.com/atlassian-account/docs/manage-api-tokens-for-your-atlassian-account/
* `JIRA_JQL_QUERY`: JQL expression for finding issues for Warp to address. Use this to filter on specific labels, projects, statuses, and more. For example, use `statusCategory = "To Do" and label = "warp-fix"` to have Warp pick up issues labeled with `warp-fix`.
* `WARP_API_KEY`: a Warp API key. All ambient agents will run as the corresponding user or team.

We recommend storing these values with [dotenvx](https://dotenvx.com/).

Additionally, you can configure the agent itself via a file, in YAML or JSON format. This file
supports the same options as the `config` field in an
[`/agent/run` API request](https://docs.warp.dev/platform/agent-api-and-sdk/agent#post-agent-run).

The subscriber will automatically load `warp.config.json`, `warp.config.yaml`, or `warp.config.yml`.

```yaml
model_id: claude-4-5-opus # Use `warp model list` to get available models
environment_id: abc123 # Use `warp environment list` to get the environment ID
mcp_servers:
    myserver:
        url: https://mcp.myserver.com
    mycommand:
        command: my-local-mcp-server
```

### Run the subscriber

#### Directly

To start the Jira subscriber, run `uv run warp-jira-agent`.

#### Via Docker

First, build the Docker image:

```sh
docker build -t warp-jira-agent .
```

Then, run it with all environment variables passed in:

```sh
docker run --rm -it \
    -e JIRA_SERVER=... \
    -e JIRA_EMAIL=... \
    -e JIRA_API_KEY=... \
    -e JIRA_JQL_QUERY=... \
    -e WARP_API_KEY=... \
    warp-jira-agent
```

To include a configuration file, add `-v warp.config.json:/app/warp.config.json:ro` (or similar for YAML).

You can also load environment variables via dotenvx:

```sh
docker run --rm -it \
    -v .env:/app/.env:ro \
    -e DOTENV_PRIVATE_KEY=... \
    warp-jira-agent
```

### Operation

The subscriber will:
1. Poll for issues matching the given JQL expression
2. For each new issue, spawn an ambient agent tasked with resolving the issue. The subscriber also adds a `warp-assigned` label to each issue, to prevent duplicate agents.
3. Update the issue status as the agent progresses, moving into `In Progress` and then `In Review`.

## Environment Setup

This integration supports any [environment](https://docs.warp.dev/platform/cli/integrations-and-environments).

We also provide a base image which includes tools allowing the agent to post comments back to Jira.
This image, defined in [`./environment/base`](./environment/base), is published as `bennavetta/jira-base:latest`.

The Jira commenting tool assumes the `JIRA_SERVER`, `JIRA_EMAIL`, and `JIRA_API_KEY` environment
variables are set while the agent is running. You can do this with [Warp-managed secrets](https://docs.warp.dev/ambient-agents/agent-secrets):

```sh
warp secret create JIRA_SERVER
warp secret create JIRA_EMAIL
warp secret create JIRA_API_KEY
```

For example usage, see [`./environment/go`](./environment/go), which extends it to include a Go toolchain.

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
