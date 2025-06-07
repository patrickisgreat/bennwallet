export interface ReportFilter {
  startDate: string;
  endDate: string;
  category: string;
  payTo: string;
  enteredBy: string;
  paid: boolean;
  optional: boolean;
  transactionDateMonth?: number;
  transactionDateYear?: number;
}

export interface CategoryTotal {
  category?: string;
  categoryId?: string;
  categoryName?: string;
  total?: number;
  amount?: number;
  count?: number;
}

export interface SettlementReport {
  month: string;
  year: number;
  monthName: string;
  totalOwed: number;
  totalPaid: number;
  netAmount: number;
  categoryTotals: CategoryTotal[];
  settlementDeductions: CategoryTotal[];
}

export interface ReportResponse {
  categoryTotals?: CategoryTotal[];
  settlementData?: SettlementReport;
}
