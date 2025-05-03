# Authentication and Security Setup for Benn Wallet

This document outlines the authentication and security setup for Benn Wallet.

## Overview

Benn Wallet uses Firebase Authentication to secure both the frontend and backend. This provides a robust, industry-standard authentication system with features like:

- Google OAuth login
- Email/password authentication
- Email verification
- Token-based API authentication
- Secure session management

## Setup Instructions

### 1. Firebase Configuration

1. Log in to the [Firebase Console](https://console.firebase.google.com/)
2. Select the project: `benwallett-ab39d`
3. Go to Project Settings > Service Accounts
4. Generate a new private key (this will download a JSON file)
5. Rename the downloaded file to `service-account.json` and place it in the `backend/` directory of the project
6. Make sure to keep this file secure and never commit it to version control

### 2. Environment Variable Configuration

For production environments (like Fly.io), set the following environment variables:

```bash
# Firebase Service Account credentials (Base64 encoded service-account.json)
FIREBASE_SERVICE_ACCOUNT=<base64-encoded-service-account-json>

# CORS allowed origins (comma-separated list)
CORS_ALLOWED_ORIGINS=https://bennwallet-prod.fly.dev,https://benwallett-ab39d.web.app

# Encryption key for sensitive data
ENCRYPTION_KEY=<32-character-random-string>
```

### 3. Testing the Authentication

1. Make sure the backend is running
2. Log in to the app using your Firebase credentials
3. Check the browser's network tab to confirm that API requests include the `Authorization: Bearer <token>` header
4. Verify that requests without a valid token are rejected with a 401 Unauthorized response

## Security Improvements

This implementation includes several security improvements:

1. **Token-based Authentication**: All API requests now require a valid Firebase JWT token
2. **CORS Protection**: The API only accepts requests from whitelisted origins
3. **Encryption**: Sensitive data is encrypted using AES-GCM
4. **Authorization**: User data is properly isolated based on the authenticated user
5. **Secure Headers**: Proper HTTP security headers are set

## Backward Compatibility

For a transition period, the backend will still accept the old authentication method using the `userId` query parameter. This will be removed in a future update once all clients are updated to use the new authentication method.

## Troubleshooting

- If you see 401 Unauthorized errors, check that your Firebase token is valid and not expired
- If you see CORS errors, make sure your domain is added to the allowed origins list
- For any Firebase initialization errors, check that the service account file is correctly formatted and contains valid credentials

For any further questions or issues, please contact the development team.
