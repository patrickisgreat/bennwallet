import axios, { InternalAxiosRequestConfig, AxiosError } from 'axios';
import { Transaction } from '../types/transaction';
import {
  Settlement,
  SettlementSummary,
  CreateSettlementRequest,
  ApplyTransactionRequest,
} from '../types/settlement';
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
  paidBy?: string;
  owedBy?: string;
  payTo?: string; // Deprecated but kept for compatibility
  paid?: boolean;
  paidDate?: string;
  enteredBy: string;
  optional?: boolean;
  note?: string;
  inSettlement?: boolean;
  categories?: Array<{
    id: string; // Backend expects string, not number
    name: string;
    description: string;
    color?: string;
    userId: string;
  }>;
}

// Convert the frontend Transaction type to the backend format
function toBackendTransaction(tx: Transaction): BackendTransaction {
  console.log('Debug - Converting frontend tx to backend:', JSON.stringify(tx, null, 2));
  console.log('Debug - Frontend note before conversion:', tx.note);

  const backendTx: BackendTransaction = {
    id: tx.id,
    amount: tx.amount,
    description: `${tx.owedBy ? tx.owedBy + ' - ' : ''}${tx.category}`, // Create description from owedBy and category
    date: tx.entered, // entered maps to backend 'date'
    transactionDate: tx.transactionDate, // transactionDate maps to backend 'transaction_date'
    type: tx.category,
    paidBy: tx.paidBy,
    owedBy: tx.owedBy,
    payTo: tx.payTo, // Keep for backward compatibility
    paid: tx.paid,
    paidDate: tx.paidDate,
    enteredBy: tx.enteredBy,
    optional: tx.optional,
    note: tx.note || '', // Keep note as the additional notes field
    categories: tx.categories?.map(cat => ({
      id: String(cat.id),
      name: cat.name,
      description: cat.description || '',
      color: cat.color,
      userId: String(cat.userId),
    })),
  };

  console.log('Debug - Backend note after conversion:', backendTx.note);
  console.log('Debug - Backend description after conversion:', backendTx.description);

  return backendTx;
}

// Convert the backend transaction format to the frontend format
function toFrontendTransaction(tx: BackendTransaction): Transaction {
  console.log('Debug - Converting backend tx to frontend:', JSON.stringify(tx, null, 2));
  console.log('Debug - Backend date field:', tx.date);
  console.log('Debug - Backend transaction_date field:', tx.transactionDate);

  const frontendTx: Transaction = {
    id: tx.id,
    entered: tx.date, // backend 'date' maps to entered
    transactionDate: tx.transactionDate || tx.date, // backend 'transaction_date' maps to transactionDate
    paidBy: tx.paidBy || '',
    owedBy: tx.owedBy || '',
    payTo: tx.payTo || '', // Keep for backward compatibility
    amount: tx.amount,
    note: tx.note || '',
    category: tx.type,
    paid: tx.paid || false,
    paidDate: tx.paidDate,
    enteredBy: tx.enteredBy || '',
    optional: tx.optional || false,
    inSettlement: tx.inSettlement || false,
    categories: tx.categories?.map(cat => ({
      id: cat.id,
      name: cat.name,
      description: cat.description,
      color: cat.color || '',
      userId: String(cat.userId),
    })),
  };

  console.log('Debug - Frontend entered field:', frontendTx.entered);
  console.log('Debug - Frontend transactionDate field:', frontendTx.transactionDate);

  return frontendTx;
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

export const fetchTransactions = async (
  params: Record<string, string | boolean | undefined> = {}
): Promise<Transaction[]> => {
  try {
    console.log('Fetching transactions with params:', params);

    // Build query string
    const queryParams = new URLSearchParams();
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined) {
        queryParams.set(key, String(value));
      }
    });

    const query = queryParams.toString();
    const queryString = query ? `?${query}` : '';

    console.log('Sending query params:', params);
    const response = await api.get<BackendTransaction[] | null>(`/transactions${queryString}`);

    console.log('Transactions response:', response.data);

    // Check if response is null or not an array, return empty array
    if (!response.data || !Array.isArray(response.data)) {
      console.log('API did not return an array:', response.data);
      return []; // Return empty array instead of throwing
    }

    // Debug note data in received transactions
    if (response.data && response.data.length > 0) {
      const sampleTx = response.data[0];
      console.log('Debug - Sample backend transaction data:', JSON.stringify(sampleTx, null, 2));
      console.log('Debug - Backend date field:', sampleTx.date);
      console.log('Debug - Backend transactionDate field:', sampleTx.transactionDate);
      // Log all raw properties to see what's available
      console.log('Debug - All properties in first transaction:', Object.keys(sampleTx));
    }

    // Transform backend transactions to frontend format
    return response.data.map(tx => toFrontendTransaction(tx));
  } catch (error) {
    if (axios.isAxiosError(error) && error.response) {
      console.log('API error response:', error.response);
    } else {
      console.log('Error fetching transactions:', error);
    }
    throw error;
  }
};

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
    console.log('Debug - Updating transaction:', id);
    console.log('Debug - Update payload:', JSON.stringify(updates, null, 2));

    // First fetch the existing transaction to get all fields
    const response = await api.get(`/transactions/${id}`);
    if (!response.data) {
      throw new Error('Transaction not found');
    }

    console.log('Debug - Existing transaction from API:', JSON.stringify(response.data, null, 2));

    // Convert backend to frontend format
    const existingTx = toFrontendTransaction(response.data);
    console.log(
      'Debug - Existing transaction after conversion:',
      JSON.stringify(existingTx, null, 2)
    );

    // Merge the updates with the existing transaction
    const mergedTx = { ...existingTx, ...updates };
    console.log('Debug - Merged transaction:', JSON.stringify(mergedTx, null, 2));

    // Update with the merged data
    const backendTx = toBackendTransaction(mergedTx);
    console.log(
      'Debug - Transaction converted back to backend format:',
      JSON.stringify(backendTx, null, 2)
    );

    await api.put(`/transactions/${id}`, backendTx);
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

export const fetchUniqueTransactionFields = async (): Promise<{
  payTo: string[];
  enteredBy: string[];
  category: string[];
}> => {
  try {
    console.log('Fetching unique transaction fields');
    const response = await api.get('/transactions/unique-fields');

    console.log('Raw unique fields response:', response.data);
    console.log('Entered By values received:', response.data.enteredBy);

    return {
      payTo: response.data.payTo || [],
      enteredBy: response.data.enteredBy || [],
      category: response.data.category || [],
    };
  } catch (error) {
    console.error('Error fetching unique transaction fields:', error);
    return {
      payTo: [],
      enteredBy: [],
      category: [],
    };
  }
};

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

// Add the User interface and fetchUsers function near the top of the file, after other interfaces are defined
export interface User {
  id: string;
  name: string;
  username: string;
  email: string;
  role?: string;
  createdAt?: string;
  updatedAt?: string;
}

// Add the fetchUsers function near the other API functions
export async function fetchUsers(): Promise<User[]> {
  try {
    console.log('Fetching users');
    const response = await api.get('/users');
    console.log('Users response:', response.data);

    if (Array.isArray(response.data)) {
      return response.data;
    } else {
      console.warn('Invalid response format from users API:', response.data);
      return []; // Return empty array instead of fallbacks
    }
  } catch (error) {
    const axiosError = error as AxiosError;
    console.error('Error fetching users:', axiosError);
    if (axiosError.response) {
      console.error('Response status:', axiosError.response.status);
      console.error('Response data:', axiosError.response.data);
    }
    return []; // Return empty array instead of fallbacks
  }
}

// Add a diagnostic function to check database health
export async function checkDatabaseHealth(): Promise<{
  status: string;
  tables: { [key: string]: number };
  errors?: string[];
}> {
  try {
    console.log('Checking database health...');
    const response = await api.get('/diagnostic/db-check');
    console.log('Database health response:', response.data);
    return response.data;
  } catch (error) {
    console.error('Error checking database health:', error);
    return {
      status: 'error',
      tables: {},
      errors: ['Failed to check database health'],
    };
  }
}

// Add the Category interface and fetchCategories function
export interface Category {
  id: string;
  name: string;
  description: string;
  color?: string;
  userId: string;
}

export async function fetchCategories(): Promise<Category[]> {
  try {
    console.log('Fetching categories from database');
    const response = await api.get('/categories');
    console.log('Categories response:', response.data);

    if (Array.isArray(response.data)) {
      return response.data;
    } else {
      console.warn('Invalid response format from categories API:', response.data);
      return []; // Return empty array instead of fallbacks
    }
  } catch (error) {
    const axiosError = error as AxiosError;
    console.error('Error fetching categories:', axiosError);
    if (axiosError.response) {
      console.error('Response status:', axiosError.response.status);
      console.error('Response data:', axiosError.response.data);
    }
    return []; // Return empty array instead of fallbacks
  }
}

export async function fetchCurrentUser(): Promise<{
  id: string;
  username: string;
  name: string;
  role: string;
  status?: string;
  isAdmin?: boolean;
} | null> {
  try {
    console.log('Fetching current user information from API');
    const response = await api.get('/user/me');
    console.log('Current user response:', response.data);
    return response.data;
  } catch (error) {
    console.error('Error fetching current user:', error);
    return null;
  }
}

// Permission API types and functions
export interface Permission {
  id: string;
  ownerUserId: string;
  grantedUserId: string;
  permissionType: string; // "read" or "write"
  resourceType: string; // "transactions", "categories", "ynab_config"
  createdAt: string;
  expiresAt?: string;
}

// Get permissions for a specific user
export async function fetchUserPermissions(userId?: string): Promise<Permission[]> {
  try {
    const queryParams = userId ? `?userId=${userId}` : '';
    const response = await api.get<Permission[]>(`/permissions${queryParams}`);
    return response.data || [];
  } catch (error) {
    console.error('Error fetching user permissions:', error);
    return [];
  }
}

// Get all permissions in the system (admin only)
export async function fetchAllPermissions(): Promise<Permission[]> {
  try {
    const response = await api.get<Permission[]>('/permissions/all');
    return response.data || [];
  } catch (error) {
    console.error('Error fetching all permissions:', error);
    return [];
  }
}

// Grant a permission to a user
export async function grantPermission(
  granteeId: string,
  resourceType: string,
  permissionType: string,
  expiresAt?: Date
): Promise<boolean> {
  try {
    await api.post('/permissions', {
      granteeId,
      resourceType,
      permissionType,
      expiresAt: expiresAt ? expiresAt.toISOString() : undefined,
    });
    return true;
  } catch (error) {
    console.error('Error granting permission:', error);
    return false;
  }
}

// Revoke a permission
export async function revokePermission(
  granteeId: string,
  ownerId: string,
  resourceType: string,
  permissionType: string
): Promise<boolean> {
  try {
    await api.delete('/permissions', {
      data: {
        granteeId,
        ownerId,
        resourceType,
        permissionType,
      },
    });
    return true;
  } catch (error) {
    console.error('Error revoking permission:', error);
    return false;
  }
}

// Settlement API
export const fetchAvailableSettlementTransactions = async (
  settlementId: string
): Promise<Transaction[]> => {
  try {
    const response = await api.get<Transaction[]>(
      `/settlements/${settlementId}/available-transactions`
    );
    return response.data;
  } catch (error) {
    console.error('Error fetching available settlement transactions:', error);
    throw new Error('Failed to fetch available transactions');
  }
};

export const updateSettlementStatus = async (
  settlementId: string,
  status: string,
  notes?: string
): Promise<Settlement> => {
  try {
    const response = await api.put<Settlement>(`/settlements/${settlementId}/status`, {
      status,
      notes,
    });
    return response.data;
  } catch (error) {
    console.error('Error updating settlement status:', error);
    throw new Error('Failed to update settlement status');
  }
};

// Settlement API functions
export async function createSettlement(data: CreateSettlementRequest): Promise<Settlement> {
  try {
    const response = await api.post<Settlement>('/settlements', data);
    return response.data;
  } catch (error) {
    console.error('Error creating settlement:', error);
    throw error;
  }
}

export async function fetchUserSettlements(status?: string): Promise<SettlementSummary[]> {
  try {
    const queryParams = status ? `?status=${status}` : '';
    const response = await api.get<SettlementSummary[]>(`/settlements${queryParams}`);
    return response.data || [];
  } catch (error) {
    console.error('Error fetching settlements:', error);
    return [];
  }
}

export async function fetchSettlement(id: string): Promise<Settlement | null> {
  try {
    const response = await api.get<Settlement>(`/settlements/${id}`);
    return response.data;
  } catch (error) {
    console.error('Error fetching settlement:', error);
    return null;
  }
}

export async function applyTransactionToSettlement(
  settlementId: string,
  data: ApplyTransactionRequest
): Promise<Settlement> {
  try {
    const response = await api.post<Settlement>(`/settlements/${settlementId}/apply`, data);
    return response.data;
  } catch (error) {
    console.error('Error applying transaction to settlement:', error);
    throw error;
  }
}

export async function removeTransactionFromSettlement(
  settlementId: string,
  transactionId: string
): Promise<Settlement> {
  try {
    const response = await api.delete<Settlement>(
      `/settlements/${settlementId}/transactions/${transactionId}`
    );
    return response.data;
  } catch (error) {
    console.error('Error removing transaction from settlement:', error);
    throw error;
  }
}

export async function fetchTransactionSettlements(transactionId: string): Promise<Settlement[]> {
  try {
    const response = await api.get<Settlement[]>(`/transactions/${transactionId}/settlements`);
    return response.data || [];
  } catch (error) {
    console.error('Error fetching transaction settlements:', error);
    return [];
  }
}

export async function applyTransactionAsPayment(
  transactionId: string,
  notes?: string
): Promise<Settlement> {
  try {
    const response = await api.post<Settlement>('/settlements/apply-payment', {
      transactionId,
      notes: notes || '',
    });
    return response.data;
  } catch (error) {
    console.error('Error applying transaction as payment:', error);
    throw error;
  }
}
