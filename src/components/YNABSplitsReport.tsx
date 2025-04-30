import { useState, useEffect } from 'react';
import { Category } from '../types/category';
import { fetchYNABSplits, CategoryTotal } from '../utils/api';

interface YNABSplitsReportProps {
  categories: Category[];
  currentUser: string;
}

export default function YNABSplitsReport({ categories, currentUser }: YNABSplitsReportProps) {
  const [startDate, setStartDate] = useState<string>('');
  const [endDate, setEndDate] = useState<string>('');
  const [selectedCategory, setSelectedCategory] = useState<string>('');
  const [results, setResults] = useState<CategoryTotal[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchReport = async () => {
    setIsLoading(true);
    setError(null);
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

  return (
    <div className="bg-white p-4 rounded shadow mb-6">
      <h2 className="text-xl font-bold mb-4">YNAB Splits Report</h2>
      
      {error && (
        <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4">
          {error}
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
      )}
      
      {!isLoading && results.length === 0 && !error && (
        <p className="text-gray-500">No results to display. Try adjusting your filters.</p>
      )}
    </div>
  );
} 