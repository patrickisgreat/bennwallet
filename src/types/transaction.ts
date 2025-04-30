export interface Transaction {
    id: string; // unique id
    entered: string; // ISO date string
    payTo: 'Sarah' | 'Patrick';
    amount: number;
    note: string;
    category: string;
    paid: boolean;
    paidDate?: string; // ISO date string, optional
    enteredBy: string; // who entered the transaction
}
  