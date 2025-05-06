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
interface TransactionFilter {
  // Entry date filters
  startDate: string;
  endDate: string;
  // Transaction date filters
  txStartDate: string;
  txEndDate: string;
  payTo: string;
  enteredBy: string;
  paid?: boolean;
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
  const [uniqueFields, setUniqueFields] = useState<{ payTo: string[]; enteredBy: string[] }>({
    payTo: [],
    enteredBy: [],
  });

  // Initialize filter with appropriate defaults based on user
  const [filter, setFilter] = useState<TransactionFilter>(() => {
    // Get saved filter from localStorage if available
    const savedFilter = localStorage.getItem('transactionFilter');
    if (savedFilter) {
      return JSON.parse(savedFilter);
    }

    // First day of current month
    const startDate = new Date(new Date().getFullYear(), new Date().getMonth(), 1)
      .toISOString()
      .split('T')[0];
    // Today
    const endDate = new Date().toISOString().split('T')[0];

    // Set default filter based on the user's role
    // If Patrick is logged in, show transactions entered by Sarah for Patrick to pay
    // If Sarah is logged in, show transactions entered by Patrick for Sarah to pay
    const userName = user?.name || '';
    const isPatrick = userName.toLowerCase().includes('patrick');

    return {
      startDate,
      endDate,
      txStartDate: '',
      txEndDate: '',
      payTo: isPatrick ? 'Patrick' : 'Sarah',
      enteredBy: isPatrick ? 'Sarah' : 'Patrick',
      paid: false,
    };
  });

  useEffect(() => {
    loadTransactions();
    loadUniqueFields();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    // Save filter to localStorage whenever it changes
    localStorage.setItem('transactionFilter', JSON.stringify(filter));

    // When filter changes, reload transactions from backend
    loadTransactions();

    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filter]);

  const loadTransactions = async () => {
    if (!currentUser) return;

    setLoading(true);
    try {
      // Create API filter parameters from the current filter
      const filterParams = {
        startDate: filter.startDate || undefined,
        endDate: filter.endDate || undefined,
        txStartDate: filter.txStartDate || undefined,
        txEndDate: filter.txEndDate || undefined,
        payTo: filter.payTo || undefined,
        enteredBy: filter.enteredBy || undefined,
        paid: filter.paid,
      };

      // Only include parameters that have values - EXCLUDE empty strings
      const apiParams: Record<string, string | boolean | undefined> = {};
      Object.entries(filterParams).forEach(([key, value]) => {
        if (value !== undefined && value !== '') {
          apiParams[key] = value;
        }
      });

      console.log('Fetching transactions with API params:', apiParams);
      const data = await fetchTransactions(apiParams);
      setTransactions(data);
      applyFilters();
      setError(null);
    } catch (err) {
      console.error('Error loading transactions:', err);
      setError('Failed to load transactions. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  const loadUniqueFields = async () => {
    try {
      const fields = await fetchUniqueTransactionFields();
      console.log('Loaded unique fields:', fields);
      setUniqueFields(fields);
    } catch (err) {
      console.error('Error loading unique transaction fields:', err);
    }
  };

  const applyFilters = () => {
    console.log('Raw transactions before filtering:', transactions);
    let filtered = [...transactions];

    // Filter by entered date range
    if (filter.startDate) {
      console.log(`Filtering by entered start date: ${filter.startDate}`);
      // For start date, set time to beginning of day
      const startDate = new Date(filter.startDate + 'T00:00:00');

      filtered = filtered.filter(tx => {
        const txDate = new Date(tx.entered);
        console.log(`Comparing entered date ${tx.entered} (${txDate}) >= ${startDate}`);
        return txDate >= startDate;
      });
    }

    if (filter.endDate) {
      console.log(`Filtering by entered end date: ${filter.endDate}`);
      // For end date, set time to end of day
      const endDate = new Date(filter.endDate + 'T23:59:59');

      filtered = filtered.filter(tx => {
        const txDate = new Date(tx.entered);
        console.log(`Comparing entered date ${tx.entered} (${txDate}) <= ${endDate}`);
        return txDate <= endDate;
      });
    }

    // Filter by transaction date range
    if (filter.txStartDate) {
      console.log(`Filtering by transaction start date: ${filter.txStartDate}`);
      const txStartDate = new Date(filter.txStartDate + 'T00:00:00');

      filtered = filtered.filter(tx => {
        const txDate = new Date(tx.transactionDate);
        return txDate >= txStartDate;
      });
    }

    if (filter.txEndDate) {
      console.log(`Filtering by transaction end date: ${filter.txEndDate}`);
      const txEndDate = new Date(filter.txEndDate + 'T23:59:59');

      filtered = filtered.filter(tx => {
        const txDate = new Date(tx.transactionDate);
        return txDate <= txEndDate;
      });
    }

    // Helper function to match names with flexibility
    const matchesName = (value: string, searchTerm: string) => {
      if (!searchTerm) return true; // If no search term, match all
      if (!value) return false; // If no value but a search term, no match

      const valueLower = value.toLowerCase();
      const searchLower = searchTerm.toLowerCase();

      // Direct match
      if (valueLower === searchLower) return true;

      // Contains match
      if (valueLower.includes(searchLower)) return true;

      // Special case for "Sarah" and "Sarah Elizabeth Wallis"
      if (
        searchLower === 'sarah' &&
        (valueLower.includes('sarah') || valueLower === 'sarah elizabeth wallis')
      )
        return true;

      // Special case for "Patrick" and "Patrick Bennett"
      if (
        searchLower === 'patrick' &&
        (valueLower.includes('patrick') || valueLower === 'patrick bennett')
      )
        return true;

      return false;
    };

    // Filter by payTo
    if (filter.payTo) {
      filtered = filtered.filter(tx => matchesName(tx.payTo, filter.payTo));
    }

    // Filter by enteredBy
    if (filter.enteredBy) {
      filtered = filtered.filter(tx => matchesName(tx.enteredBy, filter.enteredBy));
    }

    // Filter by paid status
    if (filter.paid !== undefined) {
      filtered = filtered.filter(tx => tx.paid === filter.paid);
    }

    console.log('Filtering transactions:', {
      total: transactions.length,
      filtered: filtered.length,
      filters: filter,
    });

    setFilteredTransactions(filtered);
  };

  const handleAddTransaction = async (transaction: Transaction) => {
    setTransactions(prev => [transaction, ...prev]);
    await loadTransactions(); // Reload to get fresh data
  };

  const handleUpdateTransaction = async (id: string, updates: Partial<Transaction>) => {
    try {
      const success = await updateTransaction(id, updates);
      if (success) {
        setTransactions(prev => prev.map(tx => (tx.id === id ? { ...tx, ...updates } : tx)));
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
    await handleUpdateTransaction(id, updates);
    setEditingTransaction(null);
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
      setFilter(prev => ({ ...prev, [name]: value }));
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
      paid: false,
    });
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
        const payToValue = values[payToIndex].trim() as 'Sarah' | 'Patrick';
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
          !['Sarah', 'Patrick'].includes(payToValue) ||
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
                <label className="block text-sm font-medium text-gray-700 mb-1">Pay To</label>
                <select
                  name="payTo"
                  value={filter.payTo}
                  onChange={handleFilterChange}
                  className="block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
                >
                  <option value="">All</option>
                  <option value="Sarah">Sarah</option>
                  <option value="Patrick">Patrick</option>
                  {uniqueFields.payTo
                    .filter(name => name !== 'Sarah' && name !== 'Patrick')
                    .map(name => (
                      <option key={name} value={name}>
                        {name}
                      </option>
                    ))}
                </select>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Entered By</label>
                <select
                  name="enteredBy"
                  value={filter.enteredBy}
                  onChange={handleFilterChange}
                  className="block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
                >
                  <option value="">All</option>
                  <option value="Sarah">Sarah</option>
                  <option value="Patrick">Patrick</option>
                  {uniqueFields.enteredBy
                    .filter(name => name !== 'Sarah' && name !== 'Patrick')
                    .map(name => (
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
        />
      )}
    </div>
  );
}

export default TransactionsPage;
