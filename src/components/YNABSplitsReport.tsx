import { useState, useEffect } from 'react';
import { Category } from '../types/category';
import { fetchYNABSplits, syncToYNAB, CategoryTotal } from '../utils/api';
import { useUser } from '../context/UserContext';

interface YNABSplitsReportProps {
  categories: Category[];
  currentUser: string;
}

export default function YNABSplitsReport({ categories, currentUser }: YNABSplitsReportProps) {
  const { currentUser: user } = useUser();
  const [startDate, setStartDate] = useState<string>('');
  const [endDate, setEndDate] = useState<string>('');
  const [selectedCategory, setSelectedCategory] = useState<string>('');
  const [results, setResults] = useState<CategoryTotal[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [isSyncing, setIsSyncing] = useState(false);
  const [syncSuccess, setSyncSuccess] = useState<boolean | null>(null);
  const [error, setError] = useState<string | null>(null);

  const fetchReport = async () => {
    setIsLoading(true);
    setError(null);
    setSyncSuccess(null);
    try {
      // Validate inputs before sending
      if (startDate && endDate && new Date(startDate) > new Date(endDate)) {
        setError('Start date must be before end date');
        setIsLoading(false);
        return;
      }
      
      console.log('Fetching YNAB splits with params:', {
        startDate,
        endDate,
        category: selectedCategory,
        enteredBy: currentUser
      });
      
      // Use the fetchYNABSplits function from api.ts which now uses POST
      const data = await fetchYNABSplits({
        startDate,
        endDate,
        category: selectedCategory || undefined,
        enteredBy: currentUser
      });
      
      setResults(data);
    } catch (error) {
      console.error('Error fetching report:', error);
      setError('Failed to fetch report data');
      setResults([]);
    } finally {
      setIsLoading(false);
    }
  };

  const handleSyncToYNAB = async () => {
    if (!results.length || !user) return;
    
    setIsSyncing(true);
    setError(null);
    setSyncSuccess(null);

    try {
      // Format the date - use the end date if available, otherwise today
      const syncDate = endDate ? endDate : new Date().toISOString().split('T')[0];

      // Create sync request
      const response = await syncToYNAB({
        userId: user.id.toString(),
        date: syncDate,
        payeeName: "BennWallet Split Expenses",
        memo: `Expenses from ${startDate || 'account start'} to ${endDate || 'today'}`,
        categories: results.map(item => ({
          categoryName: item.category,
          amount: item.total
        }))
      });

      setSyncSuccess(true);
    } catch (error) {
      console.error('Error syncing to YNAB:', error);
      setError('Failed to sync with YNAB');
      setSyncSuccess(false);
    } finally {
      setIsSyncing(false);
    }
  };

  return (
    <div className="bg-white p-4 rounded shadow mb-6">
      <h2 className="text-xl font-bold mb-4">YNAB Splits Report</h2>
      
      {error && (
        <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4">
          {error}
        </div>
      )}

      {syncSuccess === true && (
        <div className="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded mb-4">
          Successfully synced to YNAB!
        </div>
      )}
      
      <div className="flex flex-col gap-4 mb-4">
        <div className="flex gap-2">
          <input
            type="date"
            value={startDate}
            onChange={(e) => setStartDate(e.target.value)}
            className="border rounded p-2"
            disabled={isLoading}
          />
          <input
            type="date"
            value={endDate}
            onChange={(e) => setEndDate(e.target.value)}
            className="border rounded p-2"
            disabled={isLoading}
          />
        </div>

        <select
          value={selectedCategory}
          onChange={(e) => setSelectedCategory(e.target.value)}
          className="border rounded p-2"
          disabled={isLoading}
        >
          <option value="">All Categories</option>
          {Array.isArray(categories) ? categories.map((category) => (
            <option key={category.id} value={category.name}>
              {category.name}
            </option>
          )) : null}
        </select>

        <button
          onClick={fetchReport}
          className="bg-blue-500 text-white p-2 rounded"
          disabled={isLoading}
        >
          {isLoading ? 'Loading...' : 'Generate Report'}
        </button>
      </div>

      {results.length > 0 && (
        <>
          <div className="overflow-x-auto">
            <table className="min-w-full table-auto">
              <thead>
                <tr className="bg-gray-200">
                  <th className="p-2">Category</th>
                  <th className="p-2">Total</th>
                </tr>
              </thead>
              <tbody>
                {results.map((item) => (
                  <tr key={item.category} className="border-t">
                    <td className="p-2">{item.category}</td>
                    <td className="p-2">${item.total.toFixed(2)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="mt-4">
            <button
              onClick={handleSyncToYNAB}
              className="bg-green-500 text-white p-2 rounded"
              disabled={isSyncing || !results.length}
            >
              {isSyncing ? 'Syncing...' : 'Sync to YNAB'}
            </button>
            <p className="text-xs text-gray-500 mt-1">
              This will create a transaction in YNAB with split categories based on the report above.
            </p>
          </div>
        </>
      )}
      
      {!isLoading && results.length === 0 && !error && (
        <p className="text-gray-500">No results to display. Try adjusting your filters.</p>
      )}
    </div>
  );
} 