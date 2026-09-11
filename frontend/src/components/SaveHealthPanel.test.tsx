import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { RepairApplyReport, RepairApplyTarget, RepairIssue, RepairIssueReport } from '../lib/repairIssues';

const scanRepairIssuesLoaded = vi.fn<(idx: number) => Promise<RepairIssueReport>>();
const applyRepairsLoaded = vi.fn<(idx: number, targets: RepairApplyTarget[], stop: boolean) => Promise<RepairApplyReport>>();
const GetSaveInventoryIntegrityReport = vi.fn();
const RunDiagnosticsAllLoaded = vi.fn();
const RepairDuplicateInventoryIndices = vi.fn();
const RepairAllLoadedSlots = vi.fn();

vi.mock('../lib/repairIssues', async (importOriginal) => ({
    ...(await importOriginal<typeof import('../lib/repairIssues')>()),
    scanRepairIssuesLoaded: (idx: number) => scanRepairIssuesLoaded(idx),
    applyRepairsLoaded: (idx: number, targets: RepairApplyTarget[], stop: boolean) => applyRepairsLoaded(idx, targets, stop),
}));

vi.mock('../../wailsjs/go/main/App', () => ({
    ScanRepairIssuesLoaded: vi.fn(),
    ApplyRepairsLoaded: vi.fn(),
    GetSaveInventoryIntegrityReport: () => GetSaveInventoryIntegrityReport(),
    RunDiagnosticsAllLoaded: () => RunDiagnosticsAllLoaded(),
    RepairDuplicateInventoryIndices: (idx: number) => RepairDuplicateInventoryIndices(idx),
    RepairAllLoadedSlots: () => RepairAllLoadedSlots(),
}));

vi.mock('../lib/toast', () => {
    const fn = vi.fn() as unknown as Record<string, unknown> & ((...args: unknown[]) => void);
    fn.success = vi.fn();
    fn.error = vi.fn();
    return { default: fn };
});

import { SaveHealthPanel } from './SaveHealthPanel';

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
        charName: 'Tarnished',
        hasIssues: issues.length > 0,
        issues,
        coverage: {
            totalPhysical: 1, resolved: 1, knownDB: 1, technicalPlaceholder: 0, unknown: 0,
            resolutionChecksApplied: 1, structuralChecksApplied: 1, categoryChecksApplied: 1,
            perCategory: {}, unknownByReason: {},
        },
    };
}

const cleanIntegrity = { clean: true, slots: [{ slotIndex: 0, characterName: 'Tarnished', active: true, duplicateEntryCount: 0, conflictingIndexCount: 0, conflicts: [] }] };
const cleanDiagnostics = { source: 'loaded', canRepair: false, slots: [{ slotIndex: 0, charName: 'Tarnished', issues: [{ severity: 'info', category: 'gaitem', description: 'GaItems: 1 used' }] }] };

function renderPanel(props: Partial<React.ComponentProps<typeof SaveHealthPanel>> = {}) {
    const onOpenIssues = vi.fn();
    const onMutate = vi.fn();
    render(<SaveHealthPanel charIndex={0} platform="PC" saveLoadKey={1} onOpenIssues={onOpenIssues} onMutate={onMutate} {...props} />);
    return { onOpenIssues, onMutate };
}

describe('SaveHealthPanel', () => {
    beforeEach(() => {
        vi.clearAllMocks();
        GetSaveInventoryIntegrityReport.mockResolvedValue(cleanIntegrity);
        RunDiagnosticsAllLoaded.mockResolvedValue(cleanDiagnostics);
    });

    it('shows a placeholder and runs nothing without a loaded save', () => {
        renderPanel({ platform: null });
        expect(screen.getByTestId('save-health-empty')).toBeInTheDocument();
        expect(scanRepairIssuesLoaded).not.toHaveBeenCalled();
    });

    it('runs all three checks on mount and reports a clean character', async () => {
        scanRepairIssuesLoaded.mockResolvedValue(report([]));
        renderPanel();
        await waitFor(() => expect(screen.getByTestId('health-issues')).toBeInTheDocument());
        expect(scanRepairIssuesLoaded).toHaveBeenCalledWith(0);
        expect(GetSaveInventoryIntegrityReport).toHaveBeenCalled();
        expect(RunDiagnosticsAllLoaded).toHaveBeenCalled();
        expect(screen.getAllByText('clean')).toHaveLength(3);
        expect(screen.queryByTestId('health-actions')).not.toBeInTheDocument();
    });

    it('offers the duplicate goods fix and applies remove_record targets for that code only', async () => {
        scanRepairIssuesLoaded.mockResolvedValue(report([
            issue('duplicate_goods_row', 'warning', 292, ['remove_record', 'leave_unchanged']),
            issue('duplicate_goods_row', 'warning', 319, ['remove_record', 'leave_unchanged']),
            issue('seamless_coop_item', 'info', 200, ['no_action', 'remove_record']),
        ]));
        applyRepairsLoaded.mockResolvedValue({ applied: 2, skipped: 0, failed: 0, needsUserInput: 0, stopped: false, results: [] });
        const { onMutate } = renderPanel();

        const fix = await screen.findByRole('button', { name: 'Fix duplicate goods rows (2)' });
        fireEvent.click(fix);

        await waitFor(() => expect(applyRepairsLoaded).toHaveBeenCalledTimes(1));
        const [idx, targets, stop] = applyRepairsLoaded.mock.calls[0];
        expect(idx).toBe(0);
        expect(stop).toBe(false);
        expect(targets.map(t => t.key.row)).toEqual([292, 319]);
        expect(targets.every(t => t.selectedAction === 'remove_record')).toBe(true);
        expect(onMutate).toHaveBeenCalled();
        // The panel re-scans after applying.
        await waitFor(() => expect(scanRepairIssuesLoaded).toHaveBeenCalledTimes(2));
        expect(screen.getByTestId('health-last-result')).toHaveTextContent('Duplicate goods rows: 2 removed');
    });

    it('requires confirmation before removing Seamless Co-op items, then removes them all', async () => {
        scanRepairIssuesLoaded.mockResolvedValue(report([
            issue('seamless_coop_item', 'info', 200, ['no_action', 'remove_record']),
            issue('seamless_coop_item', 'info', 201, ['no_action', 'remove_record']),
            issue('seamless_coop_item', 'info', 15, ['no_action', 'remove_record']),
        ]));
        applyRepairsLoaded.mockResolvedValue({ applied: 3, skipped: 0, failed: 0, needsUserInput: 0, stopped: false, results: [] });
        renderPanel();

        fireEvent.click(await screen.findByRole('button', { name: 'Remove Seamless Co-op items (3)' }));
        expect(applyRepairsLoaded).not.toHaveBeenCalled();
        expect(screen.getByTestId('health-coop-confirm')).toBeInTheDocument();

        fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
        expect(screen.queryByTestId('health-coop-confirm')).not.toBeInTheDocument();
        expect(applyRepairsLoaded).not.toHaveBeenCalled();

        fireEvent.click(screen.getByRole('button', { name: 'Remove Seamless Co-op items (3)' }));
        fireEvent.click(screen.getByRole('button', { name: 'Confirm removal' }));
        await waitFor(() => expect(applyRepairsLoaded).toHaveBeenCalledTimes(1));
        const [, targets] = applyRepairsLoaded.mock.calls[0];
        expect(targets).toHaveLength(3);
        expect(targets.every(t => t.key.code === 'seamless_coop_item' && t.selectedAction === 'remove_record')).toBe(true);
    });

    it('opens the Inventory Issues modal with the scanned report', async () => {
        const rep = report([issue('quantity_zero', 'error', 3, ['remove_record'])]);
        scanRepairIssuesLoaded.mockResolvedValue(rep);
        const { onOpenIssues } = renderPanel();
        fireEvent.click(await screen.findByRole('button', { name: 'Open Repair Issues' }));
        expect(onOpenIssues).toHaveBeenCalledWith(rep);
        expect(screen.getByText('1 error(s), 0 warning(s)')).toBeInTheDocument();
    });

    it('offers the integrity repair when the slot is dirty and re-runs afterwards', async () => {
        scanRepairIssuesLoaded.mockResolvedValue(report([]));
        GetSaveInventoryIntegrityReport.mockResolvedValue({
            clean: false,
            slots: [{ slotIndex: 0, characterName: 'Tarnished', active: true, duplicateEntryCount: 1, conflictingIndexCount: 2, conflicts: [] }],
        });
        RepairDuplicateInventoryIndices.mockResolvedValue({ changed: 2, changes: [] });
        const { onMutate } = renderPanel();

        fireEvent.click(await screen.findByRole('button', { name: 'Repair integrity' }));
        await waitFor(() => expect(RepairDuplicateInventoryIndices).toHaveBeenCalledWith(0));
        expect(onMutate).toHaveBeenCalled();
        await waitFor(() => expect(GetSaveInventoryIntegrityReport).toHaveBeenCalledTimes(2));
    });
});
