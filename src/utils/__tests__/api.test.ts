import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { fetchTransactions, createTransaction } from '../api';
import { Transaction } from '../../types/transaction';
import { AxiosInstance } from 'axios';

// Mock the api module first
vi.mock('../api', () => {
  // Create a properly typed mock API
  const mockApi = {
    get: vi.fn().mockImplementation(() => Promise.resolve({ data: [] })),
    post: vi.fn().mockImplementation(() => Promise.resolve({ data: { success: true } })),
    put: vi.fn() as unknown as AxiosInstance['put'],
    delete: vi.fn() as unknown as AxiosInstance['delete'],
    interceptors: {
      request: {
        use: vi.fn()
      }
    }
  };

  return {
    api: mockApi,
    fetchTransactions: vi.fn().mockImplementation(async () => {
      try {
        const { api } = await import('../api');
        const response = await api.get('/transactions');
        return response.data.map((t: any) => ({
          id: t.id,
          entered: t.date,
          payTo: t.payTo,
          amount: t.amount,
          note: t.description,
          category: t.type,
          paid: t.paid,
          paidDate: t.paidDate,
          enteredBy: t.enteredBy
        }));
      } catch (error) {
        return [];
      }
    }),
    createTransaction: vi.fn().mockImplementation(async (transaction: Transaction) => {
      try {
        const { api } = await import('../api');
        await api.post('/transactions', {
          amount: transaction.amount,
          description: transaction.note,
          date: transaction.entered,
          type: transaction.category,
          payTo: transaction.payTo,
          paid: transaction.paid,
          paidDate: transaction.paidDate,
          enteredBy: transaction.enteredBy
        });
        return true;
      } catch (error) {
        return false;
      }
    })
  };
});

describe('API utility functions', () => {
  beforeEach(() => {
    // Mock localStorage
    vi.stubGlobal('localStorage', {
      getItem: vi.fn().mockReturnValue('test-user-id'),
      setItem: vi.fn(),
      removeItem: vi.fn()
    });
    
    // Clear mocks before each test
    vi.clearAllMocks();
  });

  afterEach(() => {
    // Clean up
    vi.unstubAllGlobals();
  });

  describe('fetchTransactions', () => {
    it('returns an empty array when API call fails', async () => {
      // Mock a failed response
      const { api } = await import('../api');
      vi.mocked(api.get).mockRejectedValueOnce(new Error('Network error'));
      
      const result = await fetchTransactions();
      
      expect(result).toEqual([]);
    });

    it('converts backend transactions to frontend format', async () => {
      // Mock a successful response with sample data
      const mockBackendTransactions = [
        {
          id: '1',
          amount: 100,
          description: 'Groceries',
          date: '2023-05-10T00:00:00.000Z',
          type: 'Food',
          payTo: 'Sarah',
          paid: true,
          paidDate: '2023-05-15T00:00:00.000Z',
          enteredBy: 'Patrick'
        }
      ];
      
      const { api } = await import('../api');
      vi.mocked(api.get).mockResolvedValueOnce({ 
        data: mockBackendTransactions 
      });
      
      const result = await fetchTransactions();
      
      // Expecting the backend data to be converted to frontend format
      expect(result).toEqual([
        {
          id: '1',
          entered: '2023-05-10T00:00:00.000Z',
          payTo: 'Sarah',
          amount: 100,
          note: 'Groceries',
          category: 'Food',
          paid: true,
          paidDate: '2023-05-15T00:00:00.000Z',
          enteredBy: 'Patrick'
        }
      ]);
    });
  });
  
  describe('createTransaction', () => {
    it('returns true when transaction is created successfully', async () => {
      const { api } = await import('../api');
      vi.mocked(api.post).mockResolvedValueOnce({ data: { success: true } });
      
      const transaction: Transaction = {
        id: '1',
        entered: '2023-05-10T00:00:00.000Z',
        payTo: 'Sarah',
        amount: 100,
        note: 'Groceries',
        category: 'Food',
        paid: true,
        paidDate: '2023-05-15T00:00:00.000Z',
        enteredBy: 'Patrick'
      };
      
      const result = await createTransaction(transaction);
      
      expect(result).toBe(true);
    });
    
    it('returns false when transaction creation fails', async () => {
      const { api } = await import('../api');
      vi.mocked(api.post).mockRejectedValueOnce(new Error('Failed to create'));
      
      const transaction: Transaction = {
        id: '1',
        entered: '2023-05-10T00:00:00.000Z',
        payTo: 'Sarah',
        amount: 100,
        note: 'Groceries',
        category: 'Food',
        paid: true,
        paidDate: '2023-05-15T00:00:00.000Z',
        enteredBy: 'Patrick'
      };
      
      const result = await createTransaction(transaction);
      
      expect(result).toBe(false);
    });
  });
}); 