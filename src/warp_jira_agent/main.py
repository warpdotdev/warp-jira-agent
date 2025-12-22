"""Main module for Warp Jira Agent."""

import asyncio
import logging
import os

from jira import JIRA
from warp_agent_sdk import AsyncWarpAPI

from .poll import poll_issues_continuously

logger = logging.getLogger(__name__)


async def async_main() -> None:
    """Async main function that runs the polling loop."""
    logger.info("Starting Warp Jira Agent")

    jira_server = os.getenv("JIRA_SERVER")
    jira_email = os.getenv("JIRA_EMAIL")
    jira_api_key = os.getenv("JIRA_API_KEY")
    warp_api_key = os.getenv("WARP_API_KEY")
    jql_query = os.getenv("JIRA_JQL_QUERY", "project = WEB AND status = 'To Do'")

    if not jira_server:
        logger.error("JIRA_SERVER environment variable is required but not set")
        return
    if not jira_email:
        logger.error("JIRA_EMAIL environment variable is required but not set")
        return
    if not jira_api_key:
        logger.error("JIRA_API_KEY environment variable is required but not set")
        return
    if not warp_api_key:
        logger.error("WARP_API_KEY environment variable is required but not set")
        return

    jira = JIRA(
        server=jira_server,
        basic_auth=(jira_email, jira_api_key),
    )

    me = jira.myself()
    logger.info(f"Authenticated as {me['displayName']}")

    warp_client = AsyncWarpAPI(api_key=warp_api_key)
    logger.info("Warp API client initialized")

    logger.info(f"Using JQL query: {jql_query}")

    # Run the continuous polling loop (this will run indefinitely)
    await poll_issues_continuously(jira, warp_client, jql_query)


def main() -> None:
    """Main entry point for the application."""
    asyncio.run(async_main())


if __name__ == "__main__":
    main()
