import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import ReportsPage from '../ReportsPage';
import { useAuth } from '../../context/AuthContext';
import { fetchYNABSplits, syncToYNAB } from '../../utils/api';

// Mock the auth hook
vi.mock('../../context/AuthContext', () => ({
  useAuth: vi.fn(),
}));

// Mock the API functions
vi.mock('../../utils/api', () => ({
  fetchYNABSplits: vi.fn(),
  syncToYNAB: vi.fn(),
}));

// Mock localStorage
vi.stubGlobal('localStorage', {
  getItem: vi.fn().mockReturnValue('test-user-id'),
  setItem: vi.fn(),
  removeItem: vi.fn(),
});

describe('ReportsPage', () => {
  const mockUser = {
    uid: 'test-uid',
    email: 'test@example.com',
    emailVerified: true,
  };

  beforeEach(() => {
    vi.clearAllMocks();

    (useAuth as unknown as ReturnType<typeof vi.fn>).mockReturnValue({
      currentUser: mockUser,
    });

    (fetchYNABSplits as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([
      { category: 'Food', total: 100 },
      { category: 'Transport', total: 50 },
    ]);

    (syncToYNAB as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({ success: true });
  });

  it('renders without crashing', () => {
    render(<ReportsPage />);
    expect(screen.getByText(/YNAB Category Splits/)).toBeInTheDocument();
  });

  it('loads and displays YNAB splits', async () => {
    render(<ReportsPage />);
    await waitFor(() => {
      expect(fetchYNABSplits).toHaveBeenCalled();
    });

    const foodCategory = screen.getByText('Food', { selector: 'div[title="Food"]' });
    const transportCategory = screen.getByText('Transport', { selector: 'div[title="Transport"]' });

    expect(foodCategory).toBeInTheDocument();
    expect(transportCategory).toBeInTheDocument();
  });

  it('handles sync to YNAB', async () => {
    render(<ReportsPage />);
    await waitFor(() => {
      expect(fetchYNABSplits).toHaveBeenCalled();
    });

    const syncButton = screen.getByText('Sync This Report to YNAB');
    fireEvent.click(syncButton);

    await waitFor(() => {
      expect(syncToYNAB).toHaveBeenCalled();
    });

    expect(
      screen.getByText(
        /This will create a single transaction in YNAB with split categories based on the report above/i
      )
    ).toBeInTheDocument();
  });

  it('handles filter changes', async () => {
    render(<ReportsPage />);
    const startDateInput = screen.getByLabelText(/start date/i);
    fireEvent.change(startDateInput, { target: { value: '2024-01-01' } });
    expect(startDateInput).toHaveValue('2024-01-01');
  });

  it('displays error message when API call fails', async () => {
    (fetchYNABSplits as unknown as ReturnType<typeof vi.fn>).mockRejectedValue(
      new Error('API Error')
    );
    render(<ReportsPage />);

    await waitFor(() => {
      expect(screen.getByText('Failed to load report data. Please try again.')).toBeInTheDocument();
    });
  });

  it('disables sync button when no splits are available', async () => {
    (fetchYNABSplits as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([]);

    render(<ReportsPage />);

    await waitFor(() => {
      expect(fetchYNABSplits).toHaveBeenCalled();
    });

    expect(screen.queryByText('Sync This Report to YNAB')).not.toBeInTheDocument();
  });

  it('shows loading state while fetching data', async () => {
    render(<ReportsPage />);
    expect(screen.getByTestId('loading-indicator')).toBeInTheDocument();

    await waitFor(() => {
      expect(fetchYNABSplits).toHaveBeenCalled();
    });
  });

  it('validates start date is before end date', async () => {
    (fetchYNABSplits as unknown as ReturnType<typeof vi.fn>).mockImplementation(() => {
      setImmediate(() => {
        // This causes the loadReportData to complete quickly
      });
      return Promise.resolve([]);
    });

    const { container } = render(<ReportsPage />);

    await waitFor(() => {
      expect(screen.queryByTestId('loading-indicator')).not.toBeInTheDocument();
    });

    const startDateInput = screen.getByLabelText(/start date/i);
    const endDateInput = screen.getByLabelText(/end date/i);

    fireEvent.change(startDateInput, { target: { value: '2024-01-02' } });
    fireEvent.change(endDateInput, { target: { value: '2024-01-01' } });

    const submitButton = container.querySelector('button[type="submit"]');
    if (!submitButton) throw new Error('Submit button not found');

    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('Start date must be before end date')).toBeInTheDocument();
    });
  });

  it('handles checkbox filter changes', async () => {
    render(<ReportsPage />);
    const paidCheckbox = screen.getByLabelText(/show only paid transactions/i);
    fireEvent.click(paidCheckbox);
    expect(paidCheckbox).toBeChecked();
  });

  it('displays no data message when no splits are returned', async () => {
    (fetchYNABSplits as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([]);
    render(<ReportsPage />);

    await waitFor(() => {
      expect(fetchYNABSplits).toHaveBeenCalled();
    });

    expect(screen.getByText('No data available for the selected filters.')).toBeInTheDocument();
  });

  it('handles optional transactions checkbox correctly', async () => {
    render(<ReportsPage />);
    const optionalCheckbox = screen.getByLabelText(/exclude optional transactions/i);
    // Check initial state (assuming it's unchecked by default)
    expect(optionalCheckbox).not.toBeChecked();

    // First click should check it
    fireEvent.click(optionalCheckbox);
    expect(optionalCheckbox).toBeChecked();

    // Second click should uncheck it if it's a toggle
    fireEvent.click(optionalCheckbox);
    expect(optionalCheckbox).not.toBeChecked();
  });

  it('renders the correct total amount in the sync button description', async () => {
    render(<ReportsPage />);
    await waitFor(() => {
      expect(fetchYNABSplits).toHaveBeenCalled();
    });

    expect(screen.getByText(/The total amount will be \$150.00./i)).toBeInTheDocument();
  });
});
