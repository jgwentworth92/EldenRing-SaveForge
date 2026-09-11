package application

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// seamless_coop_item is informational by default (no_action) but must still
// offer remove_record so a user can strip the mod's items deliberately.
func TestRepairActionsForCode_SeamlessCoopItem(t *testing.T) {
	actions, def := repairActionsForCode(core.RepairCodeSeamlessCoopItem)
	if def != core.RepairActionNoAction {
		t.Fatalf("default = %q, want %q", def, core.RepairActionNoAction)
	}
	ids := map[string]bool{}
	for _, a := range actions {
		ids[a.ID] = true
	}
	if !ids[core.RepairActionNoAction] || !ids[core.RepairActionRemoveRecord] {
		t.Errorf("actions = %v, want no_action and remove_record", actions)
	}
}

// The DTO record for a mod item carries the mod's name so the modal does not
// show an empty name next to a hex ID.
func TestScanRepairIssuesLoaded_SeamlessCoopItem_RecordNamed(t *testing.T) {
	app := repairFixture([]core.InventoryItem{
		{GaItemHandle: 0xB07FDE62, Quantity: 1, Index: 500}, // Effigy of Malenia
	}, nil)
	report, err := app.ScanRepairIssuesLoaded(0)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	found := false
	for _, iss := range report.Issues {
		if iss.Key.Code != core.RepairCodeSeamlessCoopItem {
			continue
		}
		found = true
		if iss.Record == nil {
			t.Fatalf("record missing on %s", iss.DebugKey)
		}
		if iss.Record.Name != "Effigy of Malenia" {
			t.Errorf("record name = %q, want Effigy of Malenia", iss.Record.Name)
		}
	}
	if !found {
		t.Fatalf("no %s issue in %+v", core.RepairCodeSeamlessCoopItem, report.Issues)
	}
}
