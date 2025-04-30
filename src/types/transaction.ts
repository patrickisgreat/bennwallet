export interface Transaction {
    id: string; // unique id
    entered: string; // ISO date string for when the transaction was entered
    transactionDate: string; // ISO date string for when the transaction occurred
    payTo: 'Sarah' | 'Patrick';
    amount: number;
    note: string;
    category: string;
    paid: boolean;
    paidDate?: string; // ISO date string, optional
    enteredBy: string; // who entered the transaction
    optional: boolean; // indicates if transaction is optional
}
  