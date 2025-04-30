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

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Contributors

- Patrick Bennette - Initial work
