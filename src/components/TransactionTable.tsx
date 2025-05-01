import { Transaction } from '../types/transaction';
import { useState } from 'react';

interface TransactionTableProps {
  transactions: Transaction[];
  onUpdate: (id: string, updates: Partial<Transaction>) => void;
  onDelete: (id: string) => void;
  onEdit: (id: string) => void;
  onBulkDelete?: (ids: string[]) => void;
}

type SortField = 'entered' | 'transactionDate' | 'category' | '';
type SortDirection = 'asc' | 'desc';

function TransactionTable({ transactions, onUpdate, onDelete, onEdit, onBulkDelete }: TransactionTableProps) {
  const [selectedTransactions, setSelectedTransactions] = useState<string[]>([]);
  const [sortField, setSortField] = useState<SortField>('');
  const [sortDirection, setSortDirection] = useState<SortDirection>('asc');

  const handlePaidToggle = (tx: Transaction) => {
    onUpdate(tx.id, {
      paid: !tx.paid,
      paidDate: !tx.paid ? new Date().toISOString() : undefined,
    });
  };

  const handleOptionalToggle = (tx: Transaction) => {
    onUpdate(tx.id, {
      optional: !tx.optional,
    });
  };

  const handleSelectAll = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.checked) {
      setSelectedTransactions(transactions.map(tx => tx.id));
    } else {
      setSelectedTransactions([]);
    }
  };

  const handleSelectTransaction = (e: React.ChangeEvent<HTMLInputElement>, id: string) => {
    if (e.target.checked) {
      setSelectedTransactions([...selectedTransactions, id]);
    } else {
      setSelectedTransactions(selectedTransactions.filter(txId => txId !== id));
    }
  };

  const handleBulkDelete = () => {
    if (selectedTransactions.length === 0) return;
    
    if (window.confirm(`Are you sure you want to delete ${selectedTransactions.length} transactions?`)) {
      onBulkDelete?.(selectedTransactions);
      setSelectedTransactions([]);
    }
  };

  const handleSort = (field: SortField) => {
    if (sortField === field) {
      // If already sorting by this field, toggle direction
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc');
    } else {
      // Otherwise, set new sort field and default to ascending
      setSortField(field);
      setSortDirection('asc');
    }
  };

  // Apply sorting to transactions
  const sortedTransactions = [...transactions].sort((a, b) => {
    if (sortField === 'entered') {
      const dateA = new Date(a.entered).getTime();
      const dateB = new Date(b.entered).getTime();
      return sortDirection === 'asc' ? dateA - dateB : dateB - dateA;
    } else if (sortField === 'transactionDate') {
      const dateA = new Date(a.transactionDate).getTime();
      const dateB = new Date(b.transactionDate).getTime();
      return sortDirection === 'asc' ? dateA - dateB : dateB - dateA;
    } else if (sortField === 'category') {
      const categoryA = a.category.toLowerCase();
      const categoryB = b.category.toLowerCase();
      return sortDirection === 'asc' 
        ? categoryA.localeCompare(categoryB)
        : categoryB.localeCompare(categoryA);
    }
    return 0;
  });

  return (
    <div className="bg-white p-4 rounded shadow overflow-x-auto">
      {onBulkDelete && selectedTransactions.length > 0 && (
        <div className="mb-4 flex justify-between items-center">
          <span>{selectedTransactions.length} transactions selected</span>
          <button
            onClick={handleBulkDelete}
            className="bg-red-500 hover:bg-red-600 text-white py-1 px-3 rounded"
          >
            Delete Selected
          </button>
        </div>
      )}
      <table className="min-w-full table-auto">
        <thead>
          <tr className="bg-gray-200">
            {onBulkDelete && (
              <th className="p-2 w-10">
                <input
                  type="checkbox"
                  onChange={handleSelectAll}
                  checked={selectedTransactions.length === transactions.length && transactions.length > 0}
                />
              </th>
            )}
            <th className="p-2 cursor-pointer" onClick={() => handleSort('entered')}>
              Entered Date {sortField === 'entered' && (sortDirection === 'asc' ? '↑' : '↓')}
            </th>
            <th className="p-2 cursor-pointer" onClick={() => handleSort('transactionDate')}>
              Tx Date {sortField === 'transactionDate' && (sortDirection === 'asc' ? '↑' : '↓')}
            </th>
            <th className="p-2">Pay To</th>
            <th className="p-2">Amount</th>
            <th className="p-2 cursor-pointer" onClick={() => handleSort('category')}>
              Category {sortField === 'category' && (sortDirection === 'asc' ? '↑' : '↓')}
            </th>
            <th className="p-2">Note</th>
            <th className="p-2">Paid</th>
            <th className="p-2">Paid Date</th>
            <th className="p-2">Optional</th>
            <th className="p-2">Actions</th>
          </tr>
        </thead>
        <tbody>
          {sortedTransactions.map((tx) => (
            <tr 
              key={tx.id} 
              className={`border-t ${tx.optional ? 'italic text-gray-600 bg-gray-50' : ''}`}
            >
              {onBulkDelete && (
                <td className="p-2">
                  <input
                    type="checkbox"
                    checked={selectedTransactions.includes(tx.id)}
                    onChange={(e) => handleSelectTransaction(e, tx.id)}
                  />
                </td>
              )}
              <td className="p-2">{new Date(tx.entered).toLocaleDateString()}</td>
              <td className="p-2">{new Date(tx.transactionDate).toLocaleDateString()}</td>
              <td className="p-2">{tx.payTo}</td>
              <td className="p-2">${tx.amount.toFixed(2)}</td>
              <td className="p-2">{tx.category}</td>
              <td className="p-2">{tx.note}</td>
              <td className="p-2 text-center">
                <input
                  type="checkbox"
                  checked={tx.paid}
                  onChange={() => handlePaidToggle(tx)}
                />
              </td>
              <td className="p-2">
                {tx.paidDate ? new Date(tx.paidDate).toLocaleDateString() : ''}
              </td>
              <td className="p-2 text-center">
                <input
                  type="checkbox"
                  checked={tx.optional || false}
                  onChange={() => handleOptionalToggle(tx)}
                />
              </td>
              <td className="p-2 flex gap-2 justify-center">
                <button
                  onClick={() => onEdit(tx.id)}
                  className="text-blue-500 hover:underline"
                >
                  Edit
                </button>
                <button
                  onClick={() => onDelete(tx.id)}
                  className="text-red-500 hover:underline"
                >
                  Delete
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export default TransactionTable;
