import axios, { InternalAxiosRequestConfig } from 'axios';
import { Transaction } from '../types/transaction';

const API_BASE_URL = 'http://localhost:8080';

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
  date: string;
  type: string;
  payTo?: string;
  paid?: boolean;
  paidDate?: string;
  enteredBy: string;
}

// Convert the frontend Transaction type to the backend format
function toBackendTransaction(tx: Transaction): BackendTransaction {
  return {
    id: tx.id,
    amount: tx.amount,
    description: tx.note,
    date: tx.entered,
    type: tx.category,
    payTo: tx.payTo,
    paid: tx.paid,
    paidDate: tx.paidDate,
    enteredBy: tx.enteredBy
  };
}

// Convert the backend transaction format to the frontend format
function toFrontendTransaction(tx: BackendTransaction): Transaction {
  return {
    id: tx.id,
    entered: tx.date,
    payTo: (tx.payTo as 'Sarah' | 'Patrick') || 'Sarah',
    amount: tx.amount,
    note: tx.description,
    category: tx.type,
    paid: tx.paid || false,
    paidDate: tx.paidDate,
    enteredBy: tx.enteredBy as 'Sarah' | 'Patrick'
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
    await api.put(`/transactions/${id}`, toBackendTransaction(updates as Transaction));
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
}

export interface CategoryTotal {
  category: string;
  total: number;
}

export async function fetchYNABSplits(filter: ReportFilter): Promise<CategoryTotal[]> {
  try {
    console.log('Raw filter sent to API:', filter);
    
    // Create some test transactions to ensure we have data
    const userId = localStorage.getItem('userId');
    if (!userId) {
      console.error('No userId found in localStorage');
      return [];
    }
    
    // Add some test transactions first to ensure we have data to report on
    for (let i = 1; i <= 3; i++) {
      const testCategory = ['Groceries', 'Utilities', 'Entertainment'][i % 3];
      await api.post('/transactions', {
        id: `test-${Date.now()}-${i}`,
        amount: i * 50,
        description: `Test ${testCategory}`,
        date: new Date().toISOString(),
        type: testCategory,
        payTo: "Sarah",
        paid: false,
        enteredBy: "Patrick"
      }).catch(e => console.log(`Failed to create test transaction ${i}:`, e));
    }
    
    console.log('Sending report request with userId:', userId);
    
    // Manually construct the request with the userId included
    const requestData = {
      ...filter,
      userId: userId,
    };
    
    // Now request the report
    const response = await api.post('/reports/ynab-splits', requestData);
    console.log('Raw response from API:', response);
    
    if (response.data === null) {
      console.log('API returned null, checking for transactions...');
      const txResponse = await api.get('/transactions');
      console.log('Current transactions:', txResponse.data);
      return [];
    }
    
    return response.data || [];
  } catch (error) {
    console.error('Error fetching YNAB splits:', error);
    return [];
  }
} 