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
