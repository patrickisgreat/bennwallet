#!/bin/bash
# deploy-pr.sh - Deploy to a PR environment with the correct configuration
# Usage: ./deploy-pr.sh <PR_NUMBER>

set -e

if [ -z "$1" ]; then
  echo "Error: PR number is required"
  echo "Usage: ./deploy-pr.sh <PR_NUMBER>"
  exit 1
fi

PR_NUMBER=$1
APP_NAME="bennwallet-pr-${PR_NUMBER}"
VOLUME_NAME="bennwalletpr${PR_NUMBER}_data"
CONFIG_FILE="pr${PR_NUMBER}.toml"

echo "Generating configuration for PR ${PR_NUMBER}..."
cat > ${CONFIG_FILE} << EOF
############################################
# Fly.io configuration for PR-${PR_NUMBER}
############################################
app            = "${APP_NAME}"
primary_region = "sjc"

[build]
  dockerfile = "Dockerfile"

[env]
  PORT = "8080"
  NODE_ENV = "development"
  APP_ENV = "development"
  PR_DEPLOYMENT = "true"
  RESET_DB = "true"
  PR_NUMBER = "${PR_NUMBER}"

[http_service]
  internal_port        = 8080
  force_https          = true
  auto_start_machines  = true
  auto_stop_machines   = true
  min_machines_running = 0
  processes            = ["app"]

  [[http_service.checks]]
    method        = "GET"
    path          = "/health"
    protocol      = "http"
    interval      = "30s"
    timeout       = "5s"
    grace_period  = "10s"

[[statics]]
  guest_path = "/app/dist"
  url_prefix = "/"

[[mounts]]
  source      = "${VOLUME_NAME}"
  destination = "/data"

  [mounts.options]
    size = "1"
EOF

echo "Checking if app ${APP_NAME} exists..."
if ! flyctl apps list | grep -q "${APP_NAME}"; then
  echo "Creating new app: ${APP_NAME}"
  flyctl apps create "${APP_NAME}" --org personal
else
  echo "App ${APP_NAME} already exists"
fi

echo "Checking if volume ${VOLUME_NAME} exists..."
if ! flyctl volumes list --app "${APP_NAME}" | grep -q "${VOLUME_NAME}"; then
  echo "Creating volume ${VOLUME_NAME}..."
  flyctl volumes create "${VOLUME_NAME}" --app "${APP_NAME}" --region sjc --size 1
else
  echo "Volume ${VOLUME_NAME} already exists"
fi

echo "Deploying to PR-${PR_NUMBER} environment..."
flyctl deploy --config "${CONFIG_FILE}" --app "${APP_NAME}" --env PR_DEPLOYMENT=true --env RESET_DB=true

echo "Deployment complete!"
echo "Your PR environment is available at: https://${APP_NAME}.fly.dev"

# Clean up
echo "Cleaning up temporary files..."
rm ${CONFIG_FILE} 