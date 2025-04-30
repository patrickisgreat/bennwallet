import { useState, useEffect } from 'react';
import { Category } from '../types/category';

interface YNABSplitsReportProps {
  categories: Category[];
  currentUser: string;
}

interface CategoryTotal {
  category: string;
  total: number;
}

export default function YNABSplitsReport({ categories, currentUser }: YNABSplitsReportProps) {
  const [startDate, setStartDate] = useState<string>('');
  const [endDate, setEndDate] = useState<string>('');
  const [selectedCategory, setSelectedCategory] = useState<string>('');
  const [results, setResults] = useState<CategoryTotal[]>([]);
  const [isLoading, setIsLoading] = useState(false);

  const fetchReport = async () => {
    setIsLoading(true);
    try {
      const response = await fetch('http://localhost:8080/reports/ynab-splits', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          startDate,
          endDate,
          category: selectedCategory || undefined,
          enteredBy: currentUser,
        }),
      });

      if (!response.ok) {
        throw new Error('Failed to fetch report');
      }

      const data = await response.json();
      setResults(data);
    } catch (error) {
      console.error('Error fetching report:', error);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="bg-white p-4 rounded shadow mb-6">
      <h2 className="text-xl font-bold mb-4">YNAB Splits Report</h2>
      
      <div className="flex flex-col gap-4 mb-4">
        <div className="flex gap-2">
          <input
            type="date"
            value={startDate}
            onChange={(e) => setStartDate(e.target.value)}
            className="border rounded p-2"
          />
          <input
            type="date"
            value={endDate}
            onChange={(e) => setEndDate(e.target.value)}
            className="border rounded p-2"
          />
        </div>

        <select
          value={selectedCategory}
          onChange={(e) => setSelectedCategory(e.target.value)}
          className="border rounded p-2"
        >
          <option value="">All Categories</option>
          {categories.map((category) => (
            <option key={category.id} value={category.name}>
              {category.name}
            </option>
          ))}
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
    </div>
  );
} 