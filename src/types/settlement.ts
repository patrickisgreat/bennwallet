import { Transaction } from './transaction';

export interface Settlement {
  id: string;
  createdBy: string;
  createdFor: string;
  totalAmount: number;
  remainingAmount: number;
  status: 'active' | 'completed' | 'cancelled';
  createdAt: string;
  updatedAt: string;
  completedAt?: string;
  notes?: string;
  items?: SettlementItem[];
  history?: SettlementHistory[];
}

export interface SettlementItem {
  id: number;
  settlementId: string;
  transactionId: string;
  appliedAmount: number;
  createdAt: string;
  createdBy: string;
  transaction?: Transaction;
}

export interface SettlementHistory {
  id: number;
  settlementId: string;
  action: 'created' | 'transaction_applied' | 'transaction_removed' | 'completed' | 'cancelled';
  actorId: string;
  transactionId?: string;
  amount?: number;
  details?: Record<string, any>;
  createdAt: string;
}

export interface SettlementSummary {
  id: string;
  createdBy: string;
  createdByName: string;
  createdFor: string;
  createdForName: string;
  totalAmount: number;
  remainingAmount: number;
  status: 'active' | 'completed' | 'cancelled';
  createdAt: string;
  itemCount: number;
}

export interface CreateSettlementRequest {
  transactionId: string;
  notes?: string;
}

export interface ApplyTransactionRequest {
  transactionId: string;
  amount: number;
}