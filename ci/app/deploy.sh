#!/usr/bin/env bash

source ./ci/app/compute-env.sh

echo "" > .env.sf.db
env|grep "POSTGRES_"|while read; do
  echo "${REPLY}" >> .env.sf.db
done

echo "JWT_SECRET=${JWT_SECRET}" > .env.sf.api
# DISCORD_TOKEN isn't SF_/CWCLOUD_-prefixed, so (like JWT_SECRET) it's written
# explicitly rather than swept up by the grep below. compute-env.sh maps it
# from the runner's DISCORD_TOKEN_PUBLIC.
echo "DISCORD_TOKEN=${DISCORD_TOKEN}" >> .env.sf.api
env|grep -E "(SF|CWCLOUD)_"|while read; do
  echo "${REPLY}" >> .env.sf.api
done

echo "SF_API_URL=${SF_API_URL}" > .env.sf.ui
echo "SF_UI_URL=${SF_UI_URL}" >> .env.sf.ui
echo "SF_MAX_IMAGE_SIZE=${SF_MAX_IMAGE_SIZE}" >> .env.sf.ui
echo "SF_GIT_REPO_URL=${SF_GIT_REPO_URL}" >> .env.sf.ui

docker ps -a | grep -i sf | awk '{system ("docker rm -f "$1)}' || :
docker login "${CI_REGISTRY}" --username "${CI_REGISTRY_USER}" --password "${CI_REGISTRY_PASSWORD}"
if ! docker buildx imagetools inspect "${CI_REGISTRY}/sf-ui:${UI_VERSION}" > /dev/null 2>&1; then
  export UI_VERSION="${VERSION}"
  echo "Deploying non-mobile version"
fi
docker compose -f docker-compose-live.yml up -d --force-recreate
docker logs sf-db-migrate || :
