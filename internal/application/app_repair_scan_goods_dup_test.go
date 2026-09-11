package application

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// duplicate_goods_row must offer remove_record (default) and leave_unchanged,
// so the Inventory Issues modal can repair it through the existing apply path.
func TestRepairActionsForCode_DuplicateGoodsRow(t *testing.T) {
	actions, def := repairActionsForCode(core.RepairCodeDuplicateGoodsRow)
	if def != core.RepairActionRemoveRecord {
		t.Fatalf("default = %q, want %q", def, core.RepairActionRemoveRecord)
	}
	ids := map[string]bool{}
	for _, a := range actions {
		ids[a.ID] = true
		if a.Label == "" || a.Label == a.ID {
			t.Errorf("action %q has no readable label", a.ID)
		}
	}
	if !ids[core.RepairActionRemoveRecord] || !ids[RepairActionLeaveUnchanged] {
		t.Errorf("actions = %v, want remove_record and leave_unchanged", actions)
	}
}

// End to end through the app: scan a fixture with a duplicated goods row,
// apply the default action, and confirm the second row is gone while the
// first survives untouched.
func TestApplyRepairs_DuplicateGoodsRow_RemovesSecondRowOnly(t *testing.T) {
	const handle = uint32(0xB0002774) // Smithing Stone [1]
	app := repairFixture([]core.InventoryItem{
		{GaItemHandle: handle, Quantity: 10, Index: 500},
		{GaItemHandle: 0xB0002775, Quantity: 2, Index: 501},
		{GaItemHandle: handle, Quantity: 1, Index: 502},
	}, nil)

	report, err := app.ScanRepairIssuesLoaded(0)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	var targets []RepairApplyTarget
	for _, iss := range report.Issues {
		if iss.Key.Code != core.RepairCodeDuplicateGoodsRow {
			continue
		}
		targets = append(targets, RepairApplyTarget{IssueID: iss.IssueID, Key: iss.Key, Fingerprint: iss.Fingerprint, SelectedAction: iss.DefaultAction})
	}
	if len(targets) != 1 {
		t.Fatalf("want 1 duplicate_goods_row target, got %d", len(targets))
	}
	if targets[0].Key.Row != 2 {
		t.Fatalf("target row = %d, want 2", targets[0].Key.Row)
	}

	rep, err := app.ApplyRepairsLoaded(0, targets, true)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if rep.Failed != 0 || rep.Applied != 1 {
		t.Fatalf("apply report: applied=%d failed=%d (%+v)", rep.Applied, rep.Failed, rep.Results)
	}

	items := app.save.Slots[0].Inventory.CommonItems
	count := 0
	for _, it := range items {
		if it.GaItemHandle == handle {
			count++
			if it.Quantity != 10 {
				t.Errorf("surviving row qty = %d, want 10 (first row must be the one kept)", it.Quantity)
			}
		}
	}
	if count != 1 {
		t.Errorf("rows with handle after repair = %d, want 1", count)
	}

	after, err := app.ScanRepairIssuesLoaded(0)
	if err != nil {
		t.Fatalf("rescan: %v", err)
	}
	for _, iss := range after.Issues {
		if iss.Key.Code == core.RepairCodeDuplicateGoodsRow {
			t.Errorf("issue still reported after repair: %s", iss.Description)
		}
	}
}
