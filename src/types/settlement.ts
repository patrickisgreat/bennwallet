import { Transaction } from './transaction';

export interface Settlement {
  id: string;
  creatorId: string;
  recipientId: string;
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
  details?: Record<string, unknown>;
  createdAt: string;
}

export interface SettlementSummary {
  id: string;
  creatorId: string;
  creatorName: string;
  recipientId: string;
  recipientName: string;
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

export interface SettlementDetails {
  [key: string]: string | number | boolean | null | undefined;
}
