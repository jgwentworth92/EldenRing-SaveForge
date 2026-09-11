import { describe, expect, it } from 'vitest';
import { shouldAutoOpenOnLoad, type RepairIssue, type RepairIssueReport } from './repairIssues';

function issue(code: string, severity: string): RepairIssue {
    return {
        issueID: 'issue-' + code,
        debugKey: 'slot:0|domain:inventory|code:' + code + '|scope:inventory_common|row:0',
        fingerprint: 'abc',
        key: { slot: 0, domain: 'inventory', code, scope: 'inventory_common', row: 0, handle: 0xB07FDE61 },
        description: code,
        severity,
        actions: [{ id: 'no_action', label: 'No action' }],
        defaultAction: 'no_action',
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

describe('shouldAutoOpenOnLoad', () => {
    // The automatic on-load scan must stay issue-only: a clean load never pops a modal.
    it('is false for a clean report', () => {
        expect(shouldAutoOpenOnLoad(report([]))).toBe(false);
    });

    it('is true when the report has an error issue', () => {
        expect(shouldAutoOpenOnLoad(report([issue('quantity_zero', 'error')]))).toBe(true);
    });

    it('is true when the report has a warning issue', () => {
        expect(shouldAutoOpenOnLoad(report([issue('duplicate_goods_row', 'warning')]))).toBe(true);
    });

    // Every Seamless Co-op character carries the mod's items; an info-only
    // report must not interrupt the load.
    it('is false when every issue is informational', () => {
        expect(shouldAutoOpenOnLoad(report([issue('seamless_coop_item', 'info'), issue('seamless_coop_item', 'info')]))).toBe(false);
    });

    it('is true when informational issues are mixed with a real one', () => {
        expect(shouldAutoOpenOnLoad(report([issue('seamless_coop_item', 'info'), issue('duplicate_goods_row', 'warning')]))).toBe(true);
    });
});
