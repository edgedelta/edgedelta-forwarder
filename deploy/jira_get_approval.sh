#!/usr/bin/env bash
# This script is used in approval process with JIRA and Circle CI with below steps:
# - Generates the changelog for the artifact
# - Creates a jira ticket in approval board with waiting status
# - Polls the ticket status until the given deadline.
# - If ticket is approved job is finished with success.
# - If approve is not granted in given period job or tocket has been closed CI job fails.
# Expects /tmp/workspace/ in circle ci to be mounted
#       - persist_to_workspace:
#          root: /tmp/workspace
#          paths:
#            - approval-ticket-id
# So that release completion can be notified in other jobs

# Requires git credentials to be set for change log generation

# Environment Variables Required
# APPROVAL_CHECK_PERIOD: sleep in seconds between approval checks
# APPROVAL_MAX_ATTEMPT: maximum number of attempts before timeout
# RELEASE_TYPE: artifact type as defined in changelog tool release-type: agent, api, web etc.
# JIRA_TOKEN_OWNER: username of the owner of the jira token
# JIRA_API_TOKEN: jira api token
# RELEASE_NAME: Release version

set -e

# Constants
base_uri=https://${JIRA_URL}/rest/api/2/issue
project_key="AP"

# Jira issue statuses in worflow
# Use below to get uptodate status ids
# curl \
#   -u $JIRA_TOKEN_OWNER:$JIRA_API_TOKEN \
#   -X GET \
#   -H "Content-Type: application/json" \
#   https://$JIRA_URL/rest/api/2/status
status_approved_id=400
status_closed_id=6
status_waiting_id=10003

# Jira transitions in workflow
# You can get transitions for an issue in a state as below
# curl \
# -u $JIRA_TOKEN_OWNER:$JIRA_API_TOKEN \
#   -X GET \
#   -H "Content-Type: application/json" \
#   $base_uri/$issue_id/transitions | jq
release_fail_transition_id=41
release_approved_transition_id=51

if [ "$APPROVAL_CHECK_PERIOD" == "" ]; then
  echo "APPROVAL_CHECK_PERIOD is a required parameter. Exiting with failure."
  exit 11
fi

if [ "$APPROVAL_MAX_ATTEMPT" == "" ]; then
  echo "APPROVAL_MAX_ATTEMPT is a required parameter. Exiting with failure."
  exit 12
fi

if [ "$RELEASE_TYPE" == "" ]; then
  echo "RELEASE_TYPE is a required parameter. Exiting with failure."
  exit 13
fi

if [ "$JIRA_TOKEN_OWNER" == "" ]; then
  echo "JIRA_TOKEN_OWNER is a required parameter. Exiting with failure."
  exit 14
fi

if [ "$JIRA_API_TOKEN" == "" ]; then
  echo "JIRA_API_TOKEN is a required parameter. Exiting with failure."
  exit 15
fi

if [ "$RELEASE_NAME" == "" ]; then
  echo "RELEASE_NAME is a required parameter. Exiting with failure."
  exit 16
fi

# Get change logs from change log tools as json
# Output format is timestamp author: message parsed with jq
# Remove end of commmit message double newlines with awk
git_root="$(git rev-parse --show-toplevel)"
description_body="Approval Circle CI Job: $CIRCLE_BUILD_URL\n\nReleasing Edge Delta Forwarder version to $RELEASE_NAME"

# Create Approval Ticket
create_body='{
    "fields": {
        "project": {"key": "'$project_key'"},
        "summary": "Release request for '$RELEASE_TYPE' '$RELEASE_NAME'",
        "description": "'$description_body'",
        "issuetype": {"name": "Task"}
    }
}'

resp="$(curl \
   -u $JIRA_TOKEN_OWNER:$JIRA_API_TOKEN \
   -X POST \
   --data "$create_body" \
   -H "Content-Type: application/json" \
   $base_uri)"
ticket_id="$(echo $resp | jq -r .id)"
if [ "$ticket_id" == "null" ] || [ "$ticket_id" == "" ]; then
  echo "Unable to get created ticket id: $resp. Exiting with failure."
  exit 1
fi

approved=0

# Wait for approval
attempt=0
while [ $attempt -lt $APPROVAL_MAX_ATTEMPT ]; do
  echo "Checking approval status for ticket $ticket_id"
  resp=$(curl \
   -u $JIRA_TOKEN_OWNER:$JIRA_API_TOKEN \
   -X GET \
   -H "Content-Type: application/json" \
   $base_uri/$ticket_id )
  status=$(echo $resp | jq -r '.fields.status.id')

  if [ "$status" == "$status_approved_id" ]; then
    echo "Approval received"
    approved=1
    break
  fi

  if [ "$status" == "$status_closed_id" ]; then
    echo "Approval not granted, ticket already closed. Exiting with failure."
    exit 2
  fi

  if [ "$status" == "$status_waiting_id" ]; then
    echo "Still waiting for approval"
    sleep $APPROVAL_CHECK_PERIOD
    (( attempt++ )) || true
    continue
  fi

  echo "Unknown/Invalid jira status: $status, response: $resp. Exiting with failure."
  exit 21
done

# Fail job if approval not received
if [ "$approved" == "0" ]; then
  echo "Approval not given in timeout period."

  timeout_transition_body='{"transition":{"id":"'$release_fail_transition_id'"}}'
  resp="$(curl \
    -u $JIRA_TOKEN_OWNER:$JIRA_API_TOKEN \
    -X POST \
    --data "$timeout_transition_body" \
    -H "Content-Type: application/json" \
    $base_uri/$ticket_id/transitions)"
  if [[ $resp == *"error"* ]]; then
    echo "Set timeout transition failed. Exiting with failure. Resp: $resp"
    exit 22
  fi

  timeout_comment_body='{
    "transition": {
      "id": '$release_fail_transition_id'
    },
    "update": {
        "comment": [
            {
                "add": {
                    "body": "Approval not given in timeout period."
                }
            }
        ]
    }
  }'
  resp="$(curl \
    -u $JIRA_TOKEN_OWNER:$JIRA_API_TOKEN \
    -X PUT \
    --data "$timeout_comment_body" \
    -H "Content-Type: application/json" \
    $base_uri/$ticket_id)"
  if [[ $resp == *"error"* ]]; then
    echo "Set timeout comment failed. Exiting with failure. Resp: $resp"
    exit 23
  fi

  echo "Approval not received in timeout duration. Exiting with failure."
  exit 24
fi

# Approval received mark ticket as in progress
in_progress_transition_body='{"transition":{"id":"'$release_approved_transition_id'"}}'
resp="$(curl \
  -u $JIRA_TOKEN_OWNER:$JIRA_API_TOKEN \
  -X POST \
  --data "$in_progress_transition_body" \
  -H "Content-Type: application/json" \
  $base_uri/$ticket_id/transitions)"
if [[ $resp == *"error"* ]]; then
  echo "Set in progress transition failed. Exiting with failure. Resp: $resp"
  exit 25
fi

in_progress_comment_body='{
  "update": {
      "comment": [
          {
              "add": {
                  "body": "Approved release job. Release is in progress."
              }
          }
      ]
  }
}'

echo "Sending in progress comment to ticket $ticket_id"
resp="$(curl \
  -u $JIRA_TOKEN_OWNER:$JIRA_API_TOKEN \
  -X PUT \
  --data "$in_progress_comment_body" \
  -H "Content-Type: application/json" \
  $base_uri/$ticket_id)"
if [[ $resp == *"error"* ]]; then
  echo "Set in progress comment failed. Exiting with failure. Resp: $resp"
  exit 26
fi

echo "$ticket_id" > /tmp/workspace/approval-ticket-id
echo "Saved ticket id: "$ticket_id" in /tmp/workspace/approval-ticket-id"
echo "Approval completed exiting with success."