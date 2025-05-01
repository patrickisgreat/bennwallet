import axios, { InternalAxiosRequestConfig } from 'axios';
import { Transaction } from '../types/transaction';

// Set the API base URL based on environment
const API_BASE_URL = import.meta.env.PROD 
  ? window.location.origin  // In production, API and frontend are on same domain
  : 'http://localhost:8080'; // In development, use dedicated backend port

export const api = axios.create({
    baseURL: API_BASE_URL,
    headers: {
        'Content-Type': 'application/json',
    },
});

// Add request interceptor to include user ID in all requests
api.interceptors.request.use((config: InternalAxiosRequestConfig) => {
    const userId = localStorage.getItem('userId');
    // Only add userId for authenticated endpoints
    if (userId && !config.url?.includes('/users/')) {
        config.params = {
            ...config.params,
            userId,
        };
    }
    return config;
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
    optional: tx.optional
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
    optional: tx.optional || false
  };
}

// Define explicit types for transaction filters
export interface TransactionFilterParams {
  startDate?: string;
  endDate?: string;
  payTo?: string;
  enteredBy?: string;
  category?: string;
  paid?: boolean;
}

export async function fetchTransactions(params?: TransactionFilterParams): Promise<Transaction[]> {
  try {
    const response = await api.get('/transactions', { params });
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

export async function updateTransaction(id: string, updates: Partial<Transaction>): Promise<boolean> {
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
  startDate?: string;
  endDate?: string;
  category?: string;
  payTo?: string;
  enteredBy?: string;
  paid?: boolean;
  optional?: boolean;
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

export async function fetchYNABSplits(filter: ReportFilter): Promise<CategoryTotal[]> {
  try {
    console.log('Raw filter sent to API:', filter);
    
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
      } catch (e) {
        console.warn('Invalid date format:', dateStr);
        return undefined;
      }
    };
    
    // Send request to the API
    const requestBody = {
      startDate: formatDate(filter.startDate),
      endDate: formatDate(filter.endDate),
      category: filter.category || null,
      payTo: filter.payTo || null,
      enteredBy: filter.enteredBy || null,
      paid: filter.paid
    };
    
    console.log('Sending POST request with body:', requestBody);
    
    // Use POST method with explicit headers and body
    const response = await api.post('/reports/ynab-splits', requestBody, {
      headers: {
        'Content-Type': 'application/json'
      }
    });
    console.log('Raw response from API:', response);
    
    if (!response.data) {
      console.log('API returned null or undefined');
      return [];
    }
    
    // Ensure we're returning an array
    return Array.isArray(response.data) ? response.data : [];
  } catch (error: any) {
    console.error('Error fetching YNAB splits from API:', error);
    if (error.response) {
      // The request was made and the server responded with a status code
      // that falls out of the range of 2xx
      console.error('Response data:', error.response.data);
      console.error('Response status:', error.response.status);
      console.error('Response headers:', error.response.headers);
    }
    
    throw error; // Propagate error to caller
  }
}

// Add this function to sync splits to YNAB
export async function syncToYNAB(request: YNABSyncRequest): Promise<void> {
  try {
    const response = await api.post('/ynab/sync', request, {
      headers: {
        'Content-Type': 'application/json'
      }
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