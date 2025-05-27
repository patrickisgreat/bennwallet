import { useState, useEffect } from 'react';
import { fetchTransactions, createSettlement, fetchUsers } from '../utils/api';
import { Transaction } from '../types/transaction';
import { User } from '../utils/api';
import { formatMoney } from '../utils/formatters';

interface DebtSummary {
  userId: string;
  userName: string;
  youOwe: number;
  theyOwe: number;
  net: number;
  transactionsYouOwe: Transaction[]; // Transactions where you owe them
  transactionsTheyOwe: Transaction[]; // Transactions where they owe you
  yourTransactionsToApply: Transaction[]; // Your transactions that others can apply
}

export default function DebtSummary() {
  const [debts, setDebts] = useState<DebtSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [users, setUsers] = useState<User[]>([]);
  const [selectedDebtor, setSelectedDebtor] = useState<string | null>(null);
  const [selectedTransactions, setSelectedTransactions] = useState<string[]>([]);
  const [settlementNotes, setSettlementNotes] = useState('');
  const currentUserId = localStorage.getItem('userId') || '';

  useEffect(() => {
    loadDebtSummary();
  }, []);

  const loadDebtSummary = async () => {
    try {
      setLoading(true);
      const [transactions, allUsers] = await Promise.all([
        fetchTransactions(),
        fetchUsers()
      ]);
      setUsers(allUsers);

      // Calculate debts by user
      const debtMap = new Map<string, DebtSummary>();

      transactions.forEach(tx => {
        // Skip paid transactions
        if (tx.paid) return;

        if (tx.enteredBy === currentUserId && tx.payTo) {
          // Current user entered this, so someone owes them
          // Find the user ID for payTo
          const payToUser = allUsers.find(u => 
            u.name === tx.payTo || u.username === tx.payTo
          );
          
          if (payToUser) {
            if (!debtMap.has(payToUser.id)) {
              debtMap.set(payToUser.id, {
                userId: payToUser.id,
                userName: payToUser.name,
                youOwe: 0,
                theyOwe: 0,
                net: 0,
                transactionsYouOwe: [],
                transactionsTheyOwe: [],
                yourTransactionsToApply: []
              });
            }
            const debt = debtMap.get(payToUser.id)!;
            debt.theyOwe += tx.amount;
            debt.transactionsTheyOwe.push(tx);
          }
        } else if (tx.enteredBy !== currentUserId) {
          // Someone else entered this, so current user owes them
          if (!debtMap.has(tx.enteredBy)) {
            const enteredByUser = allUsers.find(u => u.id === tx.enteredBy);
            if (enteredByUser) {
              debtMap.set(tx.enteredBy, {
                userId: tx.enteredBy,
                userName: enteredByUser.name,
                youOwe: 0,
                theyOwe: 0,
                net: 0,
                transactionsYouOwe: [],
                transactionsTheyOwe: [],
                yourTransactionsToApply: []
              });
            }
          }
          const debt = debtMap.get(tx.enteredBy);
          if (debt) {
            debt.youOwe += tx.amount;
            debt.transactionsYouOwe.push(tx);
          }
        }
      });

      // Now find transactions where current user entered and others can apply
      transactions.forEach(tx => {
        if (tx.paid) return;
        
        if (tx.enteredBy === currentUserId && tx.payTo) {
          // Find all users who might want to apply this transaction
          debtMap.forEach((debt, userId) => {
            const user = allUsers.find(u => u.id === userId);
            if (user && (user.name === tx.payTo || user.username === tx.payTo)) {
              debt.yourTransactionsToApply.push(tx);
            }
          });
        }
      });

      // Calculate net amounts and convert to array
      const debtArray = Array.from(debtMap.values()).map(debt => ({
        ...debt,
        net: debt.youOwe - debt.theyOwe
      })).filter(debt => debt.youOwe > 0 || debt.theyOwe > 0);

      setDebts(debtArray);
    } catch (error) {
      console.error('Error loading debt summary:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleCreateSettlement = async () => {
    if (!selectedDebtor || selectedTransactions.length === 0) return;

    try {
      setLoading(true);
      
      // Create settlements for each selected transaction
      for (const txId of selectedTransactions) {
        await createSettlement({
          transactionId: txId,
          notes: settlementNotes
        });
      }

      // Reset and reload
      setSelectedDebtor(null);
      setSelectedTransactions([]);
      setSettlementNotes('');
      await loadDebtSummary();
      
      alert('Settlement(s) created successfully!');
    } catch (error) {
      console.error('Error creating settlements:', error);
      alert('Failed to create settlements. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  const selectedDebt = debts.find(d => d.userId === selectedDebtor);
  const totalSelected = selectedTransactions.reduce((sum, txId) => {
    const tx = selectedDebt?.yourTransactionsToApply.find(t => t.id === txId);
    return sum + (tx?.amount || 0);
  }, 0);

  if (loading) {
    return <div className="p-4">Loading debt summary...</div>;
  }

  return (
    <div className="p-4">
      <h2 className="text-2xl font-bold mb-6">Debt Summary</h2>
      
      {debts.length === 0 ? (
        <p className="text-gray-500">No outstanding debts</p>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* Debt List */}
          <div className="space-y-4">
            <h3 className="text-lg font-semibold mb-3">Outstanding Balances</h3>
            {debts.map(debt => (
              <div
                key={debt.userId}
                onClick={() => setSelectedDebtor(debt.userId)}
                className={`border rounded-lg p-4 cursor-pointer transition ${
                  selectedDebtor === debt.userId ? 'border-blue-500 bg-blue-50' : 'hover:bg-gray-50'
                }`}
              >
                <div className="flex justify-between items-start">
                  <div>
                    <h4 className="font-medium">{debt.userName}</h4>
                    <div className="text-sm text-gray-600 mt-1">
                      {debt.youOwe > 0 && (
                        <p>You owe: {formatMoney(debt.youOwe)}</p>
                      )}
                      {debt.theyOwe > 0 && (
                        <p>They owe: {formatMoney(debt.theyOwe)}</p>
                      )}
                    </div>
                  </div>
                  <div className="text-right">
                    <p className={`font-bold ${debt.net > 0 ? 'text-red-600' : 'text-green-600'}`}>
                      {debt.net > 0 ? 'You owe' : 'They owe'}
                    </p>
                    <p className="text-lg font-bold">
                      {formatMoney(Math.abs(debt.net))}
                    </p>
                  </div>
                </div>
              </div>
            ))}
          </div>

          {/* Settlement Creator */}
          {selectedDebtor && selectedDebt && (
            <div className="border rounded-lg p-4">
              <h3 className="text-lg font-semibold mb-3">
                Create Settlement with {selectedDebt.userName}
              </h3>
              
              {selectedDebt.youOwe > 0 ? (
                <>
                  <p className="text-sm text-gray-600 mb-3">
                    You owe {selectedDebt.userName} {formatMoney(selectedDebt.youOwe)}. 
                    Select YOUR transactions (where others owe you) to apply against this debt.
                  </p>
                  
                  <div className="space-y-2 mb-4 max-h-60 overflow-y-auto">
                    <p className="text-sm font-medium">Your transactions to apply:</p>
                    {selectedDebt.yourTransactionsToApply.length === 0 ? (
                      <p className="text-sm text-gray-500">
                        No transactions available. You need transactions where others owe you money to offset this debt.
                      </p>
                    ) : (
                      selectedDebt.yourTransactionsToApply.map(tx => (
                        <label key={tx.id} className="flex items-start space-x-2 p-2 hover:bg-gray-50 rounded">
                          <input
                            type="checkbox"
                            checked={selectedTransactions.includes(tx.id)}
                            onChange={(e) => {
                              if (e.target.checked) {
                                setSelectedTransactions([...selectedTransactions, tx.id]);
                              } else {
                                setSelectedTransactions(selectedTransactions.filter(id => id !== tx.id));
                              }
                            }}
                            className="mt-1"
                          />
                          <div className="flex-1">
                            <p className="text-sm">{tx.payTo} - {formatMoney(tx.amount)}</p>
                            <p className="text-xs text-gray-500">{tx.note || 'No note'}</p>
                          </div>
                        </label>
                      ))
                    )}
                  </div>

                  {selectedTransactions.length > 0 && (
                    <div className="mb-4 p-2 bg-blue-50 rounded">
                      <p className="text-sm">
                        Total selected: {formatMoney(totalSelected)} 
                        {totalSelected > selectedDebt.youOwe && (
                          <span className="text-orange-600 ml-2">
                            (Exceeds debt by {formatMoney(totalSelected - selectedDebt.youOwe)})
                          </span>
                        )}
                      </p>
                    </div>
                  )}
                </>
              ) : (
                <p className="text-sm text-gray-600 mb-3">
                  {selectedDebt.userName} owes you {formatMoney(selectedDebt.theyOwe)}. 
                  They can create settlements to apply their transactions against this debt.
                </p>
              )}

              <div className="mb-4">
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Notes (optional)
                </label>
                <textarea
                  value={settlementNotes}
                  onChange={(e) => setSettlementNotes(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-md"
                  rows={2}
                  placeholder="Add any notes about this settlement..."
                />
              </div>

              <div className="flex justify-end space-x-3">
                <button
                  onClick={() => {
                    setSelectedDebtor(null);
                    setSelectedTransactions([]);
                    setSettlementNotes('');
                  }}
                  className="px-4 py-2 text-sm font-medium text-gray-700 bg-gray-100 hover:bg-gray-200 rounded-md"
                >
                  Cancel
                </button>
                <button
                  onClick={handleCreateSettlement}
                  disabled={selectedTransactions.length === 0 || loading}
                  className="px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md disabled:opacity-50"
                >
                  Create Settlement
                </button>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}