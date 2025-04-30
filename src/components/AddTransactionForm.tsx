import { useState, useEffect } from 'react';
import { Transaction } from '../types/transaction';
import { Category } from '../types/category';
import { v4 as uuidv4 } from 'uuid';
import { useUser } from '../context/UserContext';
import { api, createTransaction } from '../utils/api';

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
  const [payTo, setPayTo] = useState<'Sarah' | 'Patrick'>('Sarah');
  const [amount, setAmount] = useState<string>('0.00');
  const [note, setNote] = useState('');
  const [category, setCategory] = useState('');
  const [optional, setOptional] = useState(false);
  const [transactionDate, setTransactionDate] = useState<string>(new Date().toISOString().split('T')[0]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const { currentUser } = useUser();

  useEffect(() => {
    if (currentUser) {
      loadCategories();
    }
  }, [currentUser]);

  const loadCategories = async () => {
    try {
      const response = await api.get('/categories', {
        params: { userId: currentUser?.id }
      });
      
      // Ensure we always set an array, even if API returns null or undefined
      if (Array.isArray(response.data)) {
        setCategories(response.data);
      } else {
        console.warn('Categories API did not return an array:', response.data);
        setCategories([]);
      }
    } catch (error) {
      console.error('Error loading categories:', error);
      setCategories([]);
    }
  };

  useEffect(() => {
    if (editingTransaction) {
      setPayTo(editingTransaction.payTo);
      setAmount(editingTransaction.amount.toFixed(2));
      setNote(editingTransaction.note);
      setCategory(editingTransaction.category);
      setOptional(editingTransaction.optional || false);
      setTransactionDate(new Date(editingTransaction.transactionDate).toISOString().split('T')[0]);
    } else {
      resetForm();
    }
  }, [editingTransaction]);

  const resetForm = () => {
    setPayTo('Sarah');
    setAmount('0.00');
    setNote('');
    setCategory('');
    setOptional(false);
    setTransactionDate(new Date().toISOString().split('T')[0]);
  };

  // Handle amount input change
  const handleAmountChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value;
    // Allow only valid numeric input with up to 2 decimal places
    if (value === '' || /^\d+(\.\d{0,2})?$/.test(value)) {
      setAmount(value);
    }
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

    setSubmitting(true);
    
    try {
      if (editingTransaction) {
        onEditSubmit(editingTransaction.id, {
          payTo,
          amount: parsedAmount,
          note,
          category,
          optional,
          transactionDate: new Date(transactionDate + 'T00:00:00').toISOString()
        });
      } else {
        const now = new Date();
        const newTransaction: Transaction = {
          id: uuidv4(),
          entered: now.toISOString(),
          transactionDate: new Date(transactionDate + 'T00:00:00').toISOString(),
          payTo,
          amount: parsedAmount,
          note,
          category,
          paid: false,
          enteredBy: currentUser.name,
          optional
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
    <form onSubmit={handleSubmit} className="bg-white p-4 rounded shadow mb-4">
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Pay To</label>
          <select
            value={payTo}
            onChange={(e) => setPayTo(e.target.value as 'Sarah' | 'Patrick')}
            className="mt-1 block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
          >
            <option value="Sarah">Sarah</option>
            <option value="Patrick">Patrick</option>
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
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Transaction Date</label>
          <input
            type="date"
            value={transactionDate}
            onChange={(e) => setTransactionDate(e.target.value)}
            className="mt-1 block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
            required
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Category</label>
          <select
            value={category}
            onChange={(e) => setCategory(e.target.value)}
            className="mt-1 block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
            required
          >
            <option value="">Select a category</option>
            {Array.isArray(categories) ? categories.map((cat) => (
              <option key={cat.id} value={cat.name}>
                {cat.name}
              </option>
            )) : null}
          </select>
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Note</label>
          <input
            type="text"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            className="mt-1 block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
          />
        </div>
        <div className="md:col-span-2">
          <label className="flex items-center text-sm font-medium text-gray-700">
            <input
              type="checkbox"
              checked={optional}
              onChange={(e) => setOptional(e.target.checked)}
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
          disabled={submitting}
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
