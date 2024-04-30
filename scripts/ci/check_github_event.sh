#! /bin/bash -eux

# prompt error if non-supported event
if egrep -q 'release|push|pull_request|workflow_dispatch|workflow_run|schedule' <<<"${GITHUB_EVENT_NAME}"; then
  echo "Event: ${GITHUB_EVENT_NAME}"
else
  echo -e "Unsupport event: ${GITHUB_EVENT_NAME}! \nSupport: release | push | pull_request | workflow_dispatch | workflow_run | schedule"
  exit 1
fi