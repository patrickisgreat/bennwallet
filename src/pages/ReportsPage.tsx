import React, { useState, useEffect } from 'react';
import {
  fetchYNABSplits,
  syncToYNAB,
  CategoryTotal,
  fetchUniqueTransactionFields,
  fetchYNABCategories,
  CategoryGroup,
} from '../utils/api';
import { useAuth } from '../context/AuthContext';
import { ReportFilter as ReportFilterType } from '../types/report';

function ReportsPage() {
  const { currentUser } = useAuth();
  const [ynabSplits, setYNABSplits] = useState<CategoryTotal[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [authChecked, setAuthChecked] = useState(false);
  const [isSyncing, setIsSyncing] = useState(false);
  const [syncSuccess, setSyncSuccess] = useState<boolean | null>(null);
  const [uniqueFields, setUniqueFields] = useState<{ payTo: string[]; enteredBy: string[] }>({
    payTo: [],
    enteredBy: [],
  });
  const [categoryGroups, setCategoryGroups] = useState<CategoryGroup[]>([]);
  const [allCategories, setAllCategories] = useState<string[]>([]);
  const [showDebug, setShowDebug] = useState(false);

  // Initialize filter with current month
  const currentDate = new Date();
  const [filter, setFilter] = useState<ReportFilterType>({
    startDate: new Date(currentDate.getFullYear(), currentDate.getMonth(), 1)
      .toISOString()
      .split('T')[0],
    endDate: new Date(currentDate.getFullYear(), currentDate.getMonth() + 1, 0)
      .toISOString()
      .split('T')[0],
    category: '',
    payTo: '',
    enteredBy: '',
    paid: true,
    optional: false,
    transactionDateMonth: currentDate.getMonth() + 1,
    transactionDateYear: currentDate.getFullYear(),
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

  // Only load data after authentication is confirmed
  useEffect(() => {
    if (currentUser && authChecked) {
      loadReportData();
      loadCategoryData();
    }
  }, [currentUser, authChecked]);

  useEffect(() => {
    // Fetch unique transaction fields
    const loadUniqueFields = async () => {
      try {
        console.log('Fetching unique fields...');
        const fields = await fetchUniqueTransactionFields();
        console.log('Raw unique fields response:', fields);
        console.log('PayTo values:', fields.payTo);
        console.log('EnteredBy values:', fields.enteredBy);

        if (!fields.payTo || !fields.enteredBy) {
          console.error('Invalid fields data structure:', fields);
          return;
        }

        setUniqueFields(fields);
      } catch (err) {
        console.error('Error loading unique fields:', err);
        setError('Failed to load filter options. Please try refreshing the page.');
      }
    };

    if (currentUser) {
      loadUniqueFields();
    }
  }, [currentUser]);

  const loadCategoryData = async () => {
    try {
      console.log('Fetching YNAB categories...');
      const groups = await fetchYNABCategories();
      console.log('Retrieved category groups:', groups);
      setCategoryGroups(groups);

      // Extract all category names for the dropdown
      const categories: string[] = [];
      groups.forEach(group => {
        console.log(`Group: ${group.name} has ${group.categories.length} categories`);
        group.categories.forEach(cat => {
          categories.push(cat.name);
        });
      });

      console.log('All available categories:', categories);
      setAllCategories(categories);
    } catch (err) {
      console.error('Error loading YNAB categories:', err);
    }
  };

  const loadReportData = async () => {
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
    setYNABSplits([]);
    setError(null);
    setSyncSuccess(null);

    try {
      console.log('Sending filter to YNAB splits API:', filter);

      const data = await fetchYNABSplits(filter);
      console.log('Received YNAB splits data:', data);

      if (Array.isArray(data) && data.length) {
        setYNABSplits(data);
      } else {
        console.warn('No data or empty array returned from report API');
        setYNABSplits([]);
      }
    } catch (err) {
      console.error('Error loading report data:', err);
      setError('Failed to load report data. Please try again.');
      setYNABSplits([]);
    } finally {
      setLoading(false);
    }
  };

  const handleFilterChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value, type } = e.target;

    if (type === 'checkbox') {
      setFilter(prev => ({
        ...prev,
        [name]: (e.target as HTMLInputElement).checked,
      }));
    } else if (name === 'transactionDateMonth' || name === 'transactionDateYear') {
      // Convert month and year to numbers or null
      const numValue = value === '' ? null : parseInt(value, 10);
      setFilter(prev => ({
        ...prev,
        [name]: numValue,
      }));
    } else {
      setFilter(prev => ({
        ...prev,
        [name]: value,
      }));
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      const splits = await fetchYNABSplits(filter);
      setYNABSplits(splits);
    } catch (err) {
      setError('Failed to generate report. Please try again.');
      console.error('Error generating report:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleSyncToYNAB = async () => {
    if (!ynabSplits.length || !currentUser) return;

    const userId = localStorage.getItem('userId');
    if (!userId) {
      setError('User ID not found');
      return;
    }

    setIsSyncing(true);
    setError(null);
    setSyncSuccess(null);

    try {
      // Format the date - use the end date if available, otherwise today
      const syncDate = filter.endDate ? filter.endDate : new Date().toISOString().split('T')[0];

      await syncToYNAB({
        userId,
        date: syncDate,
        payeeName: 'BennWallet Split Expenses',
        memo: `Expenses from ${filter.startDate || 'account start'} to ${filter.endDate || 'today'}`,
        categories: ynabSplits.map(item => ({
          categoryName: item.category,
          amount: item.total,
        })),
      });

      setSyncSuccess(true);
    } catch (err) {
      console.error('Error syncing to YNAB:', err);
      setError('Failed to sync to YNAB. Please try again.');
      setSyncSuccess(false);
    } finally {
      setIsSyncing(false);
    }
  };

  const getColorForIndex = (index: number) => {
    const colors = [
      'bg-blue-500',
      'bg-green-500',
      'bg-yellow-500',
      'bg-red-500',
      'bg-purple-500',
      'bg-pink-500',
      'bg-indigo-500',
      'bg-teal-500',
      'bg-orange-500',
      'bg-gray-500',
    ];
    return colors[index % colors.length];
  };

  return (
    <div className="container mx-auto px-4 py-8">
      <h1 className="text-2xl font-bold mb-6">Reports</h1>

      {/* Debug panel for category structure */}
      {process.env.NODE_ENV === 'development' && (
        <div className="mb-6">
          <button
            onClick={() => setShowDebug(!showDebug)}
            className="text-gray-500 text-sm underline"
          >
            {showDebug ? 'Hide' : 'Show'} Debug Info
          </button>

          {showDebug && (
            <div className="mt-2 p-4 bg-gray-100 rounded text-xs overflow-auto max-h-64">
              <h3 className="font-bold">YNAB Category Structure:</h3>
              {categoryGroups.length > 0 ? (
                <ul className="ml-4 mt-2">
                  {categoryGroups.map(group => (
                    <li key={group.id} className="mb-2">
                      <strong>{group.name}</strong> ({group.categories.length} categories)
                      <ul className="ml-4">
                        {group.categories.map(cat => (
                          <li key={cat.id}>{cat.name}</li>
                        ))}
                      </ul>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="text-red-500">No category groups loaded</p>
              )}
            </div>
          )}
        </div>
      )}

      <div className="bg-white rounded-lg shadow-md p-6 mb-6">
        <h2 className="text-xl font-semibold mb-4">YNAB Categories Split</h2>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
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
              <label className="block text-sm font-medium text-gray-700 mb-1">Category</label>
              <select
                name="category"
                value={filter.category}
                onChange={handleFilterChange}
                className="block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
              >
                <option value="">All Categories</option>
                {allCategories.map(category => (
                  <option key={category} value={category}>
                    {category}
                  </option>
                ))}
              </select>
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
                {uniqueFields.payTo.map(value => (
                  <option key={value} value={value}>
                    {value}
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
                {uniqueFields.enteredBy.map(value => (
                  <option key={value} value={value}>
                    {value}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Transaction Month
              </label>
              <select
                name="transactionDateMonth"
                value={filter.transactionDateMonth}
                onChange={handleFilterChange}
                className="block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
              >
                <option value="">All</option>
                {Array.from({ length: 12 }, (_, i) => i + 1).map(month => (
                  <option key={month} value={month}>
                    {new Date(2000, month - 1, 1).toLocaleString('default', { month: 'long' })}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Transaction Year
              </label>
              <select
                name="transactionDateYear"
                value={filter.transactionDateYear}
                onChange={handleFilterChange}
                className="block w-full rounded-md border border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 px-3 py-2"
              >
                <option value="">All</option>
                {Array.from({ length: 5 }, (_, i) => currentDate.getFullYear() - i).map(year => (
                  <option key={year} value={year}>
                    {year}
                  </option>
                ))}
              </select>
            </div>
          </div>

          <div className="flex items-center space-x-4">
            <label className="flex items-center">
              <input
                type="checkbox"
                name="paid"
                checked={filter.paid}
                onChange={handleFilterChange}
                className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
              />
              <span className="ml-2 text-sm text-gray-700">Only Show Paid</span>
            </label>

            <label className="flex items-center">
              <input
                type="checkbox"
                name="optional"
                checked={filter.optional}
                onChange={handleFilterChange}
                className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
              />
              <span className="ml-2 text-sm text-gray-700">Include Optional</span>
            </label>
          </div>

          <button
            type="submit"
            className="bg-indigo-600 text-white px-4 py-2 rounded-md hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2"
          >
            Generate Report
          </button>
        </form>

        {error && <div className="mt-4 p-4 bg-red-50 text-red-700 rounded-md">{error}</div>}

        {syncSuccess === true && (
          <div className="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded mb-4">
            Successfully synced to YNAB!
            <button className="float-right font-bold" onClick={() => setSyncSuccess(null)}>
              &times;
            </button>
          </div>
        )}

        {loading ? (
          <div className="text-center py-4">Loading report data...</div>
        ) : ynabSplits.length > 0 ? (
          <div className="mt-6">
            <h3 className="text-lg font-medium mb-4">Results</h3>
            <div className="space-y-4">
              {ynabSplits.map((split, index) => (
                <div key={split.category} className="space-y-2">
                  <div className="flex justify-between items-center">
                    <span className="text-sm font-medium text-gray-700">{split.category}</span>
                    <span className="text-sm font-medium text-gray-900">
                      ${split.total.toFixed(2)}
                    </span>
                  </div>
                  <div className="w-full bg-gray-200 rounded-full h-2.5">
                    <div
                      className={`${getColorForIndex(index)} h-2.5 rounded-full`}
                      style={{
                        width: `${(split.total / ynabSplits.reduce((acc, item) => acc + item.total, 0)) * 100}%`,
                      }}
                    ></div>
                  </div>
                </div>
              ))}
              <div className="mt-4 pt-4 border-t border-gray-200">
                <div className="flex justify-between items-center">
                  <span className="text-lg font-medium text-gray-900">Total</span>
                  <span className="text-lg font-medium text-gray-900">
                    ${ynabSplits.reduce((acc, item) => acc + item.total, 0).toFixed(2)}
                  </span>
                </div>
              </div>
            </div>
          </div>
        ) : (
          <div className="mt-6 text-center text-gray-500">
            No data available for the selected filters
          </div>
        )}
      </div>

      <div className="mt-6">
        <h3 className="text-lg font-medium mb-4">Sync to YNAB</h3>
        <div className="flex flex-col items-center">
          <button
            onClick={handleSyncToYNAB}
            className="bg-green-500 hover:bg-green-600 text-white font-bold py-3 px-6 rounded-lg text-lg shadow-md transition-all duration-300 disabled:opacity-50 disabled:cursor-not-allowed"
            disabled={isSyncing || !ynabSplits.length}
          >
            {isSyncing ? 'Syncing to YNAB...' : 'Sync This Report to YNAB'}
          </button>
          <p className="text-sm text-gray-600 mt-2 text-center max-w-lg">
            This will create a single transaction in YNAB with split categories based on the report
            above. The total amount will be $
            {ynabSplits.reduce((acc, item) => acc + item.total, 0).toFixed(2)}.
          </p>
        </div>
      </div>
    </div>
  );
}

export default ReportsPage;
