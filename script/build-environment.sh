#!/bin/bash

env_name="${1:-base}"

docker buildx build \
    --platform linux/amd64 \
    -t "bennavetta/jira-$env_name" \
    "./environment/$env_name"

docker push "bennavetta/jira-$env_name"
