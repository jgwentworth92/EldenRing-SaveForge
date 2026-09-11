import { describe, expect, it } from 'vitest';
import type { RepairIssue, RepairIssueReport } from './repairIssues';
import { ACTION_REMOVE_RECORD, CODE_DUPLICATE_GOODS_ROW, CODE_SEAMLESS_COOP_ITEM, summariseReport, targetsForCode } from './saveHealth';

function issue(code: string, severity: string, row: number, actions: string[]): RepairIssue {
    return {
        issueID: `issue-${code}-${row}`,
        debugKey: `slot:0|domain:inventory|code:${code}|scope:inventory_common|row:${row}`,
        fingerprint: `fp-${row}`,
        key: { slot: 0, domain: 'inventory', code, scope: 'inventory_common', row, handle: 0xB07FDE61 },
        description: code,
        severity,
        actions: actions.map(id => ({ id, label: id })),
        defaultAction: actions[0],
    } as RepairIssue;
}

function report(issues: RepairIssue[]): RepairIssueReport {
    return {
        slotIndex: 0,
        charName: 'T',
        hasIssues: issues.length > 0,
        issues,
        coverage: {
            totalPhysical: 1, resolved: 1, knownDB: 1, technicalPlaceholder: 0, unknown: 0,
            resolutionChecksApplied: 1, structuralChecksApplied: 1, categoryChecksApplied: 1,
            perCategory: {}, unknownByReason: {},
        },
    };
}

describe('summariseReport', () => {
    it('counts severities and codes', () => {
        const s = summariseReport(report([
            issue('quantity_zero', 'error', 0, ['remove_record', 'leave_unchanged']),
            issue(CODE_DUPLICATE_GOODS_ROW, 'warning', 1, ['remove_record', 'leave_unchanged']),
            issue(CODE_DUPLICATE_GOODS_ROW, 'warning', 2, ['remove_record', 'leave_unchanged']),
            issue(CODE_SEAMLESS_COOP_ITEM, 'info', 3, ['no_action', 'remove_record']),
        ]));
        expect(s.errors).toBe(1);
        expect(s.warnings).toBe(2);
        expect(s.infos).toBe(1);
        expect(s.byCode).toEqual({ quantity_zero: 1, [CODE_DUPLICATE_GOODS_ROW]: 2, [CODE_SEAMLESS_COOP_ITEM]: 1 });
        expect(s.duplicateGoodsRows).toBe(2);
        expect(s.seamlessCoopItems).toBe(1);
    });

    it('is all zeros for a clean report', () => {
        const s = summariseReport(report([]));
        expect(s).toEqual({ errors: 0, warnings: 0, infos: 0, byCode: {}, duplicateGoodsRows: 0, seamlessCoopItems: 0 });
    });
});

describe('targetsForCode', () => {
    it('builds remove_record targets carrying the issue key and fingerprint', () => {
        const targets = targetsForCode(report([
            issue(CODE_SEAMLESS_COOP_ITEM, 'info', 4, ['no_action', 'remove_record']),
            issue('quantity_zero', 'error', 5, ['remove_record']),
        ]), CODE_SEAMLESS_COOP_ITEM, ACTION_REMOVE_RECORD);
        expect(targets).toHaveLength(1);
        expect(targets[0].issueID).toBe('issue-seamless_coop_item-4');
        expect(targets[0].fingerprint).toBe('fp-4');
        expect(targets[0].key.row).toBe(4);
        expect(targets[0].selectedAction).toBe('remove_record');
    });

    // A report-only issue must never be turned into a mutation by the one-click path.
    it('skips issues that do not offer the action', () => {
        const targets = targetsForCode(report([
            issue(CODE_SEAMLESS_COOP_ITEM, 'info', 6, ['no_action']),
        ]), CODE_SEAMLESS_COOP_ITEM, ACTION_REMOVE_RECORD);
        expect(targets).toHaveLength(0);
    });
});
