#!/usr/bin/env bash

source ./ci/app/compute-env.sh

echo "" > .env.sf.db
env|grep "POSTGRES_"|while read; do
  echo "${REPLY}" >> .env.cwclock.db
done

echo "JWT_SECRET=${JWT_SECRET}" > .env.cwclock.api
env|grep -E "(CWCLOCK|CWCLOUD)_"|while read; do
  echo "${REPLY}" >> .env.cwclock.api
done

echo "SF_API_URL=${SF_API_URL}" > .env.cwclock.ui
echo "SF_UI_URL=${SF_UI_URL}" >> .env.cwclock.ui
echo "SF_MAX_IMAGE_SIZE=${SF_MAX_IMAGE_SIZE}" >> .env.cwclock.ui

docker ps -a | grep -i sf | awk '{system ("docker rm -f "$1)}' || :
docker compose -f docker-compose-live.yml up -d --force-recreate
docker logs sf-db-migrate || :
