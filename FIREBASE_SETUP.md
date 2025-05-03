# Firebase Authentication Setup for Benn Wallet

This guide explains how to securely set up Firebase authentication for Benn Wallet without committing any sensitive credentials to GitHub.

## Local Development Setup

During local development, you have two options:

### Option 1: Development Mode (No Authentication)

The application will automatically run in development mode without authentication checks if:

- The `service-account.json` file is missing
- The file contains placeholder values like "REPLACE_WITH_ACTUAL_PRIVATE_KEY"

This allows you to develop locally without needing to set up real Firebase credentials.

### Option 2: Real Firebase Authentication (Recommended)

For a more realistic development experience with actual authentication:

1. **Get Firebase Credentials:**

   - Go to the [Firebase Console](https://console.firebase.google.com/)
   - Select your project: "benwallett-ab39d"
   - Go to Project Settings > Service Accounts
   - Click "Generate new private key" to download a JSON file

2. **Set up the credentials:**

   - **Option A - Environment Variable (More Secure):**

     ```bash
     # Set the environment variable with the contents of the JSON file
     export FIREBASE_SERVICE_ACCOUNT='{"type":"service_account",...}'
     ```

   - **Option B - Local File (Less Secure):**
     Rename the downloaded file to `service-account.json` and place it in the `backend/` directory.
     **Important:** Add this file to your `.gitignore` to ensure it's never committed to the repository.

## Production Setup on Fly.io with GitHub Actions

For production environments, we need to securely make Firebase credentials available to the application.

### 1. Setting Up Fly.io Secrets

Fly.io provides a secure way to store and use secrets in your application:

```bash
# Base64 encode the service account JSON file first (helps with special characters)
cat path/to/service-account.json | base64 > service-account-base64.txt

# Set the secret on Fly.io (copy the content of the base64 file)
fly secrets set FIREBASE_SERVICE_ACCOUNT_BASE64="$(cat service-account-base64.txt)"
```

Make sure your application code can handle both base64 encoded and regular JSON formats.

### 2. GitHub Actions Integration

To make this work with GitHub Actions for continuous deployment:

1. **Store the Firebase credentials as a GitHub Secret:**

   - Go to your GitHub repository
   - Navigate to Settings > Secrets and Variables > Actions
   - Create a new repository secret named `FIREBASE_SERVICE_ACCOUNT_BASE64`
   - Paste the base64-encoded service account JSON from the previous step

2. **Update your GitHub Actions workflow file (`.github/workflows/deploy.yml`):**

```yaml
name: Deploy to Fly.io

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Fly.io CLI
        uses: superfly/flyctl-actions/setup-flyctl@master

      - name: Deploy to Fly.io
        run: flyctl deploy --remote-only
        env:
          FLY_API_TOKEN: ${{ secrets.FLY_API_TOKEN }}

      # Optional: Update Fly.io secrets during deployment
      # Only do this if you need to update the secret values
      - name: Update Firebase credentials in Fly.io
        run: |
          echo "${{ secrets.FIREBASE_SERVICE_ACCOUNT_BASE64 }}" > service-account-base64.txt
          flyctl secrets set FIREBASE_SERVICE_ACCOUNT_BASE64="$(cat service-account-base64.txt)"
          rm service-account-base64.txt  # Clean up
        env:
          FLY_API_TOKEN: ${{ secrets.FLY_API_TOKEN }}
```

### 3. Update Application Code to Handle Base64-encoded Secrets

Update the `InitializeFirebase` function in `middleware/auth.go` to handle base64-encoded credentials:

```go
// Add to imports if needed:
// "encoding/base64"

// In InitializeFirebase function:
// Check for base64-encoded Firebase credentials first
firebaseCredentialsBase64 := os.Getenv("FIREBASE_SERVICE_ACCOUNT_BASE64")
if firebaseCredentialsBase64 != "" {
    log.Println("Using base64-encoded Firebase credentials from environment")

    // Decode the base64 string
    credBytes, err := base64.StdEncoding.DecodeString(firebaseCredentialsBase64)
    if err != nil {
        log.Printf("Error decoding base64 Firebase credentials: %v", err)
        return err
    }

    // Process the decoded credentials as JSON
    // ...rest of the code to create a temp file and initialize Firebase...
}
```

## Security Best Practices

1. **Never commit credentials to your repository**
2. **Restrict service account permissions** to only what your application needs
3. **Rotate credentials periodically** for better security
4. **Use environment variables** over files whenever possible
5. **Implement IP allowlisting** in Firebase Console for service account usage
6. **Monitor your Firebase usage** for any unusual activity

## Verifying Setup

To verify your Firebase auth is working correctly:

1. Start the backend with credentials properly configured
2. You should see log messages confirming "Firebase Admin SDK initialized successfully"
3. Log in through the frontend and verify that API requests can access protected endpoints
