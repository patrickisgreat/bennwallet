import { useState, useEffect } from 'react';
import AddTransactionForm from '../components/AddTransactionForm';
import TransactionTable from '../components/TransactionTable';
import { Transaction } from '../types/transaction';
import { fetchTransactions, updateTransaction, deleteTransaction } from '../utils/api';
import { useAuth } from '../context/AuthContext';
import { useUser } from '../context/UserContext';

// Define the filter interface
interface TransactionFilter {
  startDate: string;
  endDate: string;
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
  
  // Initialize filter with appropriate defaults based on user
  const [filter, setFilter] = useState<TransactionFilter>(() => {
    // Get saved filter from localStorage if available
    const savedFilter = localStorage.getItem('transactionFilter');
    if (savedFilter) {
      return JSON.parse(savedFilter);
    }
    
    // First day of current month
    const startDate = new Date(new Date().getFullYear(), new Date().getMonth(), 1).toISOString().split('T')[0];
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
      payTo: isPatrick ? 'Patrick' : 'Sarah',
      enteredBy: isPatrick ? 'Sarah' : 'Patrick',
      paid: false
    };
  });

  useEffect(() => {
    loadTransactions();
  }, []);
  
  useEffect(() => {
    // Save filter to localStorage whenever it changes
    localStorage.setItem('transactionFilter', JSON.stringify(filter));
    
    // Apply filter to transactions
    applyFilters();
  }, [filter, transactions]);

  const loadTransactions = async () => {
    if (!currentUser) return;
    
    setLoading(true);
    try {
      const data = await fetchTransactions();
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
  
  const applyFilters = () => {
    console.log('Raw transactions before filtering:', transactions);
    let filtered = [...transactions];
    
    // Filter by date range
    if (filter.startDate) {
      console.log(`Filtering by start date: ${filter.startDate}`);
      // For start date, set time to beginning of day
      const startDate = new Date(filter.startDate + 'T00:00:00');
      
      filtered = filtered.filter(tx => {
        const txDate = new Date(tx.entered);
        console.log(`Comparing transaction date ${tx.entered} (${txDate}) >= ${startDate}`);
        return txDate >= startDate;
      });
    }
    
    if (filter.endDate) {
      console.log(`Filtering by end date: ${filter.endDate}`);
      // For end date, set time to end of day
      const endDate = new Date(filter.endDate + 'T23:59:59');
      
      filtered = filtered.filter(tx => {
        const txDate = new Date(tx.entered);
        console.log(`Comparing transaction date ${tx.entered} (${txDate}) <= ${endDate}`);
        return txDate <= endDate;
      });
    }
    
    // Filter by payTo
    if (filter.payTo) {
      filtered = filtered.filter(tx => tx.payTo === filter.payTo);
    }
    
    // Filter by enteredBy
    if (filter.enteredBy) {
      filtered = filtered.filter(tx => tx.enteredBy === filter.enteredBy);
    }
    
    // Filter by paid status
    if (filter.paid !== undefined) {
      filtered = filtered.filter(tx => tx.paid === filter.paid);
    }

    console.log('Filtering transactions:', {
      total: transactions.length,
      filtered: filtered.length,
      filters: filter
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
        setTransactions(prev => 
          prev.map(tx => tx.id === id ? { ...tx, ...updates } : tx)
        );
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
        [name]: (e.target as HTMLInputElement).checked
      }));
    } else {
      setFilter(prev => ({ ...prev, [name]: value }));
    }
  };
  
  const clearFilters = () => {
    setFilter({
      startDate: '',
      endDate: '',
      payTo: '',
      enteredBy: '',
      paid: undefined
    });
  };
  
  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <h1 className="text-2xl font-bold">Transactions</h1>
        <button 
          onClick={loadTransactions}
          className="bg-indigo-100 text-indigo-700 px-3 py-1 rounded hover:bg-indigo-200"
        >
          Refresh
        </button>
      </div>
      
      {error && (
        <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4">
          {error}
          <button 
            className="float-right font-bold"
            onClick={() => setError(null)}
          >
            &times;
          </button>
        </div>
      )}
      
      {/* Filters */}
      <div className="bg-white p-4 rounded shadow mb-4">
        <div className="flex justify-between items-center mb-2">
          <h2 className="text-lg font-medium">Filters</h2>
          <button 
            onClick={clearFilters}
            className="text-sm text-indigo-600 hover:text-indigo-800"
          >
            Clear All
          </button>
        </div>
        
        <div className="grid grid-cols-1 md:grid-cols-5 gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Start Date</label>
            <input
              type="date"
              name="startDate"
              value={filter.startDate}
              onChange={handleFilterChange}
              className="block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
            />
          </div>
          
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">End Date</label>
            <input
              type="date"
              name="endDate"
              value={filter.endDate}
              onChange={handleFilterChange}
              className="block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
            />
          </div>
          
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
            </select>
          </div>
          
          <div className="flex items-center">
            <label className="flex items-center text-sm font-medium text-gray-700 mb-1 mt-4">
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
        />
      )}
    </div>
  );
}

export default TransactionsPage; 