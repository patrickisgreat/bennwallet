import { Transaction } from '../types/transaction';
import { useState } from 'react';
import { formatDate, formatMoney } from '../utils/formatters';

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

  const handleSelectAll = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.checked) {
      setSelectedTransactions(transactions.map((tx) => tx.id));
    } else {
      setSelectedTransactions([]);
    }
  };

  const handleSelectTransaction = (id: string, checked: boolean) => {
    if (checked) {
      setSelectedTransactions([...selectedTransactions, id]);
    } else {
      setSelectedTransactions(selectedTransactions.filter((txId) => txId !== id));
    }
  };

  const handleBulkDelete = () => {
    if (selectedTransactions.length === 0) return;

    if (window.confirm(`Are you sure you want to delete ${selectedTransactions.length} transactions?`)) {
      onBulkDelete(selectedTransactions);
      setSelectedTransactions([]);
    }
  };

  const handleMarkPaid = (id: string, isPaid: boolean) => {
    onUpdate(id, { 
      paid: isPaid, 
      paidDate: isPaid ? new Date().toISOString() : ''
    });
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
    <div className="mt-6 bg-white overflow-hidden shadow rounded-lg">
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
                  <th scope="col" className="relative w-12 px-1 py-3">
                    <input
                      type="checkbox"
                      checked={selectedTransactions.length === transactions.length}
                      onChange={handleSelectAll}
                      className="absolute left-4 top-1/2 -mt-2 h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"
                    />
                  </th>
                  <th
                    scope="col"
                    className="px-1 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer"
                    onClick={() => handleSortClick('entered')}
                  >
                    Entered Date
                  </th>
                  <th
                    scope="col"
                    className="px-1 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer"
                    onClick={() => handleSortClick('transactionDate')}
                  >
                    Transaction Date
                  </th>
                  <th
                    scope="col"
                    className="px-1 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer"
                    onClick={() => handleSortClick('payTo')}
                  >
                    Pay To
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
                    className="px-1 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer"
                    onClick={() => handleSortClick('category')}
                  >
                    Category
                  </th>
                  <th
                    scope="col"
                    className="px-1 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer"
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
                    className="px-1 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer"
                    onClick={() => handleSortClick('enteredBy')}
                  >
                    Entered By
                  </th>
                  <th scope="col" className="relative px-1 py-3">
                    <span className="sr-only">Actions</span>
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-gray-200">
                {transactions.length === 0 ? (
                  <tr>
                    <td colSpan={9} className="px-2 py-8 text-center text-sm text-gray-500">
                      No transactions found.
                    </td>
                  </tr>
                ) : (
                  transactions.map((tx) => (
                    <tr key={tx.id}>
                      <td className="px-1 py-2">
                        <input
                          type="checkbox"
                          checked={selectedTransactions.includes(tx.id)}
                          onChange={(e) => handleSelectTransaction(tx.id, e.target.checked)}
                          className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-gray-300 rounded"
                        />
                      </td>
                      <td className="px-1 py-2">
                        <div className="text-sm text-gray-900">
                          {tx.entered ? formatDate(tx.entered) : 'No date'}
                        </div>
                      </td>
                      <td className="px-1 py-2">
                        <div className="text-sm text-gray-900">
                          {formatDate(tx.transactionDate)}
                        </div>
                      </td>
                      <td className="px-1 py-2">
                        <div className="text-sm text-gray-900 truncate">{tx.payTo}</div>
                      </td>
                      <td className="px-1 py-2 text-sm">
                        <span className={`font-semibold ${tx.optional ? 'text-gray-500' : 'text-gray-900'}`}>
                          {formatMoney(tx.amount)}
                        </span>
                        {tx.optional && (
                          <span className="ml-1 px-1 inline-flex text-xs leading-5 font-semibold rounded-full bg-purple-100 text-purple-800">
                            Opt
                          </span>
                        )}
                      </td>
                      <td className="px-1 py-2">
                        {renderCategoryInfo(tx)}
                      </td>
                      <td className="px-1 py-2">
                        <div className="text-sm text-gray-900 truncate max-w-xs">{tx.note}</div>
                      </td>
                      <td className="px-1 py-2">
                        {tx.paid ? (
                          <button
                            onClick={() => handleMarkPaid(tx.id, false)}
                            className="px-2 inline-flex text-xs leading-5 font-semibold rounded-full bg-green-100 text-green-800 hover:bg-green-200"
                          >
                            Paid
                          </button>
                        ) : (
                          <button
                            onClick={() => handleMarkPaid(tx.id, true)}
                            className="px-2 inline-flex text-xs leading-5 font-semibold rounded-full bg-yellow-100 text-yellow-800 hover:bg-yellow-200"
                          >
                            Unpaid
                          </button>
                        )}
                      </td>
                      <td className="px-1 py-2">
                        <div className="text-sm text-gray-900 truncate">{tx.enteredBy}</div>
                      </td>
                      <td className="px-1 py-2 text-right text-sm font-medium">
                        <button
                          onClick={() => onEdit(tx.id)}
                          className="text-indigo-600 hover:text-indigo-900 mr-1"
                        >
                          Edit
                        </button>
                        <button
                          onClick={() => onDelete(tx.id)}
                          className="text-red-600 hover:text-red-900"
                        >
                          Delete
                        </button>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  );
}

export default TransactionTable;
