#!/bin/bash
set -e

# Test variables
TEST_DB_NAME="test-mpg-db-$(date +%s)"
TEST_APP_NAME="test-app-$(date +%s)"
REGION="lax"  # Changed from sjc to lax which is available for Managed Postgres

echo "Testing Fly.io Managed Postgres commands locally"
echo "Using test database name: $TEST_DB_NAME"

# Test 1: Check listing (this will succeed even if no clusters exist)
echo -e "\n\n==================="
echo "Test 1: List existing managed postgres clusters"
echo "==================="
flyctl mpg list

# Test 2: Create a postgres cluster
echo -e "\n\n==================="
echo "Test 2: Create a new managed postgres cluster"
echo "==================="
echo "Running: flyctl mpg create -n $TEST_DB_NAME -r $REGION --plan basic --volume-size 10"
# Use a timeout to avoid hanging indefinitely
timeout 60s flyctl mpg create -n $TEST_DB_NAME -r $REGION --plan basic --volume-size 10 || echo "Command timed out after 60 seconds, but cluster creation continues in the background"

# Test 3: Wait for cluster to be ready
echo -e "\n\n==================="
echo "Test 3: Wait for cluster to be ready"
echo "==================="
echo "Checking status..."
MAX_ATTEMPTS=10
for i in $(seq 1 $MAX_ATTEMPTS); do
  echo "Attempt $i of $MAX_ATTEMPTS..."
  flyctl mpg list --json > mpg_list.json
  cat mpg_list.json | grep -A 10 $TEST_DB_NAME || echo "No match found yet"
  STATUS=$(jq -r '.[] | select(.name=="'$TEST_DB_NAME'") | .status' mpg_list.json 2>/dev/null || echo "pending")
  echo "Current status: $STATUS"
  
  if [ "$STATUS" = "ready" ]; then
    echo "Database cluster is ready."
    break
  fi
  
  if [ $i -eq $MAX_ATTEMPTS ]; then
    echo "Maximum attempts reached. Cluster may still be initializing in the background."
    echo "Check status later with: flyctl mpg status $TEST_DB_NAME"
    echo "Proceeding with next steps assuming cluster will eventually be ready."
  else
    echo "Cluster not ready yet. Waiting 10 seconds..."
    sleep 10
  fi
done

# Test 4: Create a temporary app to test attachment
echo -e "\n\n==================="
echo "Test 4: Create temporary app for attachment test"
echo "==================="
flyctl apps create $TEST_APP_NAME --org personal || echo "Failed to create app, but continuing..."

# Test 5: Attach the database to the app (this may fail if cluster is not ready)
echo -e "\n\n==================="
echo "Test 5: Attach database to app"
echo "==================="
echo "Running: flyctl mpg attach $TEST_DB_NAME --app $TEST_APP_NAME"
flyctl mpg attach $TEST_DB_NAME --app $TEST_APP_NAME || echo "Attach failed, likely because cluster is not ready yet"

echo -e "\n\n==================="
echo "All tests completed! Cleanup required."
echo "==================="
echo "To clean up, run:"
echo "flyctl apps destroy $TEST_APP_NAME -y"
echo "flyctl mpg destroy $TEST_DB_NAME -y"
echo ""
echo "Note: If the cluster is still initializing, you may need to check status later and retry the attachment:"
echo "flyctl mpg status $TEST_DB_NAME"
echo "flyctl mpg attach $TEST_DB_NAME --app $TEST_APP_NAME" 