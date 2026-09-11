package application

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// The Save Health "remove Seamless Co-op items" action is a single
// ApplyRepairsLoaded batch of remove_record targets spanning common and key
// items. Every target must apply in one pass (row addresses stay stable
// because removal clears rows in place), vanilla rows must survive, and a
// rescan must report no mod items left.
func TestApplyRepairs_RemoveAllSeamlessCoopItems_Batch(t *testing.T) {
	const vanilla = uint32(0xB0002774) // Smithing Stone [1]
	app := repairFixture(
		[]core.InventoryItem{
			{GaItemHandle: 0xB07FDE61, Quantity: 1, Index: 500}, // Tiny Great Pot
			{GaItemHandle: vanilla, Quantity: 10, Index: 502},
			{GaItemHandle: 0xB07FDE62, Quantity: 1, Index: 504}, // Effigy of Malenia
			{GaItemHandle: 0xB07FDE6C, Quantity: 1, Index: 506}, // 8380012
		},
		[]core.InventoryItem{
			{GaItemHandle: 0xB07FDE67, Quantity: 0, Index: 600}, // 8380007, mod key item, qty 0
			{GaItemHandle: 0xB07FDE68, Quantity: 0, Index: 602},
		},
	)

	report, err := app.ScanRepairIssuesLoaded(0)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	var targets []RepairApplyTarget
	for _, iss := range report.Issues {
		if iss.Key.Code == core.RepairCodeSeamlessCoopItem {
			targets = append(targets, RepairApplyTarget{IssueID: iss.IssueID, Key: iss.Key, Fingerprint: iss.Fingerprint, SelectedAction: core.RepairActionRemoveRecord})
		}
	}
	if len(targets) != 5 {
		t.Fatalf("want 5 seamless_coop_item targets, got %d", len(targets))
	}

	rep, err := app.ApplyRepairsLoaded(0, targets, false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if rep.Applied != 5 || rep.Failed != 0 || rep.Skipped != 0 {
		t.Fatalf("apply report: applied=%d failed=%d skipped=%d results=%+v", rep.Applied, rep.Failed, rep.Skipped, rep.Results)
	}

	slot := &app.save.Slots[0]
	vanillaSeen := false
	for _, it := range append(append([]core.InventoryItem(nil), slot.Inventory.CommonItems...), slot.Inventory.KeyItems...) {
		if it.GaItemHandle == 0 {
			continue
		}
		if it.GaItemHandle>>24 == 0xB0 && (it.GaItemHandle&0x0FFFFFFF) >= 0x7FDE61 {
			t.Errorf("mod row still present: 0x%08X", it.GaItemHandle)
		}
		if it.GaItemHandle == vanilla {
			vanillaSeen = true
			if it.Quantity != 10 {
				t.Errorf("vanilla row qty = %d, want 10", it.Quantity)
			}
		}
	}
	if !vanillaSeen {
		t.Errorf("vanilla row was removed")
	}

	after, err := app.ScanRepairIssuesLoaded(0)
	if err != nil {
		t.Fatalf("rescan: %v", err)
	}
	for _, iss := range after.Issues {
		if iss.Key.Code == core.RepairCodeSeamlessCoopItem {
			t.Errorf("mod item still reported after batch removal: %s", iss.Description)
		}
	}
}
