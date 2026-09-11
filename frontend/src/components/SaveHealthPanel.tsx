import { useCallback, useEffect, useState } from 'react';
import toast from '../lib/toast';
import { GetSaveInventoryIntegrityReport, RepairAllLoadedSlots, RepairDuplicateInventoryIndices, RunDiagnosticsAllLoaded } from '../../wailsjs/go/main/App';
import { application as main } from '../../wailsjs/go/models';
import { applyRepairsLoaded, scanRepairIssuesLoaded, type RepairIssueReport } from '../lib/repairIssues';
import { ACTION_REMOVE_RECORD, CODE_DUPLICATE_GOODS_ROW, CODE_SEAMLESS_COOP_ITEM, summariseReport, targetsForCode, type HealthSummary } from '../lib/saveHealth';

// SaveHealthPanel runs every load-time check the editor has for the selected
// character in one pass and offers the one-click fixes that are safe to apply
// without picking rows by hand:
//   - inventory integrity (duplicate acquisition indices / duplicate Physick),
//   - corruption diagnostics (RunDiagnosticsAllLoaded, repairable subset),
//   - Repair Issues scan, with shortcuts for duplicate goods rows and for
//     stripping Seamless Co-op mod items.
// Anything finer-grained goes through the Inventory Issues modal (onOpenIssues).
interface Props {
    charIndex: number;
    platform: string | null;
    saveLoadKey: number;
    onOpenIssues: (report: RepairIssueReport) => void;
    onMutate?: () => void;
}

interface HealthState {
    report: RepairIssueReport;
    summary: HealthSummary;
    integrity: main.SlotInventoryIntegrityReport | null;
    integrityClean: boolean;
    diagnostics: main.SlotDiagResult | null;
    diagnosticsCanRepair: boolean;
}

type Busy = 'idle' | 'scanning' | 'fix-duplicates' | 'remove-coop' | 'repair-integrity' | 'repair-diagnostics';

export function SaveHealthPanel({ charIndex, platform, saveLoadKey, onOpenIssues, onMutate }: Props) {
    const [state, setState] = useState<HealthState | null>(null);
    const [busy, setBusy] = useState<Busy>('idle');
    const [confirmCoop, setConfirmCoop] = useState(false);
    const [lastResult, setLastResult] = useState<string | null>(null);

    const runChecks = useCallback(async () => {
        if (!platform) return;
        setBusy('scanning');
        setConfirmCoop(false);
        try {
            const [report, integrity, diagnostics] = await Promise.all([
                scanRepairIssuesLoaded(charIndex),
                GetSaveInventoryIntegrityReport(),
                RunDiagnosticsAllLoaded(),
            ]);
            const slotIntegrity = integrity.slots?.find(s => s.slotIndex === charIndex) ?? null;
            const slotDiag = diagnostics.slots?.find(s => s.slotIndex === charIndex) ?? null;
            setState({
                report,
                summary: summariseReport(report),
                integrity: slotIntegrity,
                integrityClean: slotIntegrity === null || (slotIntegrity.duplicateEntryCount === 0 && slotIntegrity.conflictingIndexCount === 0),
                diagnostics: slotDiag,
                diagnosticsCanRepair: diagnostics.canRepair,
            });
        } catch (err) {
            toast.error('Save health check failed: ' + String(err));
        } finally {
            setBusy('idle');
        }
    }, [charIndex, platform]);

    useEffect(() => { void runChecks(); }, [runChecks, saveLoadKey]);

    const applyTargets = async (label: Busy, code: string, what: string) => {
        if (!state || busy !== 'idle') return;
        const targets = targetsForCode(state.report, code, ACTION_REMOVE_RECORD);
        if (targets.length === 0) return;
        setBusy(label);
        try {
            const rep = await applyRepairsLoaded(charIndex, targets, false);
            const msg = `${what}: ${rep.applied} removed` + (rep.failed > 0 ? `, ${rep.failed} failed` : '') + (rep.skipped > 0 ? `, ${rep.skipped} skipped` : '');
            setLastResult(msg);
            if (rep.failed > 0) toast.error(msg); else toast.success(msg);
            onMutate?.();
        } catch (err) {
            toast.error(`${what} failed: ` + String(err));
        } finally {
            setBusy('idle');
            setConfirmCoop(false);
            await runChecks();
        }
    };

    const repairIntegrity = async () => {
        if (busy !== 'idle') return;
        setBusy('repair-integrity');
        try {
            const rep = await RepairDuplicateInventoryIndices(charIndex);
            const msg = `Integrity repair: ${rep.changed ?? 0} record(s) changed`;
            setLastResult(msg);
            toast.success(msg);
            onMutate?.();
        } catch (err) {
            toast.error('Integrity repair failed: ' + String(err));
        } finally {
            setBusy('idle');
            await runChecks();
        }
    };

    const repairDiagnostics = async () => {
        if (busy !== 'idle') return;
        setBusy('repair-diagnostics');
        try {
            const rep = await RepairAllLoadedSlots();
            const msg = `Diagnostics repair: ${rep.fixed?.length ?? 0} fixed, ${rep.skipped?.length ?? 0} skipped`;
            setLastResult(msg);
            toast.success(msg);
            onMutate?.();
        } catch (err) {
            toast.error('Diagnostics repair failed: ' + String(err));
        } finally {
            setBusy('idle');
            await runChecks();
        }
    };

    const btn = 'px-3 py-2 rounded text-[9px] font-black uppercase tracking-widest transition-all disabled:opacity-50 disabled:cursor-not-allowed';
    const btnPrimary = `${btn} bg-primary text-primary-foreground hover:brightness-110 active:scale-95 shadow-sm`;
    const btnSecondary = `${btn} bg-muted/30 text-foreground border border-border hover:bg-muted/50`;
    const btnDanger = `${btn} bg-red-500/15 text-red-400 border border-red-500/40 hover:bg-red-500/25`;
    const statusPill = (ok: boolean, okText: string, badText: string) => (
        <span className={`px-2 py-0.5 rounded text-[8px] font-black uppercase tracking-widest border ${ok ? 'bg-green-500/10 text-green-400 border-green-500/30' : 'bg-yellow-500/10 text-warning-foreground border-yellow-500/30'}`}>
            {ok ? okText : badText}
        </span>
    );

    if (!platform) {
        return (
            <div className="card px-4 py-3 text-[10px] text-muted-foreground" data-testid="save-health-empty">
                Load a save to run the health check.
            </div>
        );
    }

    const s = state?.summary;
    const diagIssues = state?.diagnostics?.issues?.filter(i => i.severity !== 'info') ?? [];

    return (
        <div className="card px-4 py-3 space-y-3" data-testid="save-health-panel">
            <div className="flex items-center justify-between gap-2">
                <p className="text-[9px] text-muted-foreground leading-relaxed">
                    Runs the inventory integrity gate, the corruption diagnostics and the Repair Issues scan for the selected character.
                </p>
                <button type="button" onClick={() => void runChecks()} disabled={busy !== 'idle'} className={btnSecondary} data-testid="save-health-rerun">
                    {busy === 'scanning' ? 'Checking…' : 'Re-run'}
                </button>
            </div>

            {state && s && (
                <div className="grid grid-cols-1 md:grid-cols-3 gap-3 text-[10px]">
                    <div className="rounded border border-border/50 bg-background/30 p-3 space-y-2" data-testid="health-integrity">
                        <div className="flex items-center justify-between">
                            <span className="font-black uppercase tracking-widest text-muted-foreground text-[8px]">Inventory integrity</span>
                            {statusPill(state.integrityClean, 'clean', 'needs repair')}
                        </div>
                        {!state.integrityClean && state.integrity && (
                            <>
                                <p className="text-muted-foreground">
                                    {state.integrity.conflictingIndexCount} conflicting acquisition index(es), {state.integrity.duplicateEntryCount} duplicate record(s).
                                </p>
                                <button type="button" onClick={() => void repairIntegrity()} disabled={busy !== 'idle'} className={btnPrimary}>
                                    {busy === 'repair-integrity' ? 'Repairing…' : 'Repair integrity'}
                                </button>
                            </>
                        )}
                    </div>

                    <div className="rounded border border-border/50 bg-background/30 p-3 space-y-2" data-testid="health-diagnostics">
                        <div className="flex items-center justify-between">
                            <span className="font-black uppercase tracking-widest text-muted-foreground text-[8px]">Corruption diagnostics</span>
                            {statusPill(diagIssues.length === 0, 'clean', `${diagIssues.length} finding(s)`)}
                        </div>
                        {diagIssues.length > 0 && (
                            <ul className="space-y-0.5 text-muted-foreground">
                                {diagIssues.slice(0, 5).map((i, idx) => <li key={idx}>• {i.description}</li>)}
                                {diagIssues.length > 5 && <li>… {diagIssues.length - 5} more</li>}
                            </ul>
                        )}
                        {state.diagnosticsCanRepair && (
                            <button type="button" onClick={() => void repairDiagnostics()} disabled={busy !== 'idle'} className={btnPrimary}>
                                {busy === 'repair-diagnostics' ? 'Repairing…' : 'Repair all loaded slots'}
                            </button>
                        )}
                    </div>

                    <div className="rounded border border-border/50 bg-background/30 p-3 space-y-2" data-testid="health-issues">
                        <div className="flex items-center justify-between">
                            <span className="font-black uppercase tracking-widest text-muted-foreground text-[8px]">Repair issues</span>
                            {statusPill(s.errors === 0 && s.warnings === 0, s.infos > 0 ? `${s.infos} info` : 'clean', `${s.errors} error(s), ${s.warnings} warning(s)`)}
                        </div>
                        {Object.keys(s.byCode).length > 0 && (
                            <ul className="space-y-0.5 text-muted-foreground" data-testid="health-issue-codes">
                                {Object.entries(s.byCode).sort(([a], [b]) => a.localeCompare(b)).map(([code, n]) => (
                                    <li key={code} className="flex justify-between gap-2"><span>{code.replaceAll('_', ' ')}</span><span className="tabular-nums font-bold text-foreground">{n}</span></li>
                                ))}
                            </ul>
                        )}
                        <button type="button" onClick={() => onOpenIssues(state.report)} disabled={busy !== 'idle'} className={btnSecondary}>
                            Open Repair Issues
                        </button>
                    </div>
                </div>
            )}

            {state && s && (s.duplicateGoodsRows > 0 || s.seamlessCoopItems > 0) && (
                <div className="flex flex-wrap items-center gap-2 pt-1 border-t border-border/40" data-testid="health-actions">
                    {s.duplicateGoodsRows > 0 && (
                        <button
                            type="button"
                            onClick={() => void applyTargets('fix-duplicates', CODE_DUPLICATE_GOODS_ROW, 'Duplicate goods rows')}
                            disabled={busy !== 'idle'}
                            className={btnPrimary}
                        >
                            {busy === 'fix-duplicates' ? 'Fixing…' : `Fix duplicate goods rows (${s.duplicateGoodsRows})`}
                        </button>
                    )}
                    {s.seamlessCoopItems > 0 && !confirmCoop && (
                        <button type="button" onClick={() => setConfirmCoop(true)} disabled={busy !== 'idle'} className={btnSecondary}>
                            Remove Seamless Co-op items ({s.seamlessCoopItems})
                        </button>
                    )}
                    {s.seamlessCoopItems > 0 && confirmCoop && (
                        <div className="flex flex-wrap items-center gap-2 rounded border border-red-500/30 bg-red-500/5 px-3 py-2" data-testid="health-coop-confirm">
                            <span className="text-[9px] text-warning-foreground">
                                Removes every Seamless Co-op item row from this character ({s.seamlessCoopItems}). The mod re-grants its items on the next co-op launch; a vanilla launch will not miss them.
                            </span>
                            <button
                                type="button"
                                onClick={() => void applyTargets('remove-coop', CODE_SEAMLESS_COOP_ITEM, 'Seamless Co-op items')}
                                disabled={busy !== 'idle'}
                                className={btnDanger}
                            >
                                {busy === 'remove-coop' ? 'Removing…' : 'Confirm removal'}
                            </button>
                            <button type="button" onClick={() => setConfirmCoop(false)} disabled={busy !== 'idle'} className={btnSecondary}>
                                Cancel
                            </button>
                        </div>
                    )}
                </div>
            )}

            {lastResult && <p className="text-[9px] text-muted-foreground" data-testid="health-last-result">{lastResult}</p>}
        </div>
    );
}
