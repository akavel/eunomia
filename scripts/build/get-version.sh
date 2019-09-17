#!/usr/bin/env bash

GIT_TAG=$(git tag --list 'v[0-9]*' --points-at HEAD)

if [ -z "$GIT_TAG" ]; then
    GIT_TAG="dev-$(git rev-parse --short HEAD)"
fi

echo "$GIT_TAG"
