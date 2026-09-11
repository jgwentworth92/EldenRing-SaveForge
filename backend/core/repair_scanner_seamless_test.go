package core

import (
	"strings"
	"testing"
)

// Seamless Co-op mod items are unknown to the vanilla DB but are not a defect:
// the mod's own regulation defines them. They get their own informational
// code instead of unknown_item_id, and the mod's zero-quantity key-item rows
// (its normal storage form) must not be offered for removal as quantity_zero.

const (
	testHandleTinyGreatPot = uint32(0xB07FDE61) // 8380001
	testHandleCoopKeyItem  = uint32(0xB07FDE67) // 8380007, stored in KeyItems with qty 0
)

func codesByRow(issues []RepairIssue, scope string) map[int][]string {
	out := map[int][]string{}
	for _, iss := range issues {
		if iss.Key.Scope == scope {
			out[iss.Key.Row] = append(out[iss.Key.Row], iss.Key.Code)
		}
	}
	return out
}

func TestScanRepairIssues_SeamlessCoopItem_NotUnknown(t *testing.T) {
	slot := &SaveSlot{
		GaMap: map[uint32]uint32{},
		Inventory: EquipInventoryData{
			CommonItems: []InventoryItem{
				{GaItemHandle: testHandleTinyGreatPot, Quantity: 1, Index: 500},
				{GaItemHandle: 0xB0FFFFFE, Quantity: 1, Index: 502}, // genuinely unknown (502: not in 500's stride-2 bucket)
			},
		},
	}
	issues := ScanRepairIssues(0, slot)
	byRow := codesByRow(issues, repairScopeInventoryCommon)

	if got := byRow[0]; len(got) != 1 || got[0] != RepairCodeSeamlessCoopItem {
		t.Fatalf("row 0 codes = %v, want [%s]", got, RepairCodeSeamlessCoopItem)
	}
	if got := byRow[1]; len(got) != 1 || got[0] != RepairCodeUnknownItemID {
		t.Fatalf("row 1 codes = %v, want [%s] (vanilla unknown must stay unknown)", got, RepairCodeUnknownItemID)
	}
	for _, iss := range issues {
		if iss.Key.Code != RepairCodeSeamlessCoopItem {
			continue
		}
		if iss.Severity != repairSeverityInfo {
			t.Errorf("severity = %q, want info", iss.Severity)
		}
		if iss.DefaultAction != RepairActionNoAction {
			t.Errorf("default = %q, want %q (never auto-remove a mod item)", iss.DefaultAction, RepairActionNoAction)
		}
		if want := "Tiny Great Pot"; !strings.Contains(iss.Description, want) {
			t.Errorf("description %q should name the item %q", iss.Description, want)
		}
	}
}

func TestScanRepairIssues_SeamlessCoopKeyItem_ZeroQuantityIsNotADefect(t *testing.T) {
	slot := &SaveSlot{
		GaMap: map[uint32]uint32{},
		Inventory: EquipInventoryData{
			KeyItems: []InventoryItem{
				{GaItemHandle: testHandleCoopKeyItem, Quantity: 0, Index: 600},
				{GaItemHandle: testHandleSmithingStone, Quantity: 0, Index: 602}, // vanilla qty 0 stays a defect
			},
		},
	}
	byRow := codesByRow(ScanRepairIssues(0, slot), repairScopeInventoryKey)
	for _, code := range byRow[0] {
		if code == RepairCodeQuantityZero || code == RepairCodeUnknownItemID {
			t.Errorf("mod key item must not be reported as %s", code)
		}
	}
	if got := byRow[0]; len(got) != 1 || got[0] != RepairCodeSeamlessCoopItem {
		t.Errorf("row 0 codes = %v, want [%s]", got, RepairCodeSeamlessCoopItem)
	}
	found := false
	for _, code := range byRow[1] {
		if code == RepairCodeQuantityZero {
			found = true
		}
	}
	if !found {
		t.Errorf("vanilla zero-quantity row must still be reported: %v", byRow[1])
	}
}
