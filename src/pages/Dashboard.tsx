import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { Transaction } from '../types/transaction';
import { Category } from '../types/category';
import { fetchTransactions, api } from '../utils/api';

function Dashboard() {
  const { currentUser } = useAuth();
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const loadData = async () => {
      if (!currentUser) return;
      
      setLoading(true);
      try {
        // Fetch categories for current user
        const categoriesResponse = await api.get('/categories', {
          params: { userId: currentUser.uid }
        });
        setCategories(categoriesResponse.data || []);

        // Fetch transactions 
        const transactionsData = await fetchTransactions();
        setTransactions(transactionsData);
        setError(null);
      } catch (err) {
        console.error('Failed to load data:', err);
        setError('Failed to load data. Please try again later.');
      } finally {
        setLoading(false);
      }
    };
    
    loadData();
  }, [currentUser]);

  if (!currentUser) {
    return <div className="text-center py-10">Loading...</div>;
  }

  return (
    <>
      <div className="mb-8">
        <h2 className="text-2xl font-bold text-gray-900">Welcome, {currentUser.displayName || currentUser.email?.split('@')[0] || 'User'}!</h2>
        <p className="text-gray-600">{new Date().toLocaleDateString('en-US', { weekday: 'long', year: 'numeric', month: 'long', day: 'numeric' })}</p>
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

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
        <div className="bg-white overflow-hidden shadow rounded-lg">
          <div className="px-4 py-5 sm:p-6">
            <dt className="text-sm font-medium text-gray-500 truncate">Total Transactions</dt>
            <dd className="mt-1 text-3xl font-semibold text-gray-900">
              {loading ? '...' : transactions.length}
            </dd>
          </div>
        </div>
        <div className="bg-white overflow-hidden shadow rounded-lg">
          <div className="px-4 py-5 sm:p-6">
            <dt className="text-sm font-medium text-gray-500 truncate">Categories</dt>
            <dd className="mt-1 text-3xl font-semibold text-gray-900">
              {loading ? '...' : categories.length}
            </dd>
          </div>
        </div>
        <div className="bg-white overflow-hidden shadow rounded-lg">
          <div className="px-4 py-5 sm:p-6">
            <dt className="text-sm font-medium text-gray-500 truncate">Account Status</dt>
            <dd className="mt-1 text-3xl font-semibold text-gray-900">
              Active
            </dd>
          </div>
        </div>
      </div>

      <div className="bg-white shadow rounded-lg mb-8">
        <div className="px-4 py-5 border-b border-gray-200 sm:px-6">
          <h3 className="text-lg leading-6 font-medium text-gray-900">
            Recent Transactions
          </h3>
        </div>
        {loading ? (
          <div className="text-center py-4">Loading transactions...</div>
        ) : transactions.length > 0 ? (
          <ul className="divide-y divide-gray-200">
            {transactions.slice(0, 5).map((transaction) => (
              <li key={transaction.id} className="px-4 py-4 sm:px-6">
                <div className="flex items-center justify-between">
                  <div className="flex items-center">
                    <div className="flex-shrink-0">
                      <div className="h-10 w-10 rounded-full bg-indigo-100 flex items-center justify-center">
                        <span className="text-indigo-600">${transaction.amount}</span>
                      </div>
                    </div>
                    <div className="ml-4">
                      <div className="text-sm font-medium text-gray-900">{transaction.note}</div>
                      <div className="text-sm text-gray-500">
                        {new Date(transaction.entered).toLocaleDateString()}
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center">
                    <span className="px-2 inline-flex text-xs leading-5 font-semibold rounded-full bg-green-100 text-green-800">
                      {transaction.category}
                    </span>
                  </div>
                </div>
              </li>
            ))}
          </ul>
        ) : (
          <div className="text-center py-4">No transactions found.</div>
        )}
        {transactions.length > 5 && (
          <div className="border-t border-gray-200 px-4 py-4 sm:px-6">
            <Link
              to="/transactions"
              className="text-sm font-medium text-indigo-600 hover:text-indigo-500"
            >
              View all transactions
            </Link>
          </div>
        )}
      </div>
    </>
  );
}

export default Dashboard; 