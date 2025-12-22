"""Main module for Warp Jira Agent."""

import logging
import os

from jira import JIRA

logger = logging.getLogger(__name__)


def main() -> None:
    """Main entry point for the application."""
    logger.info("Starting Warp Jira Agent")

    jira_server = os.getenv("JIRA_SERVER")
    jira_email = os.getenv("JIRA_EMAIL")
    jira_api_key = os.getenv("JIRA_API_KEY")

    if not jira_server:
        logger.error("JIRA_SERVER environment variable is required but not set")
        return
    if not jira_email:
        logger.error("JIRA_EMAIL environment variable is required but not set")
        return
    if not jira_api_key:
        logger.error("JIRA_API_KEY environment variable is required but not set")
        return

    jira = JIRA(
        server=jira_server,
        basic_auth=(jira_email, jira_api_key),
    )

    me = jira.myself()
    logger.info(f"Authenticated as {me['displayName']}")


if __name__ == "__main__":
    main()
