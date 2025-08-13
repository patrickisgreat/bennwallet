import { useState, useEffect, useCallback } from 'react';
import {
  fetchYNABSplits,
  syncToYNAB,
  ReportFilter,
  CategoryTotal,
  ReportResponse,
  SettlementReport,
  fetchUniqueTransactionFields,
} from '../utils/api';
import { useAuth } from '../context/AuthContext';

function ReportsPage() {
  const { currentUser } = useAuth();
  const [splits, setSplits] = useState<CategoryTotal[]>([]);
  const [settlementData, setSettlementData] = useState<SettlementReport | null>(null);
  const [showSettlements, setShowSettlements] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [authChecked, setAuthChecked] = useState(false);
  const [isSyncing, setIsSyncing] = useState(false);
  const [syncSuccess, setSyncSuccess] = useState<boolean | null>(null);
  const [filter, setFilter] = useState<Partial<ReportFilter>>({});
  const [total, setTotal] = useState(0);
  const [uniqueFields, setUniqueFields] = useState<{ payTo: string[]; enteredBy: string[] }>({
    payTo: [],
    enteredBy: [],
  });

  // First check if user is authenticated
  useEffect(() => {
    if (currentUser) {
      const userId = localStorage.getItem('userId');
      if (!userId) {
        console.log('Waiting for userId to be set in localStorage...');
        // Don't set authChecked yet
      } else {
        console.log('User authenticated with userId:', userId);
        setAuthChecked(true);
      }
    }
  }, [currentUser]);

  // Load unique fields for dropdowns
  const loadUniqueFields = useCallback(async () => {
    try {
      const fields = await fetchUniqueTransactionFields();
      console.log('Loaded unique fields for reports:', fields);
      // Add detailed logging for payTo values
      if (fields && fields.payTo && fields.payTo.length > 0) {
        console.log('Available PayTo values for filtering:', JSON.stringify(fields.payTo));
      } else {
        console.warn('No PayTo values available for filtering');
      }
      setUniqueFields(fields);
    } catch (err) {
      console.error('Error loading unique transaction fields:', err);
    }
  }, []);

  const loadReportData = useCallback(async () => {
    if (!currentUser) {
      console.warn('Attempted to load data without authenticated user');
      return;
    }

    const userId = localStorage.getItem('userId');
    if (!userId) {
      setError('Authentication issue: Please log out and log back in');
      return;
    }

    // Validate dates
    if (filter.startDate && filter.endDate) {
      const startDate = new Date(filter.startDate);
      const endDate = new Date(filter.endDate);
      if (startDate > endDate) {
        setError('Start date must be before end date');
        return;
      }
    }

    setLoading(true);
    setSplits([]);
    setSettlementData(null);
    setTotal(0);
    setError(null);
    setSyncSuccess(null);

    try {
      console.log('Sending filter to YNAB splits API:', filter);

      // Make sure filter is properly formatted
      const filterToSend: ReportFilter = {};
      if (filter.startDate) filterToSend.startDate = filter.startDate;
      if (filter.endDate) filterToSend.endDate = filter.endDate;
      if (filter.category) filterToSend.category = filter.category;
      if (filter.payTo) filterToSend.payTo = filter.payTo;
      if (filter.enteredBy) filterToSend.enteredBy = filter.enteredBy;
      if (typeof filter.paid === 'boolean') filterToSend.paid = filter.paid;
      if (typeof filter.optional === 'boolean') filterToSend.optional = filter.optional;

      // Add month/year for settlement data if we have date filters
      if (filterToSend.startDate && filterToSend.endDate) {
        const startDate = new Date(filterToSend.startDate);
        filterToSend.transactionDateMonth = startDate.getMonth() + 1;
        filterToSend.transactionDateYear = startDate.getFullYear();
      }

      // Add more detailed logging for debugging
      console.log('Filter values being sent:');
      console.log('- startDate:', filterToSend.startDate);
      console.log('- endDate:', filterToSend.endDate);
      console.log('- payTo:', filterToSend.payTo, typeof filterToSend.payTo);
      console.log('- enteredBy:', filterToSend.enteredBy, typeof filterToSend.enteredBy);
      console.log('- paid:', filterToSend.paid, typeof filterToSend.paid);
      console.log('- optional:', filterToSend.optional, typeof filterToSend.optional);

      const data = await fetchYNABSplits(filterToSend, showSettlements);
      console.log('Received YNAB splits data:', data);

      // Handle new response format
      if (showSettlements && data && typeof data === 'object' && 'categoryTotals' in data) {
        const reportResponse = data as ReportResponse;
        setSplits(reportResponse.categoryTotals || []);
        setSettlementData(reportResponse.settlementData || null);

        // Calculate total for percentage
        const categoryTotals = reportResponse.categoryTotals || [];
        const sum = categoryTotals.reduce((acc, item) => acc + (item.total || item.amount || 0), 0);
        setTotal(sum);
      } else if (Array.isArray(data) && data.length) {
        setSplits(data);
        setSettlementData(null);

        // Calculate total for percentage
        const sum = data.reduce((acc, item) => acc + (item.total || item.amount || 0), 0);
        setTotal(sum);
      } else {
        console.warn('No data or empty array returned from report API');
        setSplits([]);
        setSettlementData(null);
        setTotal(0);
      }
    } catch (err) {
      console.error('Error loading report data:', err);
      setError('Failed to load report data. Please try again.');
      setSplits([]);
      setSettlementData(null);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  }, [
    currentUser,
    filter,
    showSettlements,
    setError,
    setLoading,
    setSplits,
    setTotal,
    setSyncSuccess,
  ]);

  // Only load data after authentication is confirmed
  useEffect(() => {
    if (currentUser && authChecked) {
      loadReportData();
      loadUniqueFields();
    }
  }, [currentUser, authChecked, loadReportData, loadUniqueFields]);

  const handleFilterChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value } = e.target;

    // Add detailed logging for EnteredBy changes
    if (name === 'enteredBy') {
      console.log('EnteredBy changed to:', value);
      console.log('EnteredBy value type:', typeof value);
    }

    setFilter(prev => ({ ...prev, [name]: value }));
  };

  const handleCheckboxChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, checked } = e.target;
    setFilter(prev => ({ ...prev, [name]: checked }));
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    loadReportData();
  };

  const handleSyncToYNAB = async () => {
    if (!splits.length || !currentUser) return;

    const userId = localStorage.getItem('userId');
    if (!userId) {
      setError('Authentication issue: Please log out and log back in');
      return;
    }

    setIsSyncing(true);
    setError(null);
    setSyncSuccess(null);

    try {
      // Format the date - use the end date if available, otherwise today
      const syncDate = filter.endDate ? filter.endDate : new Date().toISOString().split('T')[0];

      // Create sync request
      await syncToYNAB({
        userId: userId,
        date: syncDate,
        payeeName: 'BennWallet Split Expenses',
        memo: `Expenses from ${filter.startDate || 'account start'} to ${filter.endDate || 'today'}`,
        categories: splits.map(item => ({
          categoryName: item.category || item.categoryName || 'Unknown',
          amount: item.total || item.amount || 0,
        })),
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

  // Create a color for each category
  const getColorForIndex = (index: number) => {
    const colors = [
      '#4299E1',
      '#48BB78',
      '#F6AD55',
      '#F56565',
      '#9F7AEA',
      '#ED64A6',
      '#ECC94B',
      '#38B2AC',
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
          <button className="float-right font-bold" onClick={() => setError(null)}>
            &times;
          </button>
        </div>
      )}

      {syncSuccess === true && (
        <div className="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded mb-4">
          Successfully synced to YNAB!
          <button className="float-right font-bold" onClick={() => setSyncSuccess(null)}>
            &times;
          </button>
        </div>
      )}

      <form onSubmit={handleSubmit} className="bg-white p-4 rounded shadow mb-4">
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div>
            <label htmlFor="startDate" className="block text-sm font-medium text-gray-700 mb-1">
              Start Date
            </label>
            <input
              id="startDate"
              type="date"
              name="startDate"
              value={filter.startDate}
              onChange={handleFilterChange}
              className="mt-1 block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
            />
          </div>

          <div>
            <label htmlFor="endDate" className="block text-sm font-medium text-gray-700 mb-1">
              End Date
            </label>
            <input
              id="endDate"
              type="date"
              name="endDate"
              value={filter.endDate}
              onChange={handleFilterChange}
              className="mt-1 block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Owed By</label>
            <select
              name="payTo"
              value={filter.payTo}
              onChange={handleFilterChange}
              className="mt-1 block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
            >
              <option value="">All</option>
              {uniqueFields.payTo.map(name => (
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
              className="mt-1 block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
            >
              <option value="">All</option>
              {uniqueFields.enteredBy.map(name => {
                console.log('EnteredBy option:', name);
                return (
                  <option key={name} value={name}>
                    {name}
                  </option>
                );
              })}
            </select>
          </div>

          <div className="flex items-center mt-6">
            <input
              id="paid-only"
              type="checkbox"
              name="paid"
              checked={filter.paid === true}
              onChange={handleCheckboxChange}
              className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
            />
            <label htmlFor="paid-only" className="ml-2 block text-sm text-gray-900">
              Show only paid transactions
            </label>
          </div>

          <div className="flex items-center mt-6">
            <input
              id="exclude-optional"
              type="checkbox"
              name="optional"
              checked={filter.optional === true}
              onChange={handleCheckboxChange}
              className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
            />
            <label htmlFor="exclude-optional" className="ml-2 block text-sm text-gray-900">
              Exclude optional transactions
            </label>
          </div>

          <div className="flex items-center mt-6">
            <input
              id="show-settlements"
              type="checkbox"
              checked={showSettlements}
              onChange={e => setShowSettlements(e.target.checked)}
              className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
            />
            <label htmlFor="show-settlements" className="ml-2 block text-sm text-gray-900">
              Include settlement data
            </label>
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

      {loading && (
        <div className="text-center py-4" data-testid="loading-indicator">
          <p>Loading report data...</p>
        </div>
      )}

      {!loading && splits.length === 0 && !error && (
        <div className="text-center py-4">
          <p>No data available for the selected filters.</p>
        </div>
      )}

      {splits.length > 0 && (
        <div className="mt-4">
          <button
            onClick={handleSyncToYNAB}
            disabled={isSyncing || splits.length === 0}
            className="bg-blue-500 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded disabled:opacity-50"
          >
            {isSyncing ? 'Syncing to YNAB...' : 'Sync This Report to YNAB'}
          </button>
          <p className="mt-2 text-sm text-gray-600">
            This will create a single transaction in YNAB with split categories based on the report
            above. The total amount will be ${total.toFixed(2)}.
          </p>
        </div>
      )}

      {splits.length > 0 ? (
        <div className="grid grid-cols-1 gap-6">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            {/* Left side: Bar chart visualization */}
            <div className="bg-white p-4 rounded shadow">
              <h2 className="text-xl font-semibold mb-4">Category Split</h2>
              {splits.length === 0 ? (
                <div className="h-64 flex items-center justify-center text-gray-500">
                  No data to display
                </div>
              ) : (
                <div className="h-64 flex items-end space-x-1" style={{ minHeight: '200px' }}>
                  {splits.map((item, index) => {
                    const amount = item.total || item.amount || 0;
                    const categoryName = item.category || item.categoryName || 'Unknown';
                    const percentage = (amount / total) * 100;
                    // Calculate height in pixels (max height would be the container's height - some space for labels)
                    const maxBarHeight = 180; // px
                    const barHeight = Math.max((percentage / 100) * maxBarHeight, 10); // min 10px

                    console.log(
                      `Bar ${categoryName}: ${percentage.toFixed(1)}% => ${barHeight}px height`
                    );

                    return (
                      <div
                        key={categoryName}
                        className="flex flex-col items-center"
                        style={{ flex: '1 1 0%', minWidth: '30px' }}
                      >
                        <div
                          className="w-full rounded-t transition-all duration-500 ease-in-out"
                          style={{
                            height: `${barHeight}px`,
                            backgroundColor: getColorForIndex(index),
                            minHeight: '10px',
                            border: '1px solid rgba(0,0,0,0.1)',
                          }}
                        ></div>
                        <div
                          className="text-xs mt-2 w-full text-center truncate font-medium"
                          title={categoryName}
                        >
                          {categoryName}
                        </div>
                        <div className="text-xs font-semibold">
                          ${amount.toFixed(2)} ({percentage.toFixed(1)}%)
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
                    {splits.map((item, index) => {
                      const amount = item.total || item.amount || 0;
                      const categoryName = item.category || item.categoryName || 'Unknown';
                      return (
                        <tr key={`${categoryName}-${index}`} className="border-t">
                          <td className="p-2">
                            <div className="flex items-center">
                              <span
                                className="w-3 h-3 rounded-full mr-2"
                                style={{ backgroundColor: getColorForIndex(index) }}
                              />
                              {categoryName}
                            </div>
                          </td>
                          <td className="p-2 text-right">${amount.toFixed(2)}</td>
                          <td className="p-2 text-right">{((amount / total) * 100).toFixed(1)}%</td>
                        </tr>
                      );
                    })}
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
        </div>
      ) : (
        <div className="bg-white p-4 rounded shadow text-center">
          No data available for the selected filters. Try adjusting your filters or adding more
          transactions.
        </div>
      )}

      {/* Settlement Data Section */}
      {settlementData && (
        <div className="mt-6">
          <div className="bg-white p-4 rounded shadow">
            <h2 className="text-xl font-semibold mb-4">
              Settlement Data - {settlementData.monthName} {settlementData.year}
            </h2>

            {/* Settlement Summary */}
            <div className="mb-4 grid grid-cols-3 gap-4">
              <div className="bg-red-50 p-4 rounded-lg">
                <div className="text-sm font-medium text-red-700">Total Owed</div>
                <div className="text-xl font-bold text-red-900">
                  ${settlementData.totalOwed.toFixed(2)}
                </div>
              </div>
              <div className="bg-green-50 p-4 rounded-lg">
                <div className="text-sm font-medium text-green-700">Total Paid</div>
                <div className="text-xl font-bold text-green-900">
                  ${settlementData.totalPaid.toFixed(2)}
                </div>
              </div>
              <div className="bg-blue-50 p-4 rounded-lg">
                <div className="text-sm font-medium text-blue-700">Net Amount</div>
                <div
                  className={`text-xl font-bold ${
                    settlementData.netAmount >= 0 ? 'text-green-900' : 'text-red-900'
                  }`}
                >
                  ${settlementData.netAmount.toFixed(2)}
                </div>
              </div>
            </div>

            {/* Settlement Deductions by Category */}
            {settlementData.settlementDeductions &&
              settlementData.settlementDeductions.length > 0 && (
                <div className="mt-6">
                  <h3 className="text-lg font-medium mb-3">Settlement Deductions by Category</h3>
                  <div className="overflow-x-auto">
                    <table className="min-w-full table-auto">
                      <thead>
                        <tr className="bg-gray-100">
                          <th className="p-2 text-left">Category</th>
                          <th className="p-2 text-right">Amount</th>
                          <th className="p-2 text-right">Count</th>
                        </tr>
                      </thead>
                      <tbody>
                        {settlementData.settlementDeductions.map((item, index) => (
                          <tr key={index} className="border-t">
                            <td className="p-2">{item.categoryName || item.category}</td>
                            <td className="p-2 text-right">${(item.amount || 0).toFixed(2)}</td>
                            <td className="p-2 text-right">{item.count || 0}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              )}
          </div>
        </div>
      )}
    </div>
  );
}

export default ReportsPage;
