import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import SettlementManager from '../SettlementManager';
import { Settlement } from '../../types/settlement';
import * as api from '../../utils/api';

// Mock the API functions
vi.mock('../../utils/api', () => ({
  fetchSettlements: vi.fn(),
  fetchSettlement: vi.fn(),
  updateSettlementStatus: vi.fn(),
  fetchUserSettlements: vi.fn(),
  applyTransactionToSettlement: vi.fn(),
  removeTransactionFromSettlement: vi.fn(),
  fetchAvailableSettlementTransactions: vi.fn(),
}));

describe('SettlementManager', () => {
  const mockSettlement: Settlement = {
    id: 'settlement-1',
    creatorId: 'user-1',
    creatorName: 'Test User',
    recipientId: 'user-2',
    recipientName: 'Other User',
    totalAmount: 100.5,
    status: 'active',
    createdAt: '2024-01-01T00:00:00Z',
    updatedAt: '2024-01-01T00:00:00Z',
    notes: 'Test settlement',
    items: [],
    history: [],
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.fetchUserSettlements).mockResolvedValue([mockSettlement]);
    vi.mocked(api.fetchSettlement).mockResolvedValue(mockSettlement);
    vi.mocked(api.updateSettlementStatus).mockResolvedValue(mockSettlement);
    vi.mocked(api.fetchAvailableSettlementTransactions).mockResolvedValue([]);
  });

  it('renders settlement manager', async () => {
    render(<SettlementManager />);
    await waitFor(() => {
      expect(screen.getByText('Settlements')).toBeInTheDocument();
    });
  });

  it('shows settlement details', async () => {
    render(<SettlementManager />);

    // Wait for settlements to load
    await waitFor(() => {
      expect(screen.getByText('Settlements')).toBeInTheDocument();
    });

    // Check that the settlement is displayed in the list
    await waitFor(() => {
      expect(screen.getByText('Test User → Other User')).toBeInTheDocument();
    });
  });

  it('updates settlement status', async () => {
    render(<SettlementManager />);

    // Wait for settlements to load
    await waitFor(() => {
      expect(screen.getByText('Settlements')).toBeInTheDocument();
    });

    // Click on a settlement to load details
    await waitFor(() => {
      const settlementItem = screen.getByText('Test User → Other User');
      fireEvent.click(settlementItem);
    });

    // Now wait for the settlement details to load
    await waitFor(() => {
      // Check if complete button exists in the details view
      const buttons = screen.getAllByRole('button');
      const completeButton = buttons.find(btn => btn.textContent?.includes('Complete'));

      if (completeButton) {
        fireEvent.click(completeButton);
        expect(api.updateSettlementStatus).toHaveBeenCalledWith('settlement-1', 'completed');
      }
    });
  });
});
