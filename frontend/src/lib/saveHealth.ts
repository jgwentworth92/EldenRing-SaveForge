import { makeRepairTarget, type RepairApplyTarget, type RepairIssueReport } from './repairIssues';

// Issue codes the Save Health panel offers one-click actions for. They mirror
// backend/core/repair_scanner.go and must stay in sync with it.
export const CODE_DUPLICATE_GOODS_ROW = 'duplicate_goods_row';
export const CODE_SEAMLESS_COOP_ITEM = 'seamless_coop_item';
export const ACTION_REMOVE_RECORD = 'remove_record';

export interface HealthSummary {
    errors: number;
    warnings: number;
    infos: number;
    byCode: Record<string, number>;
    // Rows the two one-click actions would touch. Counted from issues that
    // actually offer remove_record, so the buttons never promise more than the
    // apply path will accept.
    duplicateGoodsRows: number;
    seamlessCoopItems: number;
}

export function summariseReport(report: RepairIssueReport): HealthSummary {
    const summary: HealthSummary = { errors: 0, warnings: 0, infos: 0, byCode: {}, duplicateGoodsRows: 0, seamlessCoopItems: 0 };
    for (const issue of report.issues) {
        const code = issue.key.code;
        summary.byCode[code] = (summary.byCode[code] ?? 0) + 1;
        if (issue.severity === 'error') summary.errors++;
        else if (issue.severity === 'warning') summary.warnings++;
        else summary.infos++;
    }
    summary.duplicateGoodsRows = targetsForCode(report, CODE_DUPLICATE_GOODS_ROW, ACTION_REMOVE_RECORD).length;
    summary.seamlessCoopItems = targetsForCode(report, CODE_SEAMLESS_COOP_ITEM, ACTION_REMOVE_RECORD).length;
    return summary;
}

// targetsForCode builds apply targets for every issue of one code that offers
// the requested action. Issues that do not list the action are skipped rather
// than forced, so a report-only issue can never be turned into a mutation.
export function targetsForCode(report: RepairIssueReport, code: string, action: string): RepairApplyTarget[] {
    return report.issues
        .filter(issue => issue.key.code === code && issue.actions.some(a => a.id === action))
        .map(issue => makeRepairTarget(issue, action));
}
