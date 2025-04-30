import { Transaction } from '../types/transaction';

interface TransactionTableProps {
  transactions: Transaction[];
  onUpdate: (id: string, updates: Partial<Transaction>) => void;
  onDelete: (id: string) => void;
  onEdit: (id: string) => void;
}

function TransactionTable({ transactions, onUpdate, onDelete, onEdit }: TransactionTableProps) {
  const handlePaidToggle = (tx: Transaction) => {
    onUpdate(tx.id, {
      paid: !tx.paid,
      paidDate: !tx.paid ? new Date().toISOString() : undefined,
    });
  };

  return (
    <div className="bg-white p-4 rounded shadow overflow-x-auto">
      <table className="min-w-full table-auto">
        <thead>
          <tr className="bg-gray-200">
            <th className="p-2">Date</th>
            <th className="p-2">Pay To</th>
            <th className="p-2">Amount</th>
            <th className="p-2">Category</th>
            <th className="p-2">Note</th>
            <th className="p-2">Paid</th>
            <th className="p-2">Paid Date</th>
            <th className="p-2">Actions</th>
          </tr>
        </thead>
        <tbody>
          {transactions.map((tx) => (
            <tr key={tx.id} className="border-t">
              <td className="p-2">{new Date(tx.entered).toLocaleDateString()}</td>
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
