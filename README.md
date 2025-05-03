# BennWallet

A personal finance application for tracking shared expenses and generating YNAB-compatible reports.

## Overview

BennWallet is a full-stack web application designed to help couples or roommates track their shared expenses, categorize transactions, and generate insightful reports. The application features:

- User authentication and account management
- Transaction tracking and categorization
- Filtering and reporting capabilities
- YNAB (You Need A Budget) integration for financial data analysis

## Technologies

### Frontend

- React (with TypeScript)
- Vite for fast development and optimized builds
- React Router for SPA navigation
- Tailwind CSS for styling

### Backend

- Go (Golang)
- Gorilla Mux for routing
- SQLite for database
- JWT for authentication

## Project Structure

```
bennwallet/
├── backend/               # Go backend code
│   ├── database/          # Database connection and models
│   ├── handlers/          # API endpoint handlers
│   ├── middleware/        # Request/response middleware
│   ├── models/            # Data models
│   ├── utils/             # Helper functions
│   └── main.go            # Entry point for the backend
├── src/                   # React frontend code
│   ├── components/        # Reusable React components
│   ├── context/           # React context providers
│   ├── pages/             # Page components
│   ├── utils/             # Helper functions
│   └── App.tsx            # Main application component
├── public/                # Static assets
└── dist/                  # Production build output
```

## Getting Started

### Prerequisites

- Go 1.16+
- Node.js 16+
- npm or yarn

### Development Setup

1. Clone the repository:

   ```
   git clone https://github.com/yourusername/bennwallet.git
   cd bennwallet
   ```

2. Install frontend dependencies:

   ```
   npm install
   ```

3. Build the backend:

   ```
   cd backend
   go build
   ```

4. Run the backend server:

   ```
   ./bennwallet
   ```

5. In a separate terminal, start the frontend development server:

   ```
   npm run dev
   ```

6. Open your browser and navigate to `http://localhost:5173`

### Development Workflow

When developing for BennWallet, the following checks run automatically before each commit:

1. **Frontend Checks**:

   - ESLint for code quality
   - Prettier for code formatting
   - TypeScript type checking
   - Unit tests

2. **Backend Checks**:
   - Go formatting (gofmt)
   - Go tests

These checks help maintain code quality and prevent bugs from being introduced. You can run these checks manually:

```bash
# Frontend checks
npm run lint        # Run ESLint
npm run check-types # Run TypeScript type checking
npm test           # Run unit tests

# Backend checks
cd backend
go fmt ./...       # Format Go code
go test ./...      # Run Go tests
```

### Building for Production

1. Build the frontend:

   ```
   npm run build
   ```

2. Build the backend:

   ```
   cd backend
   go build
   ```

3. Run the production server:
   ```
   ./bennwallet
   ```

## Features

### User Management

- User registration and login
- Password reset functionality
- Profile management

### Transactions

- Add, edit, and delete transactions
- Categorize transactions
- Mark transactions as paid/unpaid
- Filter transactions by date, category, or person

### Reports

- Generate YNAB-compatible reports
- View spending by category
- Filter reports by date range and other criteria

## Deployment

The application is configured for deployment to Fly.io. The `fly.toml` file contains the necessary configuration.

To deploy:

```
fly deploy
```

## Release Process

BennWallet uses semantic versioning for releases. When you merge a PR from the `dev` branch to `main`, the GitHub Actions workflow automatically:

1. Generates a new version based on commit messages
2. Creates a new tag
3. Generates a changelog
4. Creates a GitHub release

### Conventional Commits

To ensure proper versioning, use the following commit message format:

```
<type>(<scope>): <short summary>
```

Where `type` is one of:

- `feat`: A new feature (minor version bump)
- `fix`: A bug fix (patch version bump)
- `docs`: Documentation changes (patch version bump)
- `style`: Changes that don't affect code meaning (patch version bump)
- `refactor`: Code changes that neither fix bugs nor add features (patch version bump)
- `perf`: Performance improvements (patch version bump)
- `test`: Adding or updating tests (patch version bump)
- `build`: Changes to build system or dependencies (patch version bump)
- `ci`: Changes to CI/CD configuration (patch version bump)

For a major version bump, include `BREAKING CHANGE:` in your commit message body.

Example:

```
feat(ynab): add ability to sync with multiple accounts

This adds support for syncing with multiple YNAB accounts.
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Contributors

- Patrick Bennette - Initial work

## Firebase Authentication Setup for Deployment

For production deployments, Firebase authentication credentials need to be set up correctly:

### Setting up GitHub Actions Secrets

1. Go to your GitHub repository settings → Secrets and variables → Actions
2. Add the following secrets:
   - `FIREBASE_SERVICE_ACCOUNT_JSON`: The entire content of your `service-account.json` file
   - `FLY_API_TOKEN`: Your Fly.io API token for deployments

### Manual Setup for Fly.io

If you need to set up Firebase credentials manually on Fly.io:

```bash
# Export your service account JSON content to a file
cat service-account.json > /tmp/firebase-credentials.json

# Set as a secret in Fly.io
flyctl secrets set FIREBASE_SERVICE_ACCOUNT_JSON="$(cat /tmp/firebase-credentials.json)" -a your-app-name

# Clean up
rm /tmp/firebase-credentials.json
```

### Setting Up Multiple Environments

For each deployment environment, add the Firebase credentials as a secret:

```bash
# For production
flyctl secrets set FIREBASE_SERVICE_ACCOUNT_JSON="$(cat /tmp/firebase-credentials.json)" -a bennwallet-prod

# For staging
flyctl secrets set FIREBASE_SERVICE_ACCOUNT_JSON="$(cat /tmp/firebase-credentials.json)" -a bennwallet-staging
```

The GitHub Actions workflow will automatically set these secrets during deployment.
