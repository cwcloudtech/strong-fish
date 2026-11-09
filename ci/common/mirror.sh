#!/usr/bin/env bash

REPO_PATH="${PROJECT_HOME}/strong-fish/"

cd "${REPO_PATH}"
git add .
git stash
git stash clear
git reset --hard origin/main
git pull --rebase origin main
git push -f github main
exit 0
