# Settlement Feature Guide

## Understanding the Settlement System

The settlement feature allows users to apply transactions against debts they owe each other. Here's how it works:

### Example Scenario

With our simplified test data, when you log in as Patrick Bennett:

**What I Owe:**

- Sarah Wallis: $400 total (Dinner $120 + Groceries $80 + Airbnb $200)
- Kim Donaldson: $255 total (Uber $60 + Lunch $45 + Concert $150)

**What Others Owe Me:**

- Sarah Wallis: $275 total (Gas $90 + Movies $110 + Breakfast $75)
- Kim Donaldson: $275 total (Hotel $140 + Dinner $85 + Supplies $50)

**Net Balance:**

- You owe Sarah: $125 ($400 - $275)
- Kim owes you: $20 ($275 - $255)

## How to Use Settlements

### Method 1: Direct Settlement (From Transactions Page)

1. Go to the Transactions page
2. Find a transaction where someone owes you money
3. Click the "Settle" button on that transaction
4. This creates a settlement and automatically applies that transaction

### Method 2: Settlement Manager (From Settlements Page)

1. Go to the Settlements page
2. Look at your Debt Summary
3. Click on a person's name to see details
4. Select transactions where YOU PAID and THEY OWE you
5. Click "Create Settlement"

### Key Concepts

1. **Settlements offset debts**: If Sarah owes you $275 but you owe her $400, you can create a settlement with your transactions (where she owes you) to reduce your net debt to $125.

2. **Only unpaid transactions can be settled**: Once a transaction is marked as paid or is part of a settlement, it can't be used again.

3. **Settlements can be cancelled**: Active settlements can be cancelled, which releases the transactions back to unpaid status.

4. **Visual indicators**: Transactions that are part of settlements show with a blue background and ⚖️ icon.

### Common Issues

**"No transactions available" in settlement dropdown:**

- This happens when there are no transactions where the other person owes money to offset against
- For example, if you owe Sarah $400 but she doesn't owe you anything, you can't create a settlement
- The test data ensures both directions of debt exist

**Can't see what you owe:**

- Make sure you're on the "What I Owe" view (default when logging in)
- The view toggle is at the top of the Transactions page

**Settlement not reducing debt:**

- Settlements don't automatically mark transactions as "paid"
- They create a record of which transactions offset each other
- The net balance shown on the Settlements page reflects the offset

### Test Data Summary

When you reset the database, you'll have:

- 3 users: Patrick Bennett (you), Sarah Wallis, Kim Donaldson
- Transactions in both directions between all users
- Clear examples of who owes whom
- Some historical paid transactions for reference
