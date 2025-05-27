import SettlementManager from '../components/SettlementManager';
import DebtSummary from '../components/DebtSummary';

export default function SettlementsPage() {
  return (
    <div>
      <DebtSummary />
      <div className="mt-8 border-t pt-8">
        <SettlementManager />
      </div>
    </div>
  );
}