# Setting Up YNAB Integration

This document explains how to set up YNAB API credentials for local development of BennWallet.

## About YNAB Categories

BennWallet now synchronizes categories directly from YNAB. Instead of managing categories within the app, all categories are pulled from your YNAB budget. This means:

- Categories in the transaction form will display in a hierarchical structure matching your YNAB category groups
- Any changes to categories should be made in YNAB directly
- Categories will automatically sync daily

## Prerequisites

1. You need a YNAB account. If you don't have one, you can sign up at [YNAB.com](https://www.ynab.com/).
2. You need at least one budget and one account in YNAB.

## Getting Your YNAB API Credentials

### 1. Get Personal Access Token

1. Log in to your YNAB account at [app.youneedabudget.com](https://app.youneedabudget.com/)
2. Go to Account Settings → Developer Settings
3. Click "New Token" and give it a name like "BennWallet"
4. Copy the generated token (this is your `YNAB_TOKEN_USER_x` value)

### 2. Get Budget ID

1. While logged into YNAB, look at the URL when viewing your budget:
   - The URL will look like: `https://app.youneedabudget.com/BUDGET_ID/budget`
   - Copy the `BUDGET_ID` portion (a long string of letters and numbers)
   - This is your `YNAB_BUDGET_ID_USER_x` value

### 3. Get Account ID

1. To find your Account ID, we'll use the YNAB API:
   - Go to the [YNAB API Swagger Documentation](https://api.youneedabudget.com/#/Accounts/getAccounts)
   - Click "Try It Out"
   - Enter your Budget ID in the budget_id field
   - Enter your Personal Access Token in the "Authorize" dialog
   - Execute the request
   - In the response, find the account you want to use and copy its `id` value
   - This is your `YNAB_ACCOUNT_ID_USER_x` value

## Setting Up Local Environment

1. Run the setup script which creates a template `.env` file:
   ```bash
   ./setup_dev.sh
   ```

2. Edit the `.env` file and add your YNAB credentials:
   ```
   # User 1 (Sarah)
   YNAB_TOKEN_USER_1=your_token_here
   YNAB_BUDGET_ID_USER_1=your_budget_id_here
   YNAB_ACCOUNT_ID_USER_1=your_account_id_here
   
   # User 2 (Patrick)
   YNAB_TOKEN_USER_2=your_token_here
   YNAB_BUDGET_ID_USER_2=your_budget_id_here
   YNAB_ACCOUNT_ID_USER_2=your_account_id_here
   ```

3. Run the setup script again to complete the setup:
   ```bash
   ./setup_dev.sh
   ```

## Production Deployment (on Fly.io)

For production deployment, you'll need to set these secrets in Fly.io:

```bash
fly secrets set YNAB_TOKEN_USER_1=your_token_here
fly secrets set YNAB_BUDGET_ID_USER_1=your_budget_id_here
fly secrets set YNAB_ACCOUNT_ID_USER_1=your_account_id_here

fly secrets set YNAB_TOKEN_USER_2=your_token_here
fly secrets set YNAB_BUDGET_ID_USER_2=your_budget_id_here
fly secrets set YNAB_ACCOUNT_ID_USER_2=your_account_id_here
```

## Security Notes

- **NEVER commit your YNAB API token to version control**. The `.env` file is in `.gitignore` for this reason.
- Your YNAB personal access token provides full access to your YNAB account. Keep it secure.
- For production use, consider using a dedicated YNAB account or creating a separate budget for testing. 