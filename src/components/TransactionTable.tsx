import { Transaction } from '../types/transaction';
import { useState } from 'react';
import { formatDate, formatMoney } from '../utils/formatters';
import { fetchTransactionSettlements, removeTransactionFromSettlement } from '../utils/api';
import { useNavigate } from 'react-router-dom';

export interface TransactionTableProps {
  transactions: Transaction[];
  onUpdate: (id: string, updates: Partial<Transaction>) => void;
  onDelete: (id: string) => void;
  onEdit: (id: string) => void;
  onBulkDelete: (ids: string[]) => void;
  onSortChange?: (column: keyof Transaction) => void;
}

function TransactionTable({
  transactions,
  onUpdate,
  onDelete,
  onEdit,
  onBulkDelete,
  onSortChange,
}: TransactionTableProps) {
  const [selectedTransactions, setSelectedTransactions] = useState<string[]>([]);
  const navigate = useNavigate();
  const currentUserId = localStorage.getItem('userId') || '';
  const [applyingTransaction, setApplyingTransaction] = useState<string | null>(null);

  // Calculate totals (treat settled transactions as negative)
  const totals = transactions.reduce(
    (acc, tx) => {
      const amount = tx.inSettlement ? -tx.amount : tx.amount;
      acc.total += amount;
      if (tx.paid) {
        acc.paid += amount;
      } else {
        acc.unpaid += amount;
      }
      if (tx.optional) {
        acc.optional += amount;
      }
      return acc;
    },
    { total: 0, paid: 0, unpaid: 0, optional: 0 }
  );

  const handleSelectAll = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.checked) {
      setSelectedTransactions(transactions.map(tx => tx.id));
    } else {
      setSelectedTransactions([]);
    }
  };

  const handleSelectTransaction = (id: string, checked: boolean) => {
    if (checked) {
      setSelectedTransactions([...selectedTransactions, id]);
    } else {
      setSelectedTransactions(selectedTransactions.filter(txId => txId !== id));
    }
  };

  const handleBulkDelete = () => {
    if (selectedTransactions.length === 0) return;

    if (
      window.confirm(`Are you sure you want to delete ${selectedTransactions.length} transactions?`)
    ) {
      onBulkDelete(selectedTransactions);
      setSelectedTransactions([]);
    }
  };

  const handleMarkPaid = (id: string, isPaid: boolean) => {
    onUpdate(id, {
      paid: isPaid,
      paidDate: isPaid ? new Date().toISOString() : '',
    });
  };

  const handleApplyToDebt = async (transaction: Transaction) => {
    try {
      setApplyingTransaction(transaction.id);

      // Navigate to settlements page - let user create a settlement for any transaction
      navigate('/settlements');
    } catch (error) {
      console.error('Error navigating to settlements:', error);
      alert('Failed to navigate to settlements page');
    } finally {
      setApplyingTransaction(null);
    }
  };

  // Handle unsettling a transaction (remove from settlement)
  const handleUnsettleTransaction = async (transactionId: string) => {
    try {
      // First, get the settlements this transaction belongs to
      const settlements = await fetchTransactionSettlements(transactionId);

      if (settlements.length === 0) {
        alert('No settlements found for this transaction');
        return;
      }

      // Remove from the first active settlement (there should typically be only one)
      const activeSettlement = settlements.find(s => s.status === 'active');
      if (!activeSettlement) {
        alert('No active settlement found for this transaction');
        return;
      }

      // Remove the transaction from the settlement
      await removeTransactionFromSettlement(activeSettlement.id, transactionId);

      // Refresh the transactions by calling onUpdate to trigger a reload
      onUpdate(transactionId, { inSettlement: false, paid: false });

      alert('Transaction removed from settlement successfully');
    } catch (error) {
      console.error('Error unsettling transaction:', error);
      alert('Failed to unsettle transaction');
    }
  };

  // Column header click handler for sorting
  const handleSortClick = (column: keyof Transaction) => {
    if (onSortChange) {
      onSortChange(column);
    }
  };

  // Function to display category information from transaction
  const renderCategoryInfo = (tx: Transaction) => {
    if (tx.categories && tx.categories.length > 0) {
      return (
        <div>
          {tx.categories.map((cat, index) => (
            <div key={cat.id || index} className="text-sm text-gray-900">
              {cat.name}
            </div>
          ))}
        </div>
      );
    }

    return <div className="text-sm text-gray-900">{tx.category}</div>;
  };

  return (
    <div className="mt-6">
      {/* Totals Summary */}
      <div className="mb-4 grid grid-cols-4 gap-4">
        <div className="bg-white p-4 rounded-lg shadow">
          <div className="text-sm font-medium text-gray-500">Total</div>
          <div className="text-2xl font-bold text-gray-900">{formatMoney(totals.total)}</div>
        </div>
        <div className="bg-green-50 p-4 rounded-lg shadow">
          <div className="text-sm font-medium text-green-700">Paid</div>
          <div className="text-2xl font-bold text-green-900">{formatMoney(totals.paid)}</div>
        </div>
        <div className="bg-yellow-50 p-4 rounded-lg shadow">
          <div className="text-sm font-medium text-yellow-700">Unpaid</div>
          <div className="text-2xl font-bold text-yellow-900">{formatMoney(totals.unpaid)}</div>
        </div>
        <div className="bg-purple-50 p-4 rounded-lg shadow">
          <div className="text-sm font-medium text-purple-700">Optional</div>
          <div className="text-2xl font-bold text-purple-900">{formatMoney(totals.optional)}</div>
        </div>
      </div>

      <div className="bg-white overflow-hidden shadow rounded-lg">
        {selectedTransactions.length > 0 && (
          <div className="bg-blue-50 p-4 flex justify-between items-center">
            <span className="text-blue-700">
              {selectedTransactions.length} transaction(s) selected
            </span>
            <button
              onClick={handleBulkDelete}
              className="bg-red-600 hover:bg-red-700 text-white px-4 py-2 rounded text-sm"
            >
              Delete Selected
            </button>
          </div>
        )}

        <div className="overflow-x-auto">
          <div className="inline-block min-w-full align-middle">
            <div className="overflow-hidden shadow-sm ring-1 ring-black ring-opacity-5">
              <table className="min-w-full divide-y divide-gray-300">
                <thead className="bg-gray-50">
                  <tr>
                    <th scope="col" className="relative w-8 px-1 py-3">
                      <input
                        type="checkbox"
                        checked={selectedTransactions.length === transactions.length}
                        onChange={handleSelectAll}
                        className="h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"
                      />
                    </th>
                    <th
                      scope="col"
                      className="hidden sm:table-cell px-1 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer"
                      onClick={() => handleSortClick('entered')}
                    >
                      Entered
                    </th>
                    <th
                      scope="col"
                      className="px-1 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer"
                      onClick={() => handleSortClick('transactionDate')}
                    >
                      Date
                    </th>
                    <th
                      scope="col"
                      className="px-1 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer"
                      onClick={() => handleSortClick('paidBy')}
                    >
                      Owed To
                    </th>
                    <th
                      scope="col"
                      className="hidden md:table-cell px-1 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer"
                      onClick={() => handleSortClick('owedBy')}
                    >
                      Owed By
                    </th>
                    <th
                      scope="col"
                      className="px-1 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer"
                      onClick={() => handleSortClick('amount')}
                    >
                      Amount
                    </th>
                    <th
                      scope="col"
                      className="hidden lg:table-cell px-1 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer"
                      onClick={() => handleSortClick('category')}
                    >
                      Category
                    </th>
                    <th
                      scope="col"
                      className="hidden xl:table-cell px-1 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer"
                      onClick={() => handleSortClick('note')}
                    >
                      Note
                    </th>
                    <th
                      scope="col"
                      className="px-1 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer"
                      onClick={() => handleSortClick('paid')}
                    >
                      Status
                    </th>
                    <th
                      scope="col"
                      className="hidden sm:table-cell px-1 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer"
                      onClick={() => handleSortClick('enteredBy')}
                    >
                      By
                    </th>
                    <th scope="col" className="relative px-1 py-3 w-20">
                      <span className="sr-only">Actions</span>
                    </th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-200">
                  {transactions.length === 0 ? (
                    <tr>
                      <td colSpan={11} className="px-2 py-8 text-center text-sm text-gray-500">
                        No transactions found.
                      </td>
                    </tr>
                  ) : (
                    transactions.map(tx => {
                      let rowClassName = '';
                      if (tx.paid) {
                        rowClassName = 'bg-green-50';
                      } else if (tx.inSettlement) {
                        rowClassName = 'bg-blue-50';
                      }

                      return (
                        <tr key={tx.id} className={rowClassName}>
                          <td className="px-1 py-2">
                            <input
                              type="checkbox"
                              checked={selectedTransactions.includes(tx.id)}
                              onChange={e => handleSelectTransaction(tx.id, e.target.checked)}
                              className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
                            />
                          </td>
                          <td className="hidden sm:table-cell px-1 py-2">
                            <div className="text-xs text-gray-900">
                              {tx.entered ? formatDate(tx.entered) : 'No date'}
                            </div>
                          </td>
                          <td className="px-1 py-2">
                            <div className="text-xs text-gray-900">
                              {formatDate(tx.transactionDate)}
                            </div>
                          </td>
                          <td className="px-1 py-2">
                            <div className="text-xs text-gray-900 truncate max-w-20">
                              {tx.paidBy || tx.payTo || tx.enteredBy}
                            </div>
                          </td>
                          <td className="hidden md:table-cell px-1 py-2">
                            <div className="text-xs text-gray-900 truncate max-w-20">
                              {tx.owedBy || '-'}
                            </div>
                          </td>
                          <td className="px-1 py-2 text-xs">
                            <span
                              className={`font-semibold ${
                                tx.optional
                                  ? 'text-gray-500'
                                  : tx.inSettlement
                                    ? 'text-red-600'
                                    : 'text-gray-900'
                              }`}
                            >
                              {tx.inSettlement
                                ? `-${formatMoney(tx.amount)}`
                                : formatMoney(tx.amount)}
                            </span>
                            {tx.optional && (
                              <span className="ml-1 px-1 inline-flex text-xs leading-4 font-semibold rounded-full bg-purple-100 text-purple-800">
                                Opt
                              </span>
                            )}
                            {tx.inSettlement && (
                              <span className="ml-1 px-1 inline-flex text-xs leading-4 font-semibold rounded-full bg-blue-100 text-blue-800">
                                ⚖️ Settlement
                              </span>
                            )}
                          </td>
                          <td className="hidden lg:table-cell px-1 py-2">
                            {renderCategoryInfo(tx)}
                          </td>
                          <td className="hidden xl:table-cell px-1 py-2">
                            <div className="text-xs text-gray-900 truncate max-w-32">{tx.note}</div>
                          </td>
                          <td className="px-1 py-2">
                            <div className="flex flex-col gap-1">
                              {tx.category === 'settlement' ? (
                                <span className="px-2 inline-flex text-xs leading-4 font-semibold rounded-full bg-purple-100 text-purple-800">
                                  Settlement
                                </span>
                              ) : tx.paid ? (
                                <button
                                  onClick={() => handleMarkPaid(tx.id, false)}
                                  className="px-2 inline-flex text-xs leading-4 font-semibold rounded-full bg-green-500 text-white hover:bg-green-600 shadow-sm"
                                >
                                  Paid ✓
                                </button>
                              ) : (
                                <button
                                  onClick={() => handleMarkPaid(tx.id, true)}
                                  className="px-2 inline-flex text-xs leading-4 font-semibold rounded-full bg-yellow-100 text-yellow-800 hover:bg-yellow-200"
                                >
                                  Unpaid
                                </button>
                              )}
                            </div>
                          </td>
                          <td className="hidden sm:table-cell px-1 py-2">
                            <div className="text-xs text-gray-900 truncate max-w-16">
                              {tx.enteredBy}
                            </div>
                          </td>
                          <td className="px-1 py-2 text-right">
                            <div className="flex flex-col gap-1">
                              {tx.inSettlement && tx.category !== 'settlement' ? (
                                <button
                                  onClick={() => handleUnsettleTransaction(tx.id)}
                                  className="text-xs px-1 py-1 bg-blue-100 text-blue-700 hover:bg-blue-200 rounded"
                                  title="Remove from settlement"
                                >
                                  ⚖️ Unsettle
                                </button>
                              ) : (
                                !tx.paid &&
                                tx.category !== 'settlement' && (
                                  <button
                                    onClick={() => handleApplyToDebt(tx)}
                                    disabled={applyingTransaction === tx.id}
                                    className="text-xs px-1 py-1 bg-green-100 text-green-700 hover:bg-green-200 rounded disabled:opacity-50"
                                    title={
                                      tx.enteredBy === currentUserId
                                        ? 'Create settlement for others to apply'
                                        : 'Apply this to your debt'
                                    }
                                  >
                                    {applyingTransaction === tx.id ? '...' : '⚖️ Settle'}
                                  </button>
                                )
                              )}
                              <div className="flex gap-1">
                                <button
                                  onClick={() => onEdit(tx.id)}
                                  className="text-xs px-1 py-1 bg-blue-100 text-blue-700 hover:bg-blue-200 rounded"
                                >
                                  Edit
                                </button>
                                <button
                                  onClick={() => onDelete(tx.id)}
                                  className="text-xs px-1 py-1 bg-red-100 text-red-700 hover:bg-red-200 rounded"
                                >
                                  Del
                                </button>
                              </div>
                            </div>
                          </td>
                        </tr>
                      );
                    })
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

export default TransactionTable;
