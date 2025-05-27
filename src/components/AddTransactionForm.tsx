import { useState, useEffect } from 'react';
import { Transaction } from '../types/transaction';
import { Category } from '../types/category';
import { v4 as uuidv4 } from 'uuid';
import { useUser } from '../context/UserContext';
import { createTransaction, fetchUniqueTransactionFields } from '../utils/api';
import HierarchicalCategorySelector from './HierarchicalCategorySelector';

interface AddTransactionFormProps {
  onAdd: (transaction: Transaction) => void;
  editingTransaction: Transaction | null;
  onEditSubmit: (id: string, updates: Partial<Transaction>) => void;
  cancelEdit: () => void;
}

function AddTransactionForm({
  onAdd,
  editingTransaction,
  onEditSubmit,
  cancelEdit,
}: AddTransactionFormProps) {
  const { currentUser } = useUser();

  // Set default paidBy - using string type instead of specific literals
  const [paidBy, setPaidBy] = useState<string>('');
  const [owedBy, setOwedBy] = useState<string>('');
  const [amount, setAmount] = useState<string>('0.00');
  const [note, setNote] = useState('');
  const [category, setCategory] = useState('');
  const [optional, setOptional] = useState(false);
  const [transactionDate, setTransactionDate] = useState<string>(
    editingTransaction?.transactionDate
      ? new Date(editingTransaction.transactionDate).toISOString().split('T')[0]
      : new Date().toISOString().split('T')[0]
  );
  const [submitting, setSubmitting] = useState(false);
  const [userOptions, setUserOptions] = useState<string[]>([]);

  // First define the resetForm function before it's used
  const resetForm = () => {
    // Set paidBy to current user's name when resetting the form (if available)
    setPaidBy(currentUser?.name || '');
    setOwedBy('');
    setAmount('0.00');
    setNote('');
    setCategory('');
    setOptional(false);
    setTransactionDate(new Date().toISOString().split('T')[0]);
  };

  // Load unique fields for user dropdowns
  const loadUniqueFields = async () => {
    try {
      const fields = await fetchUniqueTransactionFields();
      console.log('Loaded unique user fields:', fields);

      // Store the user options even if the array is empty
      setUserOptions(fields.payTo);

      // Only set a default paidBy if we're not already editing a transaction
      // and only if we have options available
      if (!editingTransaction && currentUser?.name && fields.payTo.length > 0) {
        // If current user is in the list, set it as the default
        if (fields.payTo.includes(currentUser.name)) {
          setPaidBy(currentUser.name);
        } else {
          // Otherwise, set the first available option
          setPaidBy(fields.payTo[0]);
        }
      }
    } catch (err) {
      console.error('Error loading unique fields:', err);
      // Don't set any fallback values, just show the empty state
      setUserOptions([]);
    }
  };

  useEffect(() => {
    if (currentUser) {
      loadUniqueFields();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentUser]);

  // Update paidBy when currentUser changes
  useEffect(() => {
    if (currentUser && !editingTransaction && userOptions.length > 0) {
      // Only update if we're not editing (to avoid overwriting edited values)
      // and if we have options loaded
      if (userOptions.includes(currentUser.name)) {
        setPaidBy(currentUser.name);
      }
    }
  }, [currentUser, editingTransaction, userOptions]);

  useEffect(() => {
    if (editingTransaction) {
      // Handle paidBy field - use paidBy or fall back to payTo
      setPaidBy(editingTransaction.paidBy || editingTransaction.payTo || '');

      // Handle owedBy field
      setOwedBy(editingTransaction.owedBy || '');

      setAmount(editingTransaction.amount ? editingTransaction.amount.toFixed(2) : '0.00');

      // Make sure we handle note correctly, falling back to empty string if it's undefined
      setNote(editingTransaction.note !== undefined ? editingTransaction.note : '');

      // Get the category from either the categories array or the legacy category field
      if (editingTransaction.categories && editingTransaction.categories.length > 0) {
        setCategory(editingTransaction.categories[0].name || '');
      } else {
        setCategory(editingTransaction.category || '');
      }

      setOptional(editingTransaction.optional || false);

      // Make sure we have a valid date
      if (editingTransaction.transactionDate) {
        try {
          setTransactionDate(
            new Date(editingTransaction.transactionDate).toISOString().split('T')[0]
          );
        } catch (e) {
          console.error('Invalid transaction date', e);
          setTransactionDate(new Date().toISOString().split('T')[0]);
        }
      } else {
        setTransactionDate(new Date().toISOString().split('T')[0]);
      }
    } else {
      resetForm();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [editingTransaction]);

  // Handle amount input change
  const handleAmountChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value;
    // Allow only valid numeric input with up to 2 decimal places
    if (value === '' || /^\d+(\.\d{0,2})?$/.test(value)) {
      setAmount(value);
    }
  };

  // Create a category object from the selected category name
  const createCategoryObject = (categoryName: string): Category => {
    return {
      id: '', // Will be determined by the backend based on category name
      name: categoryName,
      description: '', // Optional description
      color: '', // Optional color
      userId: currentUser?.id || '', // Use string ID as expected by backend
    };
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!currentUser || submitting) return;

    const parsedAmount = parseFloat(amount);

    // Validate amount
    if (isNaN(parsedAmount) || parsedAmount <= 0) {
      alert('Please enter a valid amount');
      return;
    }

    // Validate category
    if (!category) {
      alert('Please select a category');
      return;
    }

    // Validate transaction date
    if (!transactionDate) {
      alert('Please select a transaction date');
      return;
    }

    // Validate paidBy
    if (!paidBy) {
      if (userOptions.length === 0) {
        alert('No users available. Please contact your administrator to add users to the system.');
      } else {
        alert('Please select who paid');
      }
      return;
    }

    setSubmitting(true);

    try {
      if (editingTransaction) {
        // Create category object for the update
        const categoryObject = createCategoryObject(category);

        onEditSubmit(editingTransaction.id, {
          paidBy,
          owedBy,
          amount: parsedAmount,
          note,
          category, // Keep legacy category field for compatibility
          optional,
          transactionDate: new Date(transactionDate + 'T00:00:00').toISOString(),
          categories: [categoryObject], // Add categories array with the selected category
        });
      } else {
        // Create category object for the new transaction
        const categoryObject = createCategoryObject(category);

        const newTransaction: Transaction = {
          id: uuidv4(),
          entered: new Date().toISOString(), // Always use current timestamp for entered date
          transactionDate: new Date(transactionDate + 'T00:00:00').toISOString(),
          paidBy: paidBy || currentUser.name, // Default to current user
          owedBy: owedBy,
          amount: parsedAmount,
          note,
          category, // Keep legacy category field for compatibility
          categories: [categoryObject], // Add categories array with the selected category
          paid: false,
          enteredBy: currentUser.name,
          optional,
        };

        // First save to backend
        const success = await createTransaction(newTransaction);

        if (success) {
          // Then update UI
          onAdd(newTransaction);
          resetForm();
        } else {
          alert('Failed to add transaction. Please try again.');
        }
      }
    } catch (error) {
      console.error('Error handling transaction:', error);
      alert('An error occurred. Please try again.');
    } finally {
      setSubmitting(false);
    }
  };

  if (!currentUser) {
    return null;
  }

  return (
    <form onSubmit={handleSubmit} className="bg-white p-4 rounded shadow mb-6">
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Paid By</label>
          <select
            value={paidBy}
            onChange={e => setPaidBy(e.target.value)}
            className="mt-1 block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
            required
          >
            <option value="">Select recipient</option>
            {userOptions.map(name => (
              <option key={name} value={name}>
                {name}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Owed By</label>
          <select
            value={owedBy}
            onChange={e => setOwedBy(e.target.value)}
            className="mt-1 block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
          >
            <option value="">N/A - Not a debt</option>
            {userOptions.map(name => (
              <option key={name} value={name}>
                {name}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Amount ($)</label>
          <div className="relative mt-1 rounded-md shadow-sm">
            <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
              <span className="text-gray-500 sm:text-sm">$</span>
            </div>
            <input
              type="text"
              value={amount}
              onChange={handleAmountChange}
              onFocus={() => {
                if (amount === '0.00') setAmount('');
              }}
              onBlur={() => {
                if (amount === '') setAmount('0.00');
              }}
              className="block w-full rounded-md border border-gray-300 pl-7 pr-3 py-2 focus:border-indigo-500 focus:ring-indigo-500"
              placeholder="0.00"
              required
            />
          </div>
        </div>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <label htmlFor="transactionDate" className="block text-sm font-medium text-gray-700">
              Transaction Date
            </label>
            <input
              type="date"
              name="transactionDate"
              id="transactionDate"
              value={transactionDate}
              onChange={e => setTransactionDate(e.target.value)}
              className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
            />
          </div>
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Category</label>
          <HierarchicalCategorySelector
            value={category}
            onChange={setCategory}
            className="w-full"
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Note</label>
          <input
            type="text"
            value={note}
            onChange={e => setNote(e.target.value)}
            className="mt-1 block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
          />
        </div>
        <div className="md:col-span-2">
          <label className="flex items-center text-sm font-medium text-gray-700">
            <input
              type="checkbox"
              checked={optional}
              onChange={e => setOptional(e.target.checked)}
              className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded mr-2"
            />
            Mark as Optional Transaction
          </label>
        </div>
      </div>
      <div className="mt-4 flex gap-2">
        <button
          type="submit"
          className="bg-indigo-600 text-white px-4 py-2 rounded-md hover:bg-indigo-700 disabled:opacity-50"
          disabled={submitting || !owedBy}
        >
          {submitting ? 'Processing...' : editingTransaction ? 'Update' : 'Add'} Transaction
        </button>
        {editingTransaction && (
          <button
            type="button"
            onClick={cancelEdit}
            className="bg-gray-200 text-gray-700 px-4 py-2 rounded-md hover:bg-gray-300"
            disabled={submitting}
          >
            Cancel
          </button>
        )}
      </div>
    </form>
  );
}

export default AddTransactionForm;
