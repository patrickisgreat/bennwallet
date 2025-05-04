import axios, { InternalAxiosRequestConfig, AxiosError } from 'axios';
import { Transaction } from '../types/transaction';
import { auth } from '../firebase/firebase';

// Set the API base URL based on environment
const API_BASE_URL = import.meta.env.PROD
  ? window.location.origin // In production, API and frontend are on same domain
  : '/api'; // In development, use the Vite proxy which is configured in vite.config.ts

export const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Add a request interceptor to include the auth token in all requests
api.interceptors.request.use(async (config: InternalAxiosRequestConfig) => {
  try {
    // Get the current user and auth token
    const user = auth.currentUser;

    if (user) {
      // Get the ID token
      const token = await user.getIdToken();

      // Add the token to the Authorization header
      config.headers['Authorization'] = `Bearer ${token}`;
    }

    return config;
  } catch (error) {
    console.error('Error getting auth token:', error);
    return config;
  }
});

interface BackendTransaction {
  id: string;
  amount: number;
  description: string;
  date: string; // for entered date
  transactionDate: string; // for transaction date
  type: string;
  payTo?: string;
  paid?: boolean;
  paidDate?: string;
  enteredBy: string;
  optional?: boolean;
}

// Convert the frontend Transaction type to the backend format
function toBackendTransaction(tx: Transaction): BackendTransaction {
  return {
    id: tx.id,
    amount: tx.amount,
    description: tx.note,
    date: tx.entered,
    transactionDate: tx.transactionDate,
    type: tx.category,
    payTo: tx.payTo,
    paid: tx.paid,
    paidDate: tx.paidDate,
    enteredBy: tx.enteredBy,
    optional: tx.optional,
  };
}

// Convert the backend transaction format to the frontend format
function toFrontendTransaction(tx: BackendTransaction): Transaction {
  return {
    id: tx.id,
    entered: tx.date,
    transactionDate: tx.transactionDate || tx.date, // Fall back to entered date if transaction date not available
    payTo: (tx.payTo as 'Sarah' | 'Patrick') || 'Sarah',
    amount: tx.amount,
    note: tx.description,
    category: tx.type,
    paid: tx.paid || false,
    paidDate: tx.paidDate,
    enteredBy: tx.enteredBy as 'Sarah' | 'Patrick',
    optional: tx.optional || false,
  };
}

// Define explicit types for transaction filters
export interface TransactionFilterParams {
  startDate?: string;
  endDate?: string;
  txStartDate?: string;
  txEndDate?: string;
  payTo?: string;
  enteredBy?: string;
  paid?: boolean;
}

export async function fetchTransactions(params?: TransactionFilterParams): Promise<Transaction[]> {
  try {
    console.log('Fetching transactions with params:', params);

    // Create a new object for query parameters that can have string values
    const queryParams: Record<string, string | undefined> = {};

    // Copy all params to the new object, converting as needed
    if (params) {
      Object.entries(params).forEach(([key, value]) => {
        if (value !== undefined) {
          if (typeof value === 'boolean') {
            queryParams[key] = value ? 'true' : 'false';
          } else {
            queryParams[key] = value;
          }
        }
      });
    }

    console.log('Sending query params:', queryParams);
    const response = await api.get('/transactions', { params: queryParams });
    console.log('Transactions response:', response.data);
    return Array.isArray(response.data) ? response.data.map(toFrontendTransaction) : [];
  } catch (error) {
    console.error('Error fetching transactions:', error);
    return [];
  }
}

export async function createTransaction(transaction: Transaction): Promise<boolean> {
  try {
    await api.post('/transactions', toBackendTransaction(transaction));
    return true;
  } catch (error) {
    console.error('Error creating transaction:', error);
    return false;
  }
}

export async function updateTransaction(
  id: string,
  updates: Partial<Transaction>
): Promise<boolean> {
  try {
    // First fetch the existing transaction to get all fields
    const response = await api.get(`/transactions/${id}`);
    if (!response.data) {
      throw new Error('Transaction not found');
    }

    // Convert backend to frontend format
    const existingTx = toFrontendTransaction(response.data);

    // Merge the updates with the existing transaction
    const mergedTx = { ...existingTx, ...updates };

    // Update with the merged data
    await api.put(`/transactions/${id}`, toBackendTransaction(mergedTx));
    return true;
  } catch (error) {
    console.error('Error updating transaction:', error);
    return false;
  }
}

export async function deleteTransaction(id: string): Promise<boolean> {
  try {
    await api.delete(`/transactions/${id}`);
    return true;
  } catch (error) {
    console.error('Error deleting transaction:', error);
    return false;
  }
}

export interface ReportFilter {
  startDate: string;
  endDate: string;
  category?: string;
  payTo?: string;
  enteredBy?: string;
  paid?: boolean;
  optional?: boolean;
  transactionDateMonth?: number;
  transactionDateYear?: number;
}

export interface CategoryTotal {
  category: string;
  total: number;
}

export interface YNABSyncRequest {
  userId: string;
  date: string;
  payeeName: string;
  memo: string;
  categories: {
    categoryName: string;
    amount: number;
  }[];
}

export interface YNABConfig {
  id?: number;
  userId: string;
  apiToken?: string;
  budgetId?: string;
  accountId?: string;
  lastSyncTime?: string;
  syncFrequency: number;
  hasCredentials: boolean;
  createdAt?: string;
  updatedAt?: string;
}

export async function fetchYNABSplits(filter: ReportFilter): Promise<CategoryTotal[]> {
  try {
    console.log('Raw filter sent to API:', filter);
    console.log('Filter values: ', {
      startDate: filter.startDate,
      endDate: filter.endDate,
      category: filter.category,
      payTo: filter.payTo,
      enteredBy: filter.enteredBy,
      paid: filter.paid,
      optional: filter.optional,
    });

    const userId = localStorage.getItem('userId');
    if (!userId) {
      console.warn('No userId found in localStorage - user may not be fully authenticated yet');
      return [];
    }

    // Format dates to ensure they're in the expected format for SQLite (YYYY-MM-DD)
    const formatDate = (dateStr?: string) => {
      if (!dateStr) return undefined;
      try {
        const date = new Date(dateStr);
        // Simple YYYY-MM-DD format that matches our SQLite dates
        return date.toISOString().split('T')[0]; // Format as YYYY-MM-DD
      } catch {
        console.warn('Invalid date format:', dateStr);
        return undefined;
      }
    };

    // Convert month and year to integers or null
    const parseIntOrNull = (value: string | number | undefined | null): number | null => {
      if (value === '' || value === undefined || value === null) return null;
      const parsed = parseInt(String(value), 10);
      return isNaN(parsed) ? null : parsed;
    };

    // Send request to the API
    const requestBody = {
      startDate: formatDate(filter.startDate),
      endDate: formatDate(filter.endDate),
      category: filter.category || null,
      payTo: filter.payTo || null,
      enteredBy: filter.enteredBy || null,
      paid: filter.paid,
      optional: filter.optional,
      transactionDateMonth: parseIntOrNull(filter.transactionDateMonth),
      transactionDateYear: parseIntOrNull(filter.transactionDateYear),
      userId: userId, // Add userId to the request
    };

    console.log('Final request body sent to API:', JSON.stringify(requestBody, null, 2));

    // Use POST method with explicit headers and body
    const response = await api.post('/reports/ynab-splits', requestBody, {
      headers: {
        'Content-Type': 'application/json',
      },
    });
    console.log('Raw response from API:', response);
    console.log('Response data:', JSON.stringify(response.data, null, 2));

    if (!response.data) {
      console.log('API returned null or undefined');
      return [];
    }

    // Ensure we're returning an array
    return Array.isArray(response.data) ? response.data : [];
  } catch (error: Error | unknown) {
    console.error('Error fetching YNAB splits from API:', error);
    if (error && typeof error === 'object' && 'response' in error) {
      // The request was made and the server responded with a status code
      // that falls out of the range of 2xx
      const axiosError = error as { response: { data: unknown; status: number; headers: unknown } };
      console.error('Response data:', axiosError.response.data);
      console.error('Response status:', axiosError.response.status);
      console.error('Response headers:', axiosError.response.headers);
    }

    throw error; // Propagate error to caller
  }
}

// Add this function to sync splits to YNAB
export async function syncToYNAB(request: YNABSyncRequest): Promise<void> {
  try {
    const response = await api.post('/ynab/sync', request, {
      headers: {
        'Content-Type': 'application/json',
      },
    });

    if (response.status !== 200) {
      throw new Error(`YNAB sync failed with status ${response.status}`);
    }

    return;
  } catch (error) {
    console.error('Error syncing to YNAB:', error);
    throw error;
  }
}

export async function fetchYNABConfig(): Promise<YNABConfig | null> {
  try {
    const response = await api.get('/ynab/config');
    console.log('Raw YNAB config response:', response);
    console.log('YNAB config response data:', response.data);
    return response.data;
  } catch (error) {
    console.error('Error fetching YNAB configuration:', error);
    return null;
  }
}

export async function updateYNABConfig(config: {
  apiToken: string;
  budgetId: string;
  accountId: string;
  syncFrequency?: number;
}): Promise<boolean> {
  try {
    await api.put('/ynab/config', config);
    return true;
  } catch (error) {
    console.error('Error updating YNAB configuration:', error);
    throw error;
  }
}

export async function syncYNABCategories(): Promise<boolean> {
  try {
    await api.post('/ynab/sync/categories');
    return true;
  } catch (error) {
    console.error('Error syncing YNAB categories:', error);
    return false;
  }
}

export interface UniqueTransactionFields {
  payTo: string[];
  enteredBy: string[];
}

export async function fetchUniqueTransactionFields(): Promise<UniqueTransactionFields> {
  try {
    console.log('API base URL:', api.defaults.baseURL);
    const url = '/transactions/unique-fields';
    console.log('Fetching unique fields from:', url);
    const response = await api.get(url);
    console.log('Unique fields response:', response.data);
    return response.data;
  } catch (error) {
    const axiosError = error as AxiosError;
    console.error('Error fetching unique transaction fields:', axiosError);
    if (axiosError.response) {
      console.error('Response status:', axiosError.response.status);
      console.error('Response data:', axiosError.response.data);
    }
    return { payTo: [], enteredBy: [] };
  }
}

export interface YNABCategory {
  id: string;
  name: string;
  categoryGroupID: string;
  categoryGroupName: string;
}

export interface CategoryGroup {
  id: string;
  name: string;
  categories: YNABCategory[];
}

export async function fetchYNABCategories(): Promise<CategoryGroup[]> {
  console.log('📋 BEGIN fetchYNABCategories');
  try {
    const userId = localStorage.getItem('userId');
    console.log('📋 User ID from localStorage:', userId);

    if (!userId) {
      console.warn('📋 No userId found in localStorage - user may not be fully authenticated');
      return [];
    }

    const url = `/ynab/categories?userId=${userId}`;
    console.log('📋 Fetching categories from URL:', url);
    console.log('📋 API base URL:', api.defaults.baseURL);

    console.log(
      '📋 Making request with headers:',
      JSON.stringify({
        Authorization: 'Bearer ***', // Not showing actual token for security
      })
    );

    try {
      const response = await api.get(url);
      console.log('📋 Categories response status:', response.status);
      console.log('📋 Raw categories response:', response);
      console.log('📋 Categories data type:', typeof response.data);
      console.log('📋 Categories data is array?', Array.isArray(response.data));
      console.log(
        '📋 Categories data length:',
        Array.isArray(response.data) ? response.data.length : 'N/A'
      );

      // Log first item if exists, for debugging
      if (Array.isArray(response.data) && response.data.length > 0) {
        console.log('📋 First category group sample:', JSON.stringify(response.data[0]));
      }

      if (!response.data || !Array.isArray(response.data)) {
        console.warn('📋 Invalid or empty YNAB categories response');
        return [];
      }

      console.log('📋 END fetchYNABCategories - Success');
      return response.data;
    } catch (error: unknown) {
      const requestError = error as AxiosError;
      console.error('📋 Request error details:', requestError);
      if (requestError.response) {
        console.error('📋 Response status:', requestError.response.status);
        console.error('📋 Response data:', requestError.response.data);
      }
      throw error;
    }
  } catch (error) {
    console.error('📋 Error fetching YNAB categories:', error);
    console.error('📋 END fetchYNABCategories - Failed');
    return [];
  }
}
