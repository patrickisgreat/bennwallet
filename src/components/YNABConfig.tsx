import { useState, useEffect } from 'react';
import { fetchYNABConfig, updateYNABConfig, syncYNABCategories, YNABConfig as YNABConfigType } from '../utils/api';

interface YNABConfigProps {
  userId: string;
}

export function YNABConfig({ userId }: YNABConfigProps) {
  const [config, setConfig] = useState<YNABConfigType | null>(null);
  const [apiToken, setApiToken] = useState('');
  const [showApiToken, setShowApiToken] = useState(false);
  const [budgetId, setBudgetId] = useState('');
  const [accountId, setAccountId] = useState('');
  const [syncFrequency, setSyncFrequency] = useState(60);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [syncing, setSyncing] = useState(false);

  useEffect(() => {
    loadConfig();
  }, []);

  const loadConfig = async () => {
    try {
      setLoading(true);
      const data = await fetchYNABConfig();
      console.log('Config loaded:', data);
      
      // Log specific fields
      if (data) {
        console.log('budgetId:', data.budgetId);
        console.log('accountId:', data.accountId);
        console.log('apiToken value:', data.apiToken);
        console.log('apiToken type:', typeof data.apiToken);
        console.log('apiToken empty check:', data.apiToken === '');
        console.log('apiToken undefined check:', data.apiToken === undefined);
        console.log('hasCredentials:', data.hasCredentials);
        
        setConfig(data);
        setSyncFrequency(data.syncFrequency || 60);
        
        // Set the form fields with the returned data
        if (data.budgetId) {
          console.log('Setting budgetId field to:', data.budgetId);
          setBudgetId(data.budgetId);
        }
        
        if (data.accountId) {
          console.log('Setting accountId field to:', data.accountId);
          setAccountId(data.accountId);
        }
        
        // For API token, if we receive a placeholder, show it in the field
        if (data.apiToken) {
          console.log('Setting apiToken field to placeholder:', data.apiToken);
          setApiToken(data.apiToken);
        } else if (data.hasCredentials) {
          // Fallback in case the API token is missing but credentials exist
          console.log('API token missing but hasCredentials is true, setting placeholder');
          setApiToken('********');
        }
        
      } else {
        console.log('No config data returned');
      }
    } catch (err) {
      setError('Failed to load YNAB configuration');
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    // Check if API token is the placeholder or empty when updating existing config
    if (config?.hasCredentials && apiToken === '********') {
      setError('Please enter your actual YNAB API token. For security reasons, we require you to re-enter it when updating settings.');
      return;
    }
    
    if (!apiToken || !budgetId || !accountId) {
      setError('All fields are required');
      return;
    }

    try {
      setSubmitting(true);
      setError('');
      setMessage('');

      await updateYNABConfig({
        apiToken,
        budgetId,
        accountId,
        syncFrequency
      });

      setMessage('YNAB configuration updated successfully! The system will now sync your categories in the background.');
      
      // Clear form fields after successful update
      setApiToken('');
      setBudgetId('');
      setAccountId('');
      
      // Reload the configuration
      await loadConfig();
    } catch (err) {
      setError('Failed to update YNAB configuration');
      console.error(err);
    } finally {
      setSubmitting(false);
    }
  };

  const handleSync = async () => {
    try {
      setSyncing(true);
      setError('');
      setMessage('');
      
      await syncYNABCategories();
      setMessage('YNAB categories synced successfully!');
      
      // Reload the configuration to get updated sync time
      await loadConfig();
    } catch (err) {
      setError('Failed to sync YNAB categories');
      console.error(err);
    } finally {
      setSyncing(false);
    }
  };

  if (loading) {
    return <div className="py-4 text-center">Loading YNAB configuration...</div>;
  }

  return (
    <div className="bg-white shadow rounded-lg p-6 mt-6">
      <h2 className="text-xl font-bold text-gray-900 mb-4">YNAB Integration</h2>
      
      {error && (
        <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4">
          {error}
          <button 
            onClick={() => setError('')}
            className="float-right font-bold"
          >
            &times;
          </button>
        </div>
      )}
      
      {message && (
        <div className="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded mb-4">
          {message}
          <button 
            onClick={() => setMessage('')}
            className="float-right font-bold"
          >
            &times;
          </button>
        </div>
      )}

      {config?.hasCredentials && (
        <div className="mb-6 p-4 bg-gray-50 rounded-lg">
          <h3 className="text-lg font-medium text-gray-900 mb-2">Current Configuration</h3>
          <p className="text-sm text-gray-600 mb-1">
            <span className="font-medium">Status:</span> {config.hasCredentials ? 'Configured' : 'Not Configured'}
          </p>
          <p className="text-sm text-gray-600 mb-1">
            <span className="font-medium">Last Sync:</span> {config.lastSyncTime ? new Date(config.lastSyncTime).toLocaleString() : 'Never'}
          </p>
          <p className="text-sm text-gray-600 mb-3">
            <span className="font-medium">Sync Frequency:</span> Every {config.syncFrequency} minutes
          </p>

          <button
            onClick={handleSync}
            disabled={syncing}
            className="inline-flex items-center px-3 py-1.5 border border-transparent text-xs rounded-md shadow-sm text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 disabled:bg-indigo-400"
          >
            {syncing ? 'Syncing...' : 'Sync Now'}
          </button>
        </div>
      )}

      <form onSubmit={handleSubmit}>
        <div className="mb-4">
          <label htmlFor="apiToken" className="block text-sm font-medium text-gray-700 mb-1">
            YNAB API Token <span className="text-red-600">*</span>
          </label>
          <div className="relative">
            <input
              id="apiToken"
              type={showApiToken ? "text" : "password"}
              value={apiToken}
              onChange={(e) => setApiToken(e.target.value)}
              placeholder={apiToken === '********' ? 'Re-enter your token to update' : 'Enter your YNAB API token'}
              className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm pr-20"
              required
            />
            <button
              type="button"
              onClick={(e) => {
                e.preventDefault();
                setShowApiToken(!showApiToken);
              }}
              className="absolute inset-y-0 right-0 mt-1 px-3 py-1 text-xs text-indigo-600 hover:text-indigo-800 bg-gray-50 border-l border-gray-300 rounded-r-md flex items-center"
            >
              {showApiToken ? 'Hide' : 'Show'}
            </button>
          </div>
          <p className="mt-1 text-xs text-gray-500">
            {apiToken === '********' 
              ? 'For security, you must re-enter your API token to update settings. The token is stored securely and encrypted in the database.'
              : 'Your API token is stored securely and encrypted in the database.'}
          </p>
        </div>

        <div className="mb-4">
          <label htmlFor="budgetId" className="block text-sm font-medium text-gray-700 mb-1">
            Budget ID <span className="text-red-600">*</span>
          </label>
          <input
            id="budgetId"
            type="text"
            value={budgetId}
            onChange={(e) => setBudgetId(e.target.value)}
            placeholder="Enter your YNAB budget ID"
            className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
            required
          />
        </div>

        <div className="mb-4">
          <label htmlFor="accountId" className="block text-sm font-medium text-gray-700 mb-1">
            Account ID <span className="text-red-600">*</span>
          </label>
          <input
            id="accountId"
            type="text"
            value={accountId}
            onChange={(e) => setAccountId(e.target.value)}
            placeholder="Enter your YNAB account ID"
            className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
            required
          />
        </div>

        <div className="mb-6">
          <label htmlFor="syncFrequency" className="block text-sm font-medium text-gray-700 mb-1">
            Sync Frequency (minutes)
          </label>
          <input
            id="syncFrequency"
            type="number"
            min="15"
            value={syncFrequency}
            onChange={(e) => setSyncFrequency(parseInt(e.target.value))}
            className="mt-1 block w-full border border-gray-300 rounded-md shadow-sm py-2 px-3 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
          />
          <p className="mt-1 text-xs text-gray-500">
            How often the system should sync with YNAB (minimum 15 minutes)
          </p>
        </div>

        <div className="flex justify-end">
          <button
            type="submit"
            disabled={submitting}
            className="inline-flex justify-center py-2 px-4 border border-transparent shadow-sm text-sm font-medium rounded-md text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 disabled:bg-indigo-400"
          >
            {submitting ? 'Saving...' : 'Save YNAB Configuration'}
          </button>
        </div>
      </form>
    </div>
  );
}

export default YNABConfig; 