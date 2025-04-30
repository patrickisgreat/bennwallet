import { useState, useEffect } from 'react';
import { fetchYNABSplits, ReportFilter, CategoryTotal } from '../utils/api';
import { useAuth } from '../context/AuthContext';

function ReportsPage() {
  const { currentUser } = useAuth();
  const [splits, setSplits] = useState<CategoryTotal[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState<ReportFilter>({
    startDate: new Date(new Date().getFullYear(), 0, 1).toISOString().split('T')[0], // First day of current year
    endDate: new Date().toISOString().split('T')[0], // Today
    payTo: '',
    enteredBy: '',
    category: '',
  });
  const [total, setTotal] = useState(0);

  useEffect(() => {
    if (currentUser) {
      loadReportData();
    }
  }, [currentUser]);

  const loadReportData = async () => {
    if (!currentUser) return;
    
    setLoading(true);
    try {
      console.log('Sending filter to YNAB splits API:', filter);
      
      // Make sure dates are formatted correctly and filter is complete
      const filterToSend = {
        ...filter,
        startDate: filter.startDate || undefined, 
        endDate: filter.endDate || undefined,
        category: filter.category || undefined,
        payTo: filter.payTo || undefined,
        enteredBy: filter.enteredBy || undefined,
      };
      
      const data = await fetchYNABSplits(filterToSend);
      console.log('Received YNAB splits data:', data);
      
      setSplits(data);
      
      // Calculate total for percentage
      const sum = data.reduce((acc, item) => acc + item.total, 0);
      setTotal(sum);
      
      setError(null);
    } catch (err) {
      console.error('Error loading report data:', err);
      setError('Failed to load report data. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  const handleFilterChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value } = e.target;
    setFilter(prev => ({ ...prev, [name]: value }));
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    loadReportData();
  };

  // Create a color for each category
  const getColorForIndex = (index: number) => {
    const colors = [
      '#4299E1', '#48BB78', '#F6AD55', '#F56565', 
      '#9F7AEA', '#ED64A6', '#ECC94B', '#38B2AC'
    ];
    return colors[index % colors.length];
  };

  useEffect(() => {
    console.log('Rendering report with data:', splits);
  }, [splits]);

  return (
    <div>
      <h1 className="text-2xl font-bold mb-4">YNAB Category Splits</h1>
      
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
      
      <form onSubmit={handleSubmit} className="bg-white p-4 rounded shadow mb-4">
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Start Date</label>
            <input
              type="date"
              name="startDate"
              value={filter.startDate}
              onChange={handleFilterChange}
              className="mt-1 block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
            />
          </div>
          
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">End Date</label>
            <input
              type="date"
              name="endDate"
              value={filter.endDate}
              onChange={handleFilterChange}
              className="mt-1 block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
            />
          </div>
          
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Pay To</label>
            <select
              name="payTo"
              value={filter.payTo}
              onChange={handleFilterChange}
              className="mt-1 block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
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
              className="mt-1 block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
            >
              <option value="">All</option>
              <option value="Sarah">Sarah</option>
              <option value="Patrick">Patrick</option>
            </select>
          </div>
        </div>
        
        <div className="mt-4">
          <button
            type="submit"
            className="bg-indigo-600 text-white px-4 py-2 rounded-md hover:bg-indigo-700"
            disabled={loading}
          >
            {loading ? 'Loading...' : 'Generate Report'}
          </button>
        </div>
      </form>
      
      {loading ? (
        <div className="text-center py-4">Loading report data...</div>
      ) : splits.length > 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* Left side: Bar chart visualization */}
          <div className="bg-white p-4 rounded shadow">
            <h2 className="text-xl font-semibold mb-4">Category Split</h2>
            {splits.length === 0 ? (
              <div className="h-64 flex items-center justify-center text-gray-500">
                No data to display
              </div>
            ) : (
              <div className="h-64 flex items-end space-x-1" style={{ minHeight: "200px" }}>
                {splits.map((item, index) => {
                  const percentage = (item.total / total) * 100;
                  // Calculate height in pixels (max height would be the container's height - some space for labels)
                  const maxBarHeight = 180; // px
                  const barHeight = Math.max((percentage / 100) * maxBarHeight, 10); // min 10px
                  
                  console.log(`Bar ${item.category}: ${percentage.toFixed(1)}% => ${barHeight}px height`);
                  
                  return (
                    <div key={item.category} className="flex flex-col items-center" style={{ flex: '1 1 0%', minWidth: '30px' }}>
                      <div 
                        className="w-full rounded-t transition-all duration-500 ease-in-out"
                        style={{ 
                          height: `${barHeight}px`,
                          backgroundColor: getColorForIndex(index),
                          minHeight: '10px',
                          border: '1px solid rgba(0,0,0,0.1)',
                        }}
                      ></div>
                      <div className="text-xs mt-2 w-full text-center truncate font-medium" title={item.category}>
                        {item.category}
                      </div>
                      <div className="text-xs font-semibold">
                        ${item.total.toFixed(2)} ({percentage.toFixed(1)}%)
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
          
          {/* Right side: Data table */}
          <div className="bg-white p-4 rounded shadow">
            <h2 className="text-xl font-semibold mb-4">Category Breakdown</h2>
            <div className="overflow-x-auto">
              <table className="min-w-full table-auto">
                <thead>
                  <tr className="bg-gray-100">
                    <th className="p-2 text-left">Category</th>
                    <th className="p-2 text-right">Amount</th>
                    <th className="p-2 text-right">Percentage</th>
                  </tr>
                </thead>
                <tbody>
                  {splits.map((item, index) => (
                    <tr key={item.category} className="border-t">
                      <td className="p-2">
                        <div className="flex items-center">
                          <span 
                            className="w-3 h-3 rounded-full mr-2" 
                            style={{ backgroundColor: getColorForIndex(index) }}
                          />
                          {item.category}
                        </div>
                      </td>
                      <td className="p-2 text-right">${item.total.toFixed(2)}</td>
                      <td className="p-2 text-right">{((item.total / total) * 100).toFixed(1)}%</td>
                    </tr>
                  ))}
                  <tr className="font-bold border-t-2 border-gray-300">
                    <td className="p-2">Total</td>
                    <td className="p-2 text-right">${total.toFixed(2)}</td>
                    <td className="p-2 text-right">100%</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      ) : (
        <div className="bg-white p-4 rounded shadow text-center">
          No data available for the selected filters. Try adjusting your filters or adding more transactions.
        </div>
      )}
    </div>
  );
}

export default ReportsPage; 