"""Module for polling Jira issues and managing Warp agent tasks."""

import asyncio
import logging

from jira import JIRA
from jira.resources import Issue
from warp_agent_sdk import AsyncWarpAPI

from .task import build_issue_task, monitor_task, transition_issue_status

logger = logging.getLogger(__name__)

# Polling interval for continuous polling loop
POLLING_INTERVAL_SECONDS = 30


async def poll_issues_continuously(
    jira: JIRA,
    warp_client: AsyncWarpAPI,
    jql_query: str,
    max_issues: int = 100,
) -> None:
    """Continuously poll for Jira issues in a loop.

    This function runs indefinitely, polling for issues every 30 seconds.

    Args:
        jira: Authenticated Jira client
        warp_client: Authenticated async Warp API client
        jql_query: JQL query string to search for issues
        max_issues: Maximum number of issues to process per poll (default: 100)
    """
    logger.info("Starting continuous polling loop")
    while True:
        try:
            await poll_issues(jira, warp_client, jql_query, max_issues)
        except Exception as e:
            logger.error(f"Error during polling iteration: {e}", exc_info=True)

        # Wait before next poll
        logger.debug(f"Waiting {POLLING_INTERVAL_SECONDS} seconds before next poll")
        await asyncio.sleep(POLLING_INTERVAL_SECONDS)


async def poll_issues(
    jira: JIRA,
    warp_client: AsyncWarpAPI,
    jql_query: str,
    max_issues: int = 100,
) -> None:
    """Search for Jira issues and spawn Warp agent tasks for each.

    Args:
        jira: Authenticated Jira client
        warp_client: Authenticated async Warp API client
        jql_query: JQL query string to search for issues
        max_issues: Maximum number of issues to process (default: 100)
    """
    # Search for issues matching the JQL query, excluding those already assigned to Warp
    filtered_query = f"({jql_query}) AND (labels IS empty OR labels != warp-assigned)"
    issues = jira.search_issues(filtered_query, maxResults=max_issues)

    if len(issues) > 0:
        logger.info(f"Found {len(issues)} issues to process")

    for issue in issues:
        logger.info(f"Processing issue {issue.key}: {issue.fields.summary}")
        try:
            await process_issue(jira, warp_client, issue)
        except Exception as e:
            logger.error(f"Failed to process issue {issue.key}: {e}", exc_info=True)
            # Continue with next issue rather than failing completely


async def process_issue(jira: JIRA, warp_client: AsyncWarpAPI, issue: Issue) -> None:
    """Process a single Jira issue by spawning and monitoring a Warp agent task.

    Args:
        jira: Authenticated Jira client
        warp_client: Authenticated async Warp API client
        issue: Jira issue to process
    """
    issue_key = issue.key

    task_params = build_issue_task(issue)

    # Spawn the Warp agent task.
    logger.info(f"Spawning Warp agent task for issue {issue_key}")
    try:
        task_response = await warp_client.agent.run(**task_params)
    except Exception as e:
        logger.error(f"Failed to spawn Warp agent task for issue {issue_key}: {e}", exc_info=True)
        jira.add_comment(
            issue_key,
            f"⚠️ Failed to spawn Warp agent task.\n\nError: {e}",
        )
        return

    task_id = task_response.task_id
    logger.info(f"Created task {task_id} for issue {issue_key}")

    # Add warp-assigned label to mark this issue as being worked on.
    # This prevents duplicate tasks across runs of the agent.
    issue.add_field_value("labels", "warp-assigned")

    comment = jira.add_comment(
        issue_key,
        f"🤖 Warp is working on this issue (task ID: {task_id})...",
    )
    comment_id = comment.id
    logger.info(f"Added comment {comment_id} to issue {issue_key}")

    # Transition issue to In Progress
    try:
        transition_issue_status(jira, issue_key, "In Progress")
    except Exception as e:
        logger.warning(f"Failed to transition {issue_key} to In Progress: {e}")

    # Monitor the task in the background.
    asyncio.create_task(monitor_task(jira, warp_client, issue_key, comment_id, task_id))
