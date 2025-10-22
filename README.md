# Warp Jira Agent

This repo implements a Warp agent for Jira by polling its issue-search endpoint and running the Warp CLI.

## Setup

### 1. Environment Setup

Copy `.env.example` to `.env` and fill in the values. You'll need:
* A Warp API key (see https://docs.warp.dev/developers/cli#generating-api-keys)
* An Atlassian API token (see https://support.atlassian.com/atlassian-account/docs/manage-api-tokens-for-your-atlassian-account/)
* A GitHub personal access token (see https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)
* The URL of your Jira instance (e.g. https://warp-dev.atlassian.net)

Set the `WARP_JIRA_LABEL` to the name of the Jira label you'll assign to issues, like `warp-assigned`.

### 2. Repository Configuration

Copy `repos.yaml.template` to `repos.yaml` and fill in entries for all the repositories you'd like the agent to clone and have access to.

## 3. Prompt Tuning

If you're curious about the agent's prompt, or would like to make any changes, edit `cmd/poll.go`.

## Running the agent

```sh
# Build and start the container
docker-compose up --build --detach

# View logs
docker-compose logs -f

# Stop the agent
docker-compose down

# Delete agent state
docker-compose down --volumes

# View logs for a particular issue
docker-compose exec warp-jira-agent cat workspaces/$ISSUE/output.log
```

## Usage

1. Add your chosen label to an issue
2. Wait for the bot to comment on the issue