#!/bin/bash

docker buildx build \
    --platform linux/amd64 \
    --build-context "config=." \
    -t "bennavetta/jira-base" \
    "./environment-base"

docker push bennavetta/jira-base
