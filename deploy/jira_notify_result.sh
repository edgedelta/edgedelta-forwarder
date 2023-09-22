#!/usr/bin/env bash
# This script is used in approval process with JIRA and Circle CI with below steps:
# - Reads ticket id
# - Updates ticket status as done and adds comment.
# Expects /tmp/workspace/approval-ticket-id to be filled as an attached workspace in circle ci:
#  - attach_workspace:
#    at: /tmp/workspace

# Environment Variables Required
# JIRA_TOKEN_OWNER: username of the owner of the jira token
# JIRA_API_TOKEN: jira api token
# SUCCESS: if set 1 notify success else notify failure.
set -e

# Constants
base_uri=https://${JIRA_URL}/rest/api/2/issue
project_key="AP"

# Jira transitions in workflow
# You can get transitions for an issue in a state as below
# curl \
# -u $jira_token_owner:$jira_api_token \
#   -X GET \
#   -H "Content-Type: application/json" \
#   $base_uri/$issue_id/transitions | jq
release_success_transition_id=21
release_fail_transition_id=41

if [ "$JIRA_TOKEN_OWNER" == "" ]; then
  echo "JIRA_TOKEN_OWNER is a required parameter. Exiting with failure."
  exit 14
fi

if [ "$JIRA_API_TOKEN" == "" ]; then
  echo "JIRA_API_TOKEN is a required parameter. Exiting with failure."
  exit 15
fi

ticket_id="$(cat /tmp/workspace/approval-ticket-id)"
if [ "$ticket_id" == "" ]; then
  echo "Unable to get ticket id. Exiting with failure."
  exit 1
fi

if [ "$SUCCESS" == "1" ];then
  echo "Notifying release success for ticket id: $ticket_id"

  # Mark ticket as success
  success_transition_body='{"transition":{"id":"'$release_success_transition_id'"}}'
  resp=$(curl \
    -u $JIRA_TOKEN_OWNER:$JIRA_API_TOKEN \
    -X POST \
    --data "$success_transition_body" \
    -H "Content-Type: application/json" \
    $base_uri/$ticket_id/transitions)
  echo "Update status response: $resp"
  if [[ $resp == *"error"* ]]; then
    echo "Update status failed. Exiting with failure."
    exit 2
  fi

  success_comment_body='{
    "update": {
        "comment": [
            {
                "add": {
                    "body": "Release completed with success.\nFinal Job Link: '$CIRCLE_BUILD_URL'"
                }
            }
        ]
    }
  }'

  resp=$(curl \
    -u $JIRA_TOKEN_OWNER:$JIRA_API_TOKEN \
    -X PUT \
    --data "$success_comment_body" \
    -H "Content-Type: application/json" \
    $base_uri/$ticket_id)
  echo "Add comment response: $resp"
  if [[ $resp == *"error"* ]]; then
    echo "Add comment failed. Exiting with failure."
    exit 3
  fi
else
  echo "Notifying release failure for ticket id: $ticket_id"
  # Mark ticket as fail
  fail_transition_body='{"transition":{"id":"'$release_fail_transition_id'"}}'
  resp=$(curl \
    -u $JIRA_TOKEN_OWNER:$JIRA_API_TOKEN \
    -X POST \
    --data "$fail_transition_body" \
    -H "Content-Type: application/json" \
    $base_uri/$ticket_id/transitions)
  echo "Update status response: $resp"
  if [[ $resp == *"error"* ]]; then
    echo "Update status failed. Exiting with failure."
    exit 2
  fi

  fail_comment_body='{
    "update": {
        "comment": [
            {
                "add": {
                    "body": "Release failed check Circle CI.\nFinal Job Link: '$CIRCLE_BUILD_URL'"
                }
            }
        ]
    }
  }'

  resp=$(curl \
    -u $JIRA_TOKEN_OWNER:$JIRA_API_TOKEN \
    -X PUT \
    --data "$fail_comment_body" \
    -H "Content-Type: application/json" \
    $base_uri/$ticket_id)
  echo "Add comment response: $resp"
  if [[ $resp == *"error"* ]]; then
    echo "Add comment failed. Exiting with failure."
    exit 3
  fi
fi
echo "Jira notification completed. Exiting with success."
exit 0