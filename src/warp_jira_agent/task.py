"""Functions for building Warp agent tasks from Jira issues."""

import asyncio
import json
import logging
import os
from pathlib import Path

from typing import Any, cast

import yaml
from jira import JIRA
from jira.resources import Issue
from warp_agent_sdk import AsyncWarpAPI
from warp_agent_sdk.types import AgentRunParams
from warp_agent_sdk.types.agent import TaskItem
from warp_agent_sdk.types.ambient_agent_config_param import AmbientAgentConfigParam

logger = logging.getLogger(__name__)


def load_task_config() -> AmbientAgentConfigParam | None:
    """Load task configuration from file specified in WARP_TASK_CONFIG_FILE environment variable.

    The config file can be either JSON or YAML format.

    Returns:
        Task configuration object, or None if no config file is specified

    Raises:
        FileNotFoundError: If the config file doesn't exist
        ValueError: If the config file format is invalid or unsupported
    """
    config_file_path = os.getenv("WARP_TASK_CONFIG_FILE")
    if not config_file_path:
        logger.debug("No WARP_TASK_CONFIG_FILE environment variable set")
        return None

    config_path = Path(config_file_path)
    if not config_path.exists():
        raise FileNotFoundError(f"Task config file not found: {config_file_path}")

    logger.info(f"Loading task config from {config_file_path}")

    # Determine format based on file extension
    suffix = config_path.suffix.lower()
    with open(config_path) as f:
        if suffix in (".json",):
            return cast(AmbientAgentConfigParam, json.load(f))
        elif suffix in (".yaml", ".yml"):
            return cast(AmbientAgentConfigParam, yaml.safe_load(f))
        else:
            raise ValueError(f"Unsupported config file format: {suffix}. Use .json, .yaml, or .yml")


def build_issue_task(issue: Issue) -> AgentRunParams:
    """Build a Warp agent task from a Jira issue.

    Args:
        issue: A Jira Issue for the agent to work on

    Returns:
        AgentRunParams suitable for spawning an agent to investigate or address the issue
    """
    # Extract key issue details
    summary = issue.fields.summary
    issue_key = issue.key

    # Convert issue to JSON representation for the prompt.
    issue_dict = issue.raw
    issue_json = json.dumps(issue_dict, indent=2)

    prompt = f"""
Address the following Jira issue to the best of your ability. You are given \
information about the issue in a simplified XML format.
<issue-key>{issue_key}</issue-key>
<issue-summary>{summary}</issue-summary>
<issue>{issue_json}</issue>

As you make progress on the issue, you can post comments by running this command:
<comment-command>
jira-comment "{{issue-key}}" "{{comment-text}}"
</comment-command>

If you make any code changes, submit them as PRs to the modified repository.

<system-reminder>
DO NOT respond in XML, even though the issue description uses XML
</system-reminder>"""

    # Build the AgentRunParams
    params: AgentRunParams = {
        "prompt": prompt,
    }

    # Load task config from file if specified
    config = load_task_config()
    if config:
        params["config"] = config

    return params


# Polling intervals in seconds
POLL_INTERVAL_PENDING = 5  # Poll frequently while task is pending
POLL_INTERVAL_IN_PROGRESS = 30  # Poll periodically while task is in progress


def get_task_error_message(task_status: TaskItem) -> str:
    """Extract error message from a failed task status.

    Args:
        task_status: The task status item

    Returns:
        The error message, or "Unknown error" if not available
    """
    error_msg = task_status.status_message.message if task_status.status_message else None
    return error_msg if error_msg else "Unknown error"


async def monitor_task(
    jira: JIRA,
    warp_client: AsyncWarpAPI,
    issue_key: str,
    comment_id: str,
    task_id: str,
) -> TaskItem:
    """Monitor a Warp agent task and update Jira issue as it progresses.

    This function polls the task status and updates the Jira issue:
    - When a session sharing link becomes available, updates the comment with the link
    - When the task fails, updates the comment with the error
    - When the task succeeds, transitions the issue to "In Review"

    Args:
        jira: Authenticated Jira client
        warp_client: Authenticated async Warp API client
        issue_key: Issue key (e.g., 'WEB-1')
        comment_id: ID of the existing Jira comment to update
        task_id: ID of the task to monitor

    Returns:
        The final TaskItem status

    Raises:
        Any exceptions from the Warp API client
    """
    logger.info(f"Starting to monitor task {task_id} for issue {issue_key}")
    session_link_updated = False
    session_link: str | None = None

    # Poll for session sharing link and task completion
    logger.debug(f"Monitoring task {task_id} for updates...")

    while True:
        task_status = await warp_client.agent.tasks.retrieve(task_id)
        state = task_status.state

        logger.debug(f"Task {task_id} state: {state}")

        # Check if task failed after starting
        if state == "FAILED":
            error_message = get_task_error_message(task_status)
            logger.error(f"Task {task_id} failed: {error_message}")
            # Update existing comment with failure, including session link if available
            failure_text = f"⚠️ Warp agent task failed.\n\nError: {error_message}"
            if session_link:
                failure_text += f"\n\nView session: {session_link}"
            update_comment(jira, issue_key, comment_id, failure_text)
            return task_status

        # Post the session-sharing link if this is the first time it's available.
        if task_status.session_link and not session_link_updated:
            session_link = task_status.session_link
            logger.info(f"Session sharing link available: {session_link}")
            update_comment(
                jira,
                issue_key,
                comment_id,
                f"🤖 Warp is working on this issue (task ID: {task_id}).\n\n"
                f"View live session: {session_link}",
            )
            session_link_updated = True

        # Task completed successfully
        if state == "SUCCEEDED":
            logger.info(f"Task {task_id} succeeded")
            # Update comment to indicate completion
            success_text = f"✅ Warp has finished working on this issue (task ID: {task_id})."
            if session_link:
                success_text += f"\n\nView session: {session_link}"
            update_comment(jira, issue_key, comment_id, success_text)
            # Transition issue to In Review
            try:
                transition_issue_status(jira, issue_key, "In Review")
            except Exception as e:
                logger.warning(f"Failed to transition {issue_key} to In Review: {e}")
            return task_status

        # Continue polling - use shorter interval until session link is available.
        poll_interval = (
            POLL_INTERVAL_PENDING if not session_link_updated else POLL_INTERVAL_IN_PROGRESS
        )
        await asyncio.sleep(poll_interval)


def update_comment(jira: JIRA, issue_key: str, comment_id: str | None, new_text: str) -> str:
    """Update an existing Jira comment or create a new one if comment_id is None.

    Args:
        jira: Authenticated Jira client
        issue_key: Issue key (e.g., 'WEB-1')
        comment_id: ID of the comment to update, or None to create a new comment
        new_text: New text for the comment

    Returns:
        The comment ID (either the existing one or newly created one)
    """
    if comment_id is None:
        # Create new comment
        comment = jira.add_comment(issue_key, new_text)
        logger.info(f"Created comment {comment.id} on issue {issue_key}")
        return str(comment.id)
    else:
        # Update existing comment
        comment = jira.comment(issue_key, comment_id)
        comment.update(body=new_text)
        logger.info(f"Updated comment {comment_id} on issue {issue_key}")
        return comment_id


def transition_issue_status(jira: JIRA, issue_key: str, status_name: str) -> None:
    """Transition a Jira issue to a new status.

    Args:
        jira: Authenticated Jira client
        issue_key: Issue key (e.g., 'WEB-1')
        status_name: Target status name (e.g. 'In Progress')

    Raises:
        ValueError: If the transition is not available for this issue
        Exception: For other Jira API errors
    """
    # Get available transitions for the issue
    transitions = jira.transitions(issue_key)

    # Find the first available transition to the intended status.
    transition_id = None
    for transition in transitions:
        to_status = transition.get("to", {})
        if to_status.get("name", "") == status_name:
            transition_id = transition["id"]
            break

    if transition_id is None:
        # Build helpful error message with available target statuses
        available = ", ".join([t.get("to", {}).get("name", "Unknown") for t in transitions])
        raise ValueError(
            f"Status '{status_name}' not available for {issue_key}. "
            f"Available target statuses: {available}"
        )

    # Perform the transition
    jira.transition_issue(issue_key, transition_id)
    logger.info(f"Transitioned {issue_key} to '{status_name}'")
