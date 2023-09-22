#!/usr/bin/env bash
set -e

# This script depends on the following environment variables.
# export SLACK_ALERT_CHANNEL_WEBHOOK=""
# export SLACK_ALERT_STAGING_CHANNEL_WEBHOOK=""

# Usage: ./post_to_slack.sh "my message"

MESSAGE=$1
ENVIRONMENT=$2

if [[ $ENVIRONMENT == "staging" ]]; then
    curl -X POST -H 'Content-type: application/json' --data "{\"text\":\"$MESSAGE\"}" "$SLACK_ALERT_STAGING_CHANNEL_WEBHOOK"
else
    curl -X POST -H 'Content-type: application/json' --data "{\"text\":\"$MESSAGE\"}" "$SLACK_ALERT_CHANNEL_WEBHOOK"
fi