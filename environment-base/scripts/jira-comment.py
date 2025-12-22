import argparse
import os

from jira import JIRA


def main() -> None:
    parser = argparse.ArgumentParser(description="Post a comment to a Jira issue")
    parser.add_argument("issue_id", help="The Jira issue ID (e.g., PROJ-123)")
    parser.add_argument("comment", help="The comment text to post")
    args = parser.parse_args()

    jira_server = os.getenv("JIRA_SERVER")
    jira_email = os.getenv("JIRA_EMAIL")
    jira_api_key = os.getenv("JIRA_API_KEY")

    if not all([jira_server, jira_email, jira_api_key]):
        raise ValueError(
            "Missing required environment variables: JIRA_SERVER, JIRA_EMAIL, JIRA_API_KEY"
        )

    jira = JIRA(
        server=jira_server,
        basic_auth=(jira_email, jira_api_key),
    )

    jira.add_comment(args.issue_id, args.comment)
    print(f"Comment posted to {args.issue_id}")


if __name__ == "__main__":
    main()
