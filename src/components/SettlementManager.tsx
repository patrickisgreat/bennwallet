import { useState, useEffect, useCallback } from 'react';
import {
  fetchUserSettlements,
  fetchSettlement,
  applyTransactionToSettlement,
  removeTransactionFromSettlement,
  fetchAvailableSettlementTransactions,
  updateSettlementStatus,
} from '../utils/api';
import { Settlement, SettlementSummary, SettlementItem } from '../types/settlement';
import { Transaction } from '../types/transaction';
import { formatDate, formatMoney } from '../utils/formatters';

export default function SettlementManager() {
  const [settlements, setSettlements] = useState<SettlementSummary[]>([]);
  const [selectedSettlement, setSelectedSettlement] = useState<Settlement | null>(null);
  const [availableTransactions, setAvailableTransactions] = useState<Transaction[]>([]);
  const [statusFilter, setStatusFilter] = useState<string>('');
  const [loading, setLoading] = useState(false);
  const [applyAmount, setApplyAmount] = useState<string>('');
  const [selectedTransaction, setSelectedTransaction] = useState<string>('');

  const loadSettlements = useCallback(async () => {
    try {
      setLoading(true);
      const data = await fetchUserSettlements(statusFilter);
      setSettlements(data);
    } catch (error) {
      console.error('Error loading settlements:', error);
    } finally {
      setLoading(false);
    }
  }, [statusFilter]);

  // Load settlements
  useEffect(() => {
    loadSettlements();
  }, [loadSettlements]);

  const loadSettlementDetails = async (id: string) => {
    try {
      setLoading(true);
      const settlement = await fetchSettlement(id);
      setSelectedSettlement(settlement);

      // Load available transactions for applying
      if (settlement) {
        const transactions = await fetchAvailableSettlementTransactions(id);
        setAvailableTransactions(transactions);
      }
    } catch (error) {
      console.error('Error loading settlement details:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleApplyTransaction = async () => {
    if (!selectedSettlement || !selectedTransaction || !applyAmount) return;

    try {
      setLoading(true);
      const updated = await applyTransactionToSettlement(selectedSettlement.id, {
        transactionId: selectedTransaction,
        amount: parseFloat(applyAmount),
      });
      setSelectedSettlement(updated);
      setSelectedTransaction('');
      setApplyAmount('');
      // Reload settlements list
      loadSettlements();
    } catch (error) {
      console.error('Error applying transaction:', error);
      alert('Failed to apply transaction');
    } finally {
      setLoading(false);
    }
  };

  const handleRemoveTransaction = async (item: SettlementItem) => {
    if (!selectedSettlement) return;

    if (!window.confirm('Are you sure you want to remove this transaction from the settlement?')) {
      return;
    }

    try {
      setLoading(true);
      const updated = await removeTransactionFromSettlement(
        selectedSettlement.id,
        item.transactionId
      );
      setSelectedSettlement(updated);
      // Reload settlements list
      loadSettlements();
    } catch (error) {
      console.error('Error removing transaction:', error);
      alert('Failed to remove transaction');
    } finally {
      setLoading(false);
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'active':
        return 'bg-yellow-100 text-yellow-800';
      case 'completed':
        return 'bg-green-100 text-green-800';
      case 'cancelled':
        return 'bg-red-100 text-red-800';
      default:
        return 'bg-gray-100 text-gray-800';
    }
  };

  const handleCancelSettlement = async () => {
    if (!selectedSettlement || selectedSettlement.status !== 'active') return;

    const reason = window.prompt('Please provide a reason for cancelling this settlement:');
    if (reason === null) return; // User cancelled

    try {
      setLoading(true);
      const updated = await updateSettlementStatus(selectedSettlement.id, 'cancelled', reason);
      setSelectedSettlement(updated);
      // Reload settlements list
      loadSettlements();
      alert('Settlement cancelled successfully');
    } catch (error) {
      console.error('Error cancelling settlement:', error);
      alert('Failed to cancel settlement');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="container mx-auto px-4 py-8">
      <h1 className="text-3xl font-bold mb-8">Settlement Manager</h1>

      {/* Filter Controls */}
      <div className="mb-6 flex items-center space-x-4">
        <label className="flex items-center">
          <span className="mr-2 text-sm font-medium">Status:</span>
          <select
            value={statusFilter}
            onChange={e => setStatusFilter(e.target.value)}
            className="border rounded px-3 py-1"
          >
            <option value="">All</option>
            <option value="active">Active</option>
            <option value="completed">Completed</option>
            <option value="cancelled">Cancelled</option>
          </select>
        </label>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Settlements List */}
        <div className="bg-white rounded-lg shadow p-6">
          <h2 className="text-xl font-semibold mb-4">Settlements</h2>

          {loading && !selectedSettlement ? (
            <p className="text-gray-500">Loading...</p>
          ) : settlements.length === 0 ? (
            <p className="text-gray-500">No settlements found</p>
          ) : (
            <div className="space-y-4">
              {settlements.map(settlement => (
                <div
                  key={settlement.id}
                  onClick={() => loadSettlementDetails(settlement.id)}
                  className="border rounded-lg p-4 cursor-pointer hover:bg-gray-50 transition"
                >
                  <div className="flex justify-between items-start mb-2">
                    <div>
                      <p className="font-medium">
                        {settlement.createdByName} → {settlement.createdForName}
                      </p>
                      <p className="text-sm text-gray-500">
                        Created {formatDate(settlement.createdAt)}
                      </p>
                    </div>
                    <span
                      className={`px-2 py-1 text-xs rounded-full ${getStatusColor(settlement.status)}`}
                    >
                      {settlement.status}
                    </span>
                  </div>
                  <div className="flex justify-between items-end">
                    <div>
                      <p className="text-sm text-gray-600">{settlement.itemCount} transaction(s)</p>
                    </div>
                    <div className="text-right">
                      <p className="text-sm text-gray-600">
                        {formatMoney(settlement.remainingAmount)} /{' '}
                        {formatMoney(settlement.totalAmount)}
                      </p>
                      <div className="w-32 bg-gray-200 rounded-full h-2 mt-1">
                        <div
                          className="bg-blue-600 h-2 rounded-full"
                          style={{
                            width: `${((settlement.totalAmount - settlement.remainingAmount) / settlement.totalAmount) * 100}%`,
                          }}
                        />
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Settlement Details */}
        <div className="bg-white rounded-lg shadow p-6">
          <h2 className="text-xl font-semibold mb-4">Settlement Details</h2>

          {!selectedSettlement ? (
            <p className="text-gray-500">Select a settlement to view details</p>
          ) : (
            <div>
              <div className="mb-6">
                <div className="flex justify-between items-center mb-2">
                  <h3 className="font-medium">Overview</h3>
                  <div className="flex items-center space-x-2">
                    <span
                      className={`px-2 py-1 text-xs rounded-full ${getStatusColor(selectedSettlement.status)}`}
                    >
                      {selectedSettlement.status}
                    </span>
                    {selectedSettlement.status === 'active' && (
                      <button
                        onClick={handleCancelSettlement}
                        className="text-red-600 hover:text-red-800 text-sm font-medium"
                        disabled={loading}
                      >
                        Cancel
                      </button>
                    )}
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-4 text-sm">
                  <div>
                    <p className="text-gray-600">Total Amount</p>
                    <p className="font-medium">{formatMoney(selectedSettlement.totalAmount)}</p>
                  </div>
                  <div>
                    <p className="text-gray-600">Remaining</p>
                    <p className="font-medium">{formatMoney(selectedSettlement.remainingAmount)}</p>
                  </div>
                  <div>
                    <p className="text-gray-600">Created</p>
                    <p className="font-medium">{formatDate(selectedSettlement.createdAt)}</p>
                  </div>
                  {selectedSettlement.completedAt && (
                    <div>
                      <p className="text-gray-600">Completed</p>
                      <p className="font-medium">{formatDate(selectedSettlement.completedAt)}</p>
                    </div>
                  )}
                </div>
                {selectedSettlement.notes && (
                  <div className="mt-4">
                    <p className="text-gray-600 text-sm">Notes</p>
                    <p className="text-sm">{selectedSettlement.notes}</p>
                  </div>
                )}
              </div>

              {/* Settlement Items */}
              <div className="mb-6">
                <h3 className="font-medium mb-2">Applied Transactions</h3>
                {selectedSettlement.items && selectedSettlement.items.length > 0 ? (
                  <div className="space-y-2">
                    {selectedSettlement.items.map(item => (
                      <div
                        key={item.id}
                        className="border rounded p-3 flex justify-between items-center"
                      >
                        <div className="flex-1">
                          <p className="text-sm font-medium">
                            {item.transaction?.description || 'Transaction'}
                          </p>
                          <p className="text-xs text-gray-500">
                            Applied {formatDate(item.createdAt)}
                          </p>
                        </div>
                        <div className="flex items-center space-x-2">
                          <span className="font-medium">{formatMoney(item.appliedAmount)}</span>
                          {selectedSettlement.status === 'active' && (
                            <button
                              onClick={() => handleRemoveTransaction(item)}
                              className="text-red-600 hover:text-red-800 text-sm"
                            >
                              Remove
                            </button>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="text-sm text-gray-500">No transactions applied yet</p>
                )}
              </div>

              {/* Apply Transaction Form */}
              {selectedSettlement.status === 'active' && selectedSettlement.remainingAmount > 0 && (
                <div className="border-t pt-4">
                  <h3 className="font-medium mb-2">Apply Transaction</h3>
                  <div className="space-y-3">
                    <select
                      value={selectedTransaction}
                      onChange={e => setSelectedTransaction(e.target.value)}
                      className="w-full border rounded px-3 py-2"
                    >
                      <option value="">Select a transaction</option>
                      {availableTransactions.map(tx => (
                        <option key={tx.id} value={tx.id}>
                          {tx.payTo} - {formatMoney(tx.amount)} - {formatDate(tx.transactionDate)}
                        </option>
                      ))}
                    </select>
                    <input
                      type="number"
                      step="0.01"
                      placeholder="Amount to apply"
                      value={applyAmount}
                      onChange={e => setApplyAmount(e.target.value)}
                      max={selectedSettlement.remainingAmount}
                      className="w-full border rounded px-3 py-2"
                    />
                    <button
                      onClick={handleApplyTransaction}
                      disabled={!selectedTransaction || !applyAmount || loading}
                      className="w-full bg-blue-600 text-white py-2 rounded hover:bg-blue-700 disabled:opacity-50"
                    >
                      Apply Transaction
                    </button>
                  </div>
                </div>
              )}

              {/* History */}
              {selectedSettlement.history && selectedSettlement.history.length > 0 && (
                <div className="mt-6 border-t pt-4">
                  <h3 className="font-medium mb-2">History</h3>
                  <div className="space-y-2">
                    {selectedSettlement.history.map(entry => (
                      <div key={entry.id} className="text-sm">
                        <p className="text-gray-600">
                          {formatDate(entry.createdAt)} - {entry.action.replace('_', ' ')}
                          {entry.amount && ` - ${formatMoney(entry.amount)}`}
                        </p>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
