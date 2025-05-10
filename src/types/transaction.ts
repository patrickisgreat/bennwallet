import { Category } from './category';

export interface Transaction {
    id: string; // unique id
    entered: string; // ISO date string for when the transaction was entered
    transactionDate: string; // ISO date string for when the transaction occurred
    payTo: string; // Who is being paid
    amount: number;
    note: string;
    category: string;
    paid: boolean;
    paidDate?: string; // ISO date string, optional
    enteredBy: string; // who entered the transaction
    optional: boolean; // indicates if transaction is optional
    categories?: Category[]; // array of categories associated with this transaction
}
  