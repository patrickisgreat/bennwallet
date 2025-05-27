import { Category } from './category';

export interface Transaction {
  id: string; // unique id
  entered: string; // ISO date string for when the transaction was entered
  transactionDate: string; // ISO date string for when the transaction occurred
  paidBy?: string; // Who paid for this expense (defaults to enteredBy)
  owedBy?: string; // Who owes money for this expense
  payTo?: string; // Deprecated - kept for backward compatibility
  amount: number;
  note: string;
  category: string;
  paid: boolean;
  paidDate?: string; // ISO date string, optional
  enteredBy: string; // who entered the transaction
  optional: boolean; // indicates if transaction is optional
  inSettlement?: boolean; // whether this transaction is part of a settlement
  categories?: Category[]; // array of categories associated with this transaction
}
