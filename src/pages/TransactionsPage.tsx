import { useState, useEffect } from 'react';
import AddTransactionForm from '../components/AddTransactionForm';
import TransactionTable from '../components/TransactionTable';
import { Transaction } from '../types/transaction';
import {
  fetchTransactions,
  updateTransaction,
  deleteTransaction,
  createTransaction,
  fetchUniqueTransactionFields,
} from '../utils/api';
import { useAuth } from '../context/AuthContext';
import { useUser } from '../context/UserContext';
import React from 'react';
import { v4 as uuidv4 } from 'uuid';

// Define the filter interface
export interface TransactionFilter {
  // Entry date filters
  startDate: string;
  endDate: string;
  // Transaction date filters
  txStartDate: string;
  txEndDate: string;
  payTo: string;
  enteredBy: string;
  paid?: boolean;
  paidStatus: string; // 'all', 'paid', 'unpaid'
}

function TransactionsPage() {
  const { currentUser } = useAuth();
  const { currentUser: user } = useUser();
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [filteredTransactions, setFilteredTransactions] = useState<Transaction[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [editingTransaction, setEditingTransaction] = useState<Transaction | null>(null);
  const [isUploading, setIsUploading] = useState(false);
  const fileInputRef = React.useRef<HTMLInputElement>(null);
  const [payToOptions, setPayToOptions] = useState<string[]>([]);
  const [enteredByOptions, setEnteredByOptions] = useState<string[]>([]);
  const [apiError, setApiError] = useState<string | null>(null);
  const [viewMode, setViewMode] = useState<'iOwe' | 'othersOwe' | 'all'>('iOwe');

  // Initialize filter with appropriate defaults based on user
  const [filter, setFilter] = useState<TransactionFilter>({
    startDate: '',
    endDate: '',
    txStartDate: '',
    txEndDate: '',
    payTo: '',
    enteredBy: '',
    paid: undefined,
    paidStatus: 'all',
  });

  // Sorting state
  const [sortColumn, setSortColumn] = useState<keyof Transaction>('transactionDate');
  const [sortDirection, setSortDirection] = useState<'asc' | 'desc'>('desc');

  // Function to sort transactions
  const sortTransactions = (
    data: Transaction[],
    column: keyof Transaction,
    direction: 'asc' | 'desc'
  ): Transaction[] => {
    return [...data].sort((a, b) => {
      const aValue = a[column];
      const bValue = b[column];

      // Special handling for dates
      if (column === 'transactionDate' || column === 'entered' || column === 'paidDate') {
        // Safely convert to Date objects or timestamps
        const aDate = aValue ? new Date(aValue as string).getTime() : 0;
        const bDate = bValue ? new Date(bValue as string).getTime() : 0;

        return direction === 'asc' ? aDate - bDate : bDate - aDate;
      }

      if (aValue === bValue) return 0;
      if (aValue === null || aValue === undefined) return 1;
      if (bValue === null || bValue === undefined) return -1;

      if (direction === 'asc') {
        return aValue < bValue ? -1 : 1;
      } else {
        return aValue > bValue ? -1 : 1;
      }
    });
  };

  useEffect(() => {
    loadTransactions();
    loadUniqueFields();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Reload transactions when viewMode changes
  useEffect(() => {
    if (user?.name) {
      // Only reload if user is loaded
      loadTransactions();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [viewMode, user?.name]);

  // Apply viewMode filtering to existing transactions when viewMode changes
  useEffect(() => {
    if (transactions.length > 0 && user?.name) {
      let filtered = [...transactions];

      // Apply viewMode filtering
      if (viewMode === 'othersOwe') {
        filtered = transactions.filter(tx => tx.owedBy !== user.name);
      }

      // Apply sorting
      const sorted = sortTransactions(filtered, sortColumn, sortDirection);
      setFilteredTransactions(sorted);
    }
  }, [viewMode, user?.name, transactions, sortColumn, sortDirection]);

  // Load unique fields for dropdowns and update default filter if needed
  const loadUniqueFields = async () => {
    try {
      setApiError(null);
      const fields = await fetchUniqueTransactionFields();
      console.log('Loaded unique transaction fields:', fields);
      setPayToOptions(fields.payTo);
      setEnteredByOptions(fields.enteredBy);

      // If we have the user's name and filter fields are empty, set default filters
      if (user?.name && (!filter.payTo || !filter.enteredBy)) {
        // Update filter with user-specific defaults if we have user info
        // If the current user's name is in the list, use it
        const userName = user.name;
        let otherUsers: string[] = [];

        if (fields.payTo.includes(userName)) {
          // Find other users - prefer to show transactions from others
          otherUsers = fields.payTo.filter(name => name !== userName);
        }

        // Set default filters based on user context
        setFilter(prev => ({
          ...prev,
          payTo: prev.payTo || userName, // Default to current user if not set
          // Default enteredBy to another user if available, otherwise empty
          enteredBy: prev.enteredBy || (otherUsers.length > 0 ? otherUsers[0] : ''),
        }));
      }
    } catch (err) {
      console.error('Error loading unique transaction fields:', err);
      setApiError('Failed to load user options. Please try refreshing the page.');
    }
  };

  useEffect(() => {
    // Save filter to localStorage whenever it changes
    localStorage.setItem('transactionFilter', JSON.stringify(filter));

    // When filter changes, reload transactions from backend
    loadTransactions();

    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filter, viewMode, user?.name]);

  const loadTransactions = async () => {
    if (!currentUser) return;

    setLoading(true);
    setApiError(null);

    try {
      // Create API filter parameters
      const filterParams: Record<string, string | boolean | undefined> = {
        startDate: filter.startDate || undefined,
        endDate: filter.endDate || undefined,
        txStartDate: filter.txStartDate || undefined,
        txEndDate: filter.txEndDate || undefined,
        paid:
          filter.paidStatus === 'paid' ? true : filter.paidStatus === 'unpaid' ? false : undefined,
      };

      // Apply view mode filtering
      if (viewMode === 'iOwe' && user?.name) {
        filterParams.owedBy = user.name;
        filterParams.enteredBy = filter.enteredBy || undefined;
      } else if (viewMode === 'othersOwe' && user?.name) {
        filterParams.paidBy = user.name;
        // Don't include enteredBy filter in othersOwe mode to avoid conflicts
        // We'll filter out transactions where owedBy = current user in the frontend
      } else {
        // For 'all' mode or when not in specific debt view modes
        filterParams.enteredBy = filter.enteredBy || undefined;
        if (filter.payTo) {
          // For backward compatibility and the filter dropdown
          filterParams.payTo = filter.payTo;
        }
      }

      // Only include parameters with values
      const apiParams: Record<string, string | boolean | undefined> = {};
      Object.entries(filterParams).forEach(([key, value]) => {
        if (value !== undefined && value !== '') {
          apiParams[key] = value;
        }
      });

      console.log('Fetching transactions with API params:', apiParams);
      const data = await fetchTransactions(apiParams);

      // Log categories for debugging
      if (data && data.length > 0) {
        console.log(`Received ${data.length} transactions from API`);
        data.forEach(tx => {
          if (tx.categories && tx.categories.length > 0) {
            console.log(
              `Transaction ${tx.id} has ${tx.categories.length} categories:`,
              tx.categories.map(cat => cat.name).join(', ')
            );
          } else if (tx.category) {
            console.log(`Transaction ${tx.id} has legacy category: ${tx.category}`);
          } else {
            console.log(`Transaction ${tx.id} has no categories`);
          }
        });
      }

      // Update state with the fetched data
      let finalData = data;

      // Additional frontend filtering for 'othersOwe' mode
      if (viewMode === 'othersOwe' && user?.name) {
        finalData = data.filter(tx => tx.owedBy !== user.name);
      }

      setTransactions(finalData);

      // Apply sorting
      const sorted = sortTransactions(finalData, sortColumn, sortDirection);
      setFilteredTransactions(sorted);

      setError(null);
    } catch (err) {
      console.error('Error loading transactions:', err);
      setApiError(
        'Failed to load transactions. The server returned an error (500). Please contact your administrator.'
      );
      // Don't clear transactions on error
    } finally {
      setLoading(false);
    }
  };

  const handleAddTransaction = async (transaction: Transaction) => {
    setTransactions(prev => [transaction, ...prev]);
    await loadTransactions(); // Reload to get fresh data
  };

  const handleUpdateTransaction = async (id: string, updates: Partial<Transaction>) => {
    try {
      const success = await updateTransaction(id, updates);
      if (success) {
        const updatedTransactions = transactions.map(tx =>
          tx.id === id ? { ...tx, ...updates } : tx
        );
        setTransactions(updatedTransactions);

        // Also update the filtered transactions to ensure the UI reflects the change
        setFilteredTransactions(sortTransactions(updatedTransactions, sortColumn, sortDirection));

        // Remove forced refresh to prevent notes from reverting
        // Local state updates above should be sufficient
      } else {
        setError('Failed to update transaction');
      }
    } catch (err) {
      console.error('Error updating transaction:', err);
      setError('Failed to update transaction. Please try again.');
    }
  };

  const handleDeleteTransaction = async (id: string) => {
    if (!window.confirm('Are you sure you want to delete this transaction?')) {
      return;
    }

    try {
      const success = await deleteTransaction(id);
      if (success) {
        setTransactions(prev => prev.filter(tx => tx.id !== id));
        setFilteredTransactions(prev => prev.filter(tx => tx.id !== id));
      } else {
        setError('Failed to delete transaction');
      }
    } catch (err) {
      console.error('Error deleting transaction:', err);
      setError('Failed to delete transaction. Please try again.');
    }
  };

  const handleEditTransaction = (id: string) => {
    const transaction = transactions.find(tx => tx.id === id);
    if (transaction) {
      setEditingTransaction(transaction);
    }
  };

  const handleEditSubmit = async (id: string, updates: Partial<Transaction>) => {
    try {
      console.log('Submitting edit with updates:', updates);
      // First update the transaction
      const success = await updateTransaction(id, updates);

      if (success) {
        console.log('Update successful, updating local state with:', updates);
        // Update local state for immediate UI feedback
        const updatedTransactions = transactions.map(tx =>
          tx.id === id ? { ...tx, ...updates } : tx
        );
        setTransactions(updatedTransactions);
        setFilteredTransactions(sortTransactions(updatedTransactions, sortColumn, sortDirection));

        // Clear editing state
        setEditingTransaction(null);
      } else {
        setError('Failed to update transaction');
      }
    } catch (err) {
      console.error('Error in handleEditSubmit:', err);
      setError('Failed to update transaction. Please try again.');
    }
  };

  const handleCancelEdit = () => {
    setEditingTransaction(null);
  };

  const handleFilterChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value } = e.target;

    if (name === 'paid') {
      // Handle checkbox for paid status
      setFilter(prev => ({
        ...prev,
        [name]: (e.target as HTMLInputElement).checked,
      }));
    } else {
      console.log(`Changing filter ${name} to ${value}`);
      setFilter(prev => {
        const newFilter = { ...prev, [name]: value };

        // If both payTo and enteredBy are set to "All" (empty string), reset view mode
        if (name === 'payTo' || name === 'enteredBy') {
          const payToValue = name === 'payTo' ? value : prev.payTo;
          const enteredByValue = name === 'enteredBy' ? value : prev.enteredBy;

          if (payToValue === '' && enteredByValue === '') {
            setViewMode('all');
          }
        }

        return newFilter;
      });
    }
  };

  const clearFilters = () => {
    setFilter({
      startDate: '',
      endDate: '',
      txStartDate: '',
      txEndDate: '',
      payTo: '',
      enteredBy: '',
      paid: undefined,
      paidStatus: 'all',
    });
    setViewMode('all');
  };

  const handleBulkDelete = async (ids: string[]) => {
    setLoading(true);
    let failedCount = 0;

    for (const id of ids) {
      try {
        const success = await deleteTransaction(id);
        if (!success) {
          failedCount++;
        }
      } catch (err) {
        console.error(`Error deleting transaction ${id}:`, err);
        failedCount++;
      }
    }

    if (failedCount > 0) {
      setError(`Failed to delete ${failedCount} transaction(s). Please try again.`);
    }

    await loadTransactions();
    setLoading(false);
  };

  const exportToCSV = () => {
    // Return if no transactions to export
    if (!filteredTransactions.length) {
      setError('No transactions to export');
      return;
    }

    try {
      // Define CSV headers
      const headers = [
        'ID',
        'Entry Date',
        'Transaction Date',
        'Pay To',
        'Amount',
        'Category',
        'Note',
        'Paid',
        'Paid Date',
        'Entered By',
        'Optional',
      ];

      // Format transactions for CSV
      const csvRows = filteredTransactions.map(tx => {
        // Format dates for better readability
        const enteredDate = new Date(tx.entered).toLocaleDateString();
        const txDate = new Date(tx.transactionDate).toLocaleDateString();
        const paidDate = tx.paidDate ? new Date(tx.paidDate).toLocaleDateString() : '';

        return [
          tx.id,
          enteredDate,
          txDate,
          tx.payTo,
          tx.amount.toFixed(2),
          tx.category,
          tx.note.replace(/,/g, ' '), // Replace commas in notes to avoid CSV issues
          tx.paid ? 'Yes' : 'No',
          paidDate,
          tx.enteredBy,
          tx.optional ? 'Yes' : 'No',
        ];
      });

      // Combine headers and rows
      const csvContent = [headers.join(','), ...csvRows.map(row => row.join(','))].join('\n');

      // Create a blob and download link
      const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');

      // Set filename with current date
      const today = new Date().toISOString().split('T')[0];
      const fileName = `transactions_${today}.csv`;

      // Set up and trigger download
      link.setAttribute('href', url);
      link.setAttribute('download', fileName);
      link.style.visibility = 'hidden';
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
    } catch (err) {
      console.error('Error exporting transactions to CSV:', err);
      setError('Failed to export transactions');
    }
  };

  const handleCSVUpload = () => {
    fileInputRef.current?.click();
  };

  const processCSVFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file || !currentUser) return;

    setIsUploading(true);

    try {
      const text = await file.text();
      const rows = text.split('\n');
      const headers = rows[0].split(',');

      // Get indices for each required field
      const dateIndex = headers.findIndex(h => h.toLowerCase().includes('date'));
      const amountIndex = headers.findIndex(h => h.toLowerCase().includes('amount'));
      const payToIndex = headers.findIndex(
        h => h.toLowerCase().includes('pay to') || h.toLowerCase().includes('payto')
      );
      const categoryIndex = headers.findIndex(h => h.toLowerCase().includes('category'));
      const noteIndex = headers.findIndex(
        h => h.toLowerCase().includes('note') || h.toLowerCase().includes('description')
      );
      const optionalIndex = headers.findIndex(h => h.toLowerCase().includes('optional'));

      // Validate required columns
      if (dateIndex === -1 || amountIndex === -1 || payToIndex === -1 || categoryIndex === -1) {
        setError('CSV file must contain date, amount, payTo, and category columns');
        setIsUploading(false);
        return;
      }

      let successCount = 0;
      let failedCount = 0;

      // Process each row (skip header)
      for (let i = 1; i < rows.length; i++) {
        const row = rows[i].trim();
        if (!row) continue; // Skip empty rows

        const values = row.split(',');

        if (values.length < Math.max(dateIndex, amountIndex, payToIndex, categoryIndex) + 1) {
          console.warn(`Row ${i} has insufficient columns, skipping`);
          failedCount++;
          continue;
        }

        // Extract values
        const enteredDateValue = values[dateIndex].trim();
        const amountValue = parseFloat(values[amountIndex].replace('$', '').trim());
        const payToValue = values[payToIndex].trim();
        const categoryValue = values[categoryIndex].trim();
        const noteValue = noteIndex !== -1 && values[noteIndex] ? values[noteIndex].trim() : '';
        const optionalValue =
          optionalIndex !== -1 && values[optionalIndex]
            ? values[optionalIndex].toLowerCase().trim() === 'true' ||
              values[optionalIndex].trim() === '1'
            : false;

        // Look for transaction date in separate column or use entered date
        const txDateIndex = headers.findIndex(
          h => h.toLowerCase().includes('transaction date') || h.toLowerCase().includes('tx date')
        );
        const transactionDateValue =
          txDateIndex !== -1 && values[txDateIndex] ? values[txDateIndex].trim() : enteredDateValue; // Default to entered date if tx date not found

        // Validate values
        if (
          !enteredDateValue ||
          isNaN(amountValue) ||
          amountValue <= 0 ||
          !payToValue ||
          !categoryValue
        ) {
          console.warn(`Row ${i} has invalid data, skipping`);
          failedCount++;
          continue;
        }

        // Create transaction
        try {
          const newTransaction: Transaction = {
            id: uuidv4(),
            entered: new Date(enteredDateValue).toISOString(),
            transactionDate: new Date(transactionDateValue).toISOString(),
            payTo: payToValue,
            amount: amountValue,
            note: noteValue,
            category: categoryValue,
            paid: false,
            enteredBy: user?.name || 'User',
            optional: optionalValue,
          };

          const success = await createTransaction(newTransaction);
          if (success) {
            successCount++;
          } else {
            failedCount++;
          }
        } catch (err) {
          console.error(`Error creating transaction from row ${i}:`, err);
          failedCount++;
        }
      }

      // Show results
      if (successCount > 0) {
        alert(
          `Successfully imported ${successCount} transaction(s).` +
            (failedCount > 0 ? ` Failed to import ${failedCount} transaction(s).` : '')
        );
        loadTransactions();
      } else if (failedCount > 0) {
        setError(`Failed to import ${failedCount} transaction(s). Please check the CSV format.`);
      } else {
        setError('No transactions found in the CSV file.');
      }
    } catch (err) {
      console.error('Error processing CSV file:', err);
      setError('Failed to process CSV file. Please check the format.');
    } finally {
      setIsUploading(false);
      // Reset the file input
      if (fileInputRef.current) {
        fileInputRef.current.value = '';
      }
    }
  };

  // Add a function to handle column sorting
  const handleSortChange = (column: keyof Transaction) => {
    if (column === sortColumn) {
      // Toggle direction if clicking the same column
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc');
    } else {
      // Set new column with default desc direction
      setSortColumn(column);
      setSortDirection('desc');
    }

    // Re-sort the filtered transactions
    setFilteredTransactions(
      sortTransactions(
        transactions,
        column === sortColumn && sortDirection === 'asc' ? column : column,
        column === sortColumn && sortDirection === 'asc' ? 'desc' : 'asc'
      )
    );
  };

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <h1 className="text-2xl font-bold">Transactions</h1>
        <div className="flex gap-2">
          <button
            onClick={exportToCSV}
            disabled={loading || filteredTransactions.length === 0}
            className="bg-blue-600 text-white px-3 py-1 rounded hover:bg-blue-700 disabled:bg-gray-400"
          >
            Export CSV
          </button>
          <button
            onClick={handleCSVUpload}
            disabled={isUploading}
            className="bg-green-600 text-white px-3 py-1 rounded hover:bg-green-700 disabled:bg-gray-400"
          >
            {isUploading ? 'Uploading...' : 'Import CSV'}
          </button>
          <input
            type="file"
            ref={fileInputRef}
            accept=".csv"
            onChange={processCSVFile}
            className="hidden"
          />
          <button
            onClick={loadTransactions}
            className="bg-indigo-100 text-indigo-700 px-3 py-1 rounded hover:bg-indigo-200"
          >
            Refresh
          </button>
        </div>
      </div>

      {error && (
        <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4">
          {error}
          <button className="float-right font-bold" onClick={() => setError(null)}>
            &times;
          </button>
        </div>
      )}

      {/* View Mode Toggle */}
      <div className="bg-white p-4 rounded shadow mb-4">
        <div className="flex justify-center gap-4">
          <button
            onClick={() => setViewMode('iOwe')}
            className={`px-4 py-2 rounded ${
              viewMode === 'iOwe'
                ? 'bg-indigo-600 text-white'
                : 'bg-gray-200 text-gray-700 hover:bg-gray-300'
            }`}
          >
            What I Owe
          </button>
          <button
            onClick={() => setViewMode('othersOwe')}
            className={`px-4 py-2 rounded ${
              viewMode === 'othersOwe'
                ? 'bg-indigo-600 text-white'
                : 'bg-gray-200 text-gray-700 hover:bg-gray-300'
            }`}
          >
            What Others Owe Me
          </button>
          {user?.isAdmin && (
            <button
              onClick={() => setViewMode('all')}
              className={`px-4 py-2 rounded ${
                viewMode === 'all'
                  ? 'bg-indigo-600 text-white'
                  : 'bg-gray-200 text-gray-700 hover:bg-gray-300'
              }`}
            >
              All Transactions
            </button>
          )}
        </div>
      </div>

      {/* Filters */}
      <div className="bg-white p-4 rounded shadow mb-4">
        <div className="flex justify-between items-center mb-2">
          <h2 className="text-lg font-medium">Filters</h2>
          <button onClick={clearFilters} className="text-sm text-indigo-600 hover:text-indigo-800">
            Clear All
          </button>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
          <div>
            <h3 className="font-medium text-gray-700 mb-2">Entry Date</h3>
            <div className="grid grid-cols-2 gap-2">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Start</label>
                <input
                  type="date"
                  name="startDate"
                  value={filter.startDate}
                  onChange={handleFilterChange}
                  className="block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">End</label>
                <input
                  type="date"
                  name="endDate"
                  value={filter.endDate}
                  onChange={handleFilterChange}
                  className="block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
                />
              </div>
            </div>
          </div>

          <div>
            <h3 className="font-medium text-gray-700 mb-2">Transaction Date</h3>
            <div className="grid grid-cols-2 gap-2">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Start</label>
                <input
                  type="date"
                  name="txStartDate"
                  value={filter.txStartDate}
                  onChange={handleFilterChange}
                  className="block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">End</label>
                <input
                  type="date"
                  name="txEndDate"
                  value={filter.txEndDate}
                  onChange={handleFilterChange}
                  className="block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
                />
              </div>
            </div>
          </div>

          <div>
            <h3 className="font-medium text-gray-700 mb-2">Other Filters</h3>
            <div className="grid grid-cols-2 gap-2">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Owed By</label>
                <select
                  name="payTo"
                  value={filter.payTo}
                  onChange={handleFilterChange}
                  className="block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
                >
                  <option value="">All</option>
                  {payToOptions.map(name => (
                    <option key={name} value={name}>
                      {name}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Owed To</label>
                <select
                  name="enteredBy"
                  value={filter.enteredBy}
                  onChange={handleFilterChange}
                  className="block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
                >
                  <option value="">All</option>
                  {enteredByOptions.map(name => (
                    <option key={name} value={name}>
                      {name}
                    </option>
                  ))}
                </select>
              </div>
            </div>
          </div>
        </div>

        <div className="flex items-center">
          <label className="flex items-center text-sm font-medium text-gray-700">
            <input
              type="checkbox"
              name="paid"
              checked={filter.paid ?? false}
              onChange={handleFilterChange}
              className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded mr-2"
            />
            Only Show Unpaid
          </label>
        </div>
      </div>

      {/* Display API error messages */}
      {apiError && (
        <div
          className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-6"
          role="alert"
        >
          <p>{apiError}</p>
        </div>
      )}

      {/* Empty state message when no options are available */}
      {payToOptions.length === 0 && enteredByOptions.length === 0 && !apiError && (
        <div
          className="bg-yellow-100 border border-yellow-400 text-yellow-700 px-4 py-3 rounded mb-6"
          role="alert"
        >
          <p>No transaction data found. You may need to add some transactions first.</p>
        </div>
      )}

      <AddTransactionForm
        onAdd={handleAddTransaction}
        editingTransaction={editingTransaction}
        onEditSubmit={handleEditSubmit}
        cancelEdit={handleCancelEdit}
      />

      {loading ? (
        <div className="text-center py-4">Loading transactions...</div>
      ) : (
        <TransactionTable
          transactions={filteredTransactions}
          onUpdate={handleUpdateTransaction}
          onDelete={handleDeleteTransaction}
          onEdit={handleEditTransaction}
          onBulkDelete={handleBulkDelete}
          onSortChange={handleSortChange}
        />
      )}
    </div>
  );
}

export default TransactionsPage;
