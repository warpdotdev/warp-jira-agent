#!/bin/bash
set -e

echo "Starting repository setup..."

# Create repos directory if it doesn't exist
mkdir -p /app/repos
cd /app/repos

# Function to clone or update a repository
clone_or_update_repo() {
    local repo_path=$1
    local branch=$2
    # Extract repo name from URL (part after the slash)
    local repo_name=$(basename "$repo_path")
    
    if [ -d "$repo_name" ]; then
        echo "Repository $repo_name exists, updating to branch $branch..."
        cd "$repo_name"
        git fetch origin
        git checkout "$branch"
        git reset --hard "origin/$branch"
        cd ..
    else
        echo "Cloning $repo_name from $repo_path (branch: $branch)..."
        gh repo clone "$repo_path" "$repo_name"
        cd "$repo_name"
        git checkout "$branch"
        cd ..
    fi
}

# Require GitHub token for private repos
if [ -z "$GITHUB_TOKEN" ]; then
    echo "ERROR: GITHUB_TOKEN environment variable is required for private repositories."
    echo "Please set GITHUB_TOKEN in your .env file."
    exit 1
fi

# Authenticate with GitHub CLI
echo "Authenticating with GitHub CLI..."
# Set GH_TOKEN instead of GITHUB_TOKEN to avoid CLI warnings
GH_TOKEN="$GITHUB_TOKEN"
unset GITHUB_TOKEN
echo "$GH_TOKEN" | gh auth login --with-token
gh auth setup-git

# Read repositories from repos.yaml and clone/update each one
echo "Reading repositories from repos.yaml..."
yq eval '.repositories[] | [.url, .branch] | @tsv' /app/repos.yaml | while IFS=$'\t' read -r repo_path branch; do
    repo_name=$(basename "$repo_path")
    echo "Processing: $repo_name -> $repo_path (branch: $branch)"
    clone_or_update_repo "$repo_path" "$branch"
done

cd /app

echo "All repositories ready!"

echo "Starting Slack bot application..."

exec /usr/bin/warp-jira-agent