import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import TransactionsPage from '../TransactionsPage';
import * as api from '../../utils/api';
import { AuthProvider } from '../../context/AuthContext';
import { UserProvider } from '../../context/UserContext';

// Mock the API module
vi.mock('../../utils/api');

// Mock firebase auth
vi.mock('../../firebase/firebase', () => ({
  auth: {},
  db: {},
  googleProvider: {},
}));

// Mock firebase/auth
vi.mock('firebase/auth', () => ({
  onAuthStateChanged: vi.fn((auth, callback) => {
    callback({ uid: 'test-user-id' });
    return vi.fn();
  }),
  signOut: vi.fn(),
  signInWithEmailAndPassword: vi.fn(),
  createUserWithEmailAndPassword: vi.fn(),
  sendPasswordResetEmail: vi.fn(),
  sendEmailVerification: vi.fn(),
  updateProfile: vi.fn(),
  signInWithPopup: vi.fn(),
}));

const mockUser = {
  id: 'test-user-id',
  username: 'patrick',
  name: 'Patrick Bennett',
  email: 'patrick@test.com',
  isAdmin: false,
};

const mockTransactions = [
  {
    id: '1',
    amount: 100,
    date: '2025-05-28',
    enteredDate: '2025-05-28',
    enteredBy: 'Patrick Bennett',
    paidBy: 'Patrick Bennett',
    owedBy: 'Sarah Wallis',
    category: 'Food',
    categories: [],
    note: 'Dinner',
    paid: false,
    paidDate: null,
    optional: false,
    type: 'personal',
  },
  {
    id: '2',
    amount: 50,
    date: '2025-05-28',
    enteredDate: '2025-05-28',
    enteredBy: 'Sarah Wallis',
    paidBy: 'Sarah Wallis',
    owedBy: 'Patrick Bennett',
    category: 'Transport',
    categories: [],
    note: 'Uber',
    paid: false,
    paidDate: null,
    optional: false,
    type: 'personal',
  },
  {
    id: '3',
    amount: 75,
    date: '2025-05-28',
    enteredDate: '2025-05-28',
    enteredBy: 'Kim Donaldson',
    paidBy: 'Patrick Bennett',
    owedBy: 'Kim Donaldson',
    category: 'Entertainment',
    categories: [],
    note: 'Concert tickets',
    paid: false,
    paidDate: null,
    optional: false,
    type: 'personal',
  },
];

const mockUniqueFields = {
  payTo: ['Patrick Bennett', 'Sarah Wallis', 'Kim Donaldson'],
  enteredBy: ['Patrick Bennett', 'Sarah Wallis', 'Kim Donaldson'],
  category: ['Food', 'Transport', 'Entertainment'],
};

const renderTransactionsPage = () => {
  return render(
    <MemoryRouter>
      <AuthProvider>
        <UserProvider>
          <TransactionsPage />
        </UserProvider>
      </AuthProvider>
    </MemoryRouter>
  );
};

describe('TransactionsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    localStorage.setItem('userId', 'test-user-id');

    // Setup default mocks
    vi.mocked(api.fetchCurrentUser).mockResolvedValue(mockUser);
    vi.mocked(api.fetchTransactions).mockResolvedValue(mockTransactions);
    vi.mocked(api.fetchUniqueTransactionFields).mockResolvedValue(mockUniqueFields);
    vi.mocked(api.createTransaction).mockResolvedValue(true);
    vi.mocked(api.updateTransaction).mockResolvedValue(true);
    vi.mocked(api.deleteTransaction).mockResolvedValue(true);
    vi.mocked(api.fetchCategories).mockResolvedValue([]);
    vi.mocked(api.fetchYNABCategories).mockResolvedValue([]);
  });

  describe('View Mode Filtering', () => {
    it('should send correct filters when "What Others Owe Me" is clicked', async () => {
      renderTransactionsPage();

      // Wait for initial load
      await waitFor(() => {
        expect(screen.getByText('What Others Owe Me')).toBeInTheDocument();
      });

      // Click "What Others Owe Me" button
      fireEvent.click(screen.getByText('What Others Owe Me'));

      // Wait for the API call
      await waitFor(() => {
        // Check that fetchTransactions was called with correct parameters
        const calls = vi.mocked(api.fetchTransactions).mock.calls;
        const lastCall = calls[calls.length - 1];
        const params = lastCall[0];

        // Critical test: should NOT include enteredBy when paidBy is set
        expect(params).toHaveProperty('paidBy', 'Patrick Bennett');
        expect(params).not.toHaveProperty('enteredBy');
      });
    });

    it('should send correct filters when "What I Owe" is clicked', async () => {
      renderTransactionsPage();

      await waitFor(() => {
        expect(screen.getByText('What I Owe')).toBeInTheDocument();
      });

      // The default view is "What I Owe", so check initial call
      await waitFor(() => {
        const calls = vi.mocked(api.fetchTransactions).mock.calls;
        const lastCall = calls[calls.length - 1];
        const params = lastCall[0];

        expect(params).toHaveProperty('owedBy', 'Patrick Bennett');
      });
    });

    it('should filter out transactions where owedBy equals current user in othersOwe mode', async () => {
      renderTransactionsPage();

      await waitFor(() => {
        expect(screen.getByText('What Others Owe Me')).toBeInTheDocument();
      });

      // Click "What Others Owe Me"
      fireEvent.click(screen.getByText('What Others Owe Me'));

      await waitFor(() => {
        // Should show transaction where Patrick paid and others owe
        expect(screen.getByText('Concert tickets')).toBeInTheDocument(); // Patrick paid, Kim owes

        // Should NOT show transaction where Patrick owes
        expect(screen.queryByText('Uber')).not.toBeInTheDocument(); // Sarah paid, Patrick owes
      });
    });

    it('should not combine conflicting filters', async () => {
      renderTransactionsPage();

      await waitFor(() => {
        expect(screen.getByText('What Others Owe Me')).toBeInTheDocument();
      });

      // Set enteredBy filter first (labeled as "Owed To" in UI)
      // Find by looking for the select element with name="enteredBy"
      const enteredBySelect = document.querySelector('select[name="enteredBy"]');
      if (!enteredBySelect) throw new Error('Could not find enteredBy select');
      fireEvent.change(enteredBySelect, { target: { value: 'Sarah Wallis' } });

      // Then click "What Others Owe Me"
      fireEvent.click(screen.getByText('What Others Owe Me'));

      await waitFor(() => {
        const calls = vi.mocked(api.fetchTransactions).mock.calls;
        const lastCall = calls[calls.length - 1];
        const params = lastCall[0];

        // Should have paidBy but NOT enteredBy to avoid conflicts
        expect(params).toHaveProperty('paidBy', 'Patrick Bennett');
        expect(params).not.toHaveProperty('enteredBy');
      });
    });
  });

  describe('API Parameter Construction', () => {
    it('should construct correct parameters for each view mode', async () => {
      renderTransactionsPage();

      await waitFor(() => {
        expect(screen.getByText('What I Owe')).toBeInTheDocument();
      });

      // Test "What I Owe" mode (default)
      let calls = vi.mocked(api.fetchTransactions).mock.calls;
      expect(calls[calls.length - 1][0]).toMatchObject({
        owedBy: 'Patrick Bennett',
      });

      // Test "What Others Owe Me" mode
      fireEvent.click(screen.getByText('What Others Owe Me'));
      await waitFor(() => {
        calls = vi.mocked(api.fetchTransactions).mock.calls;
        expect(calls[calls.length - 1][0]).toMatchObject({
          paidBy: 'Patrick Bennett',
        });
        expect(calls[calls.length - 1][0]).not.toHaveProperty('enteredBy');
      });
    });
  });

  describe('Filter Controls', () => {
    it('should reset view mode when Clear All is clicked', async () => {
      renderTransactionsPage();

      await waitFor(() => {
        expect(screen.getByText('What Others Owe Me')).toBeInTheDocument();
      });

      // Click "What Others Owe Me" to change view mode
      fireEvent.click(screen.getByText('What Others Owe Me'));

      // Verify we're in "othersOwe" mode
      await waitFor(() => {
        const calls = vi.mocked(api.fetchTransactions).mock.calls;
        const lastCall = calls[calls.length - 1];
        expect(lastCall[0]).toHaveProperty('paidBy', 'Patrick Bennett');
      });

      // Click "Clear All"
      fireEvent.click(screen.getByText('Clear All'));

      // Should reset to showing all transactions (no specific filters)
      await waitFor(() => {
        const calls = vi.mocked(api.fetchTransactions).mock.calls;
        const lastCall = calls[calls.length - 1];
        const params = lastCall[0];

        // Should not have specific user filters
        expect(params).not.toHaveProperty('owedBy');
        expect(params).not.toHaveProperty('paidBy');
      });
    });

    it('should reset view mode when both dropdowns are set to All', async () => {
      renderTransactionsPage();

      await waitFor(() => {
        expect(screen.getByText('What Others Owe Me')).toBeInTheDocument();
      });

      // Click "What Others Owe Me" to change view mode
      fireEvent.click(screen.getByText('What Others Owe Me'));

      // Set "Owed By" dropdown to a specific user
      const owedBySelect = document.querySelector('select[name="payTo"]');
      if (!owedBySelect) throw new Error('Could not find payTo select');
      fireEvent.change(owedBySelect, { target: { value: 'Sarah Wallis' } });

      // Set "Owed To" dropdown to a specific user
      const owedToSelect = document.querySelector('select[name="enteredBy"]');
      if (!owedToSelect) throw new Error('Could not find enteredBy select');
      fireEvent.change(owedToSelect, { target: { value: 'Kim Donaldson' } });

      // Now set both to "All"
      fireEvent.change(owedBySelect, { target: { value: '' } });
      fireEvent.change(owedToSelect, { target: { value: '' } });

      // Should reset to showing all transactions
      await waitFor(() => {
        const calls = vi.mocked(api.fetchTransactions).mock.calls;
        const lastCall = calls[calls.length - 1];
        const params = lastCall[0];

        // Should not have specific user filters
        expect(params).not.toHaveProperty('owedBy');
        expect(params).not.toHaveProperty('paidBy');
        expect(params).not.toHaveProperty('payTo');
        expect(params).not.toHaveProperty('enteredBy');
      });
    });

    it('should filter unpaid transactions when checkbox is checked', async () => {
      renderTransactionsPage();

      await waitFor(() => {
        expect(screen.getByText('Only Show Unpaid')).toBeInTheDocument();
      });

      // Find and check the "Only Show Unpaid" checkbox
      const checkbox = screen.getByRole('checkbox', { name: /only show unpaid/i });
      fireEvent.click(checkbox);

      // Should add paid: false filter
      await waitFor(() => {
        const calls = vi.mocked(api.fetchTransactions).mock.calls;
        const lastCall = calls[calls.length - 1];
        expect(lastCall[0]).toHaveProperty('paid', false);
      });

      // Uncheck the checkbox
      fireEvent.click(checkbox);

      // Should remove the paid filter
      await waitFor(() => {
        const calls = vi.mocked(api.fetchTransactions).mock.calls;
        const lastCall = calls[calls.length - 1];
        expect(lastCall[0]).not.toHaveProperty('paid');
      });
    });

    it('should include unpaid filter when clearing filters if checkbox was checked', async () => {
      renderTransactionsPage();

      await waitFor(() => {
        expect(screen.getByText('Only Show Unpaid')).toBeInTheDocument();
      });

      // Check the "Only Show Unpaid" checkbox
      const checkbox = screen.getByRole('checkbox', { name: /only show unpaid/i });
      fireEvent.click(checkbox);

      // Set some other filters
      const owedBySelect = document.querySelector('select[name="payTo"]');
      if (!owedBySelect) throw new Error('Could not find payTo select');
      fireEvent.change(owedBySelect, { target: { value: 'Sarah Wallis' } });

      // Click "Clear All"
      fireEvent.click(screen.getByText('Clear All'));

      // Should clear all filters including the paid checkbox
      await waitFor(() => {
        const calls = vi.mocked(api.fetchTransactions).mock.calls;
        const lastCall = calls[calls.length - 1];
        const params = lastCall[0];

        // Should not have any filters
        expect(params).not.toHaveProperty('paid');
        expect(params).not.toHaveProperty('payTo');
        expect(params).not.toHaveProperty('owedBy');
        expect(params).not.toHaveProperty('paidBy');
      });

      // Checkbox should be unchecked
      expect(checkbox).not.toBeChecked();
    });
  });
});
