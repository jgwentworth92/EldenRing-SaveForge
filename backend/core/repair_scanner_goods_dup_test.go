package core

import "testing"

// duplicate_goods_row: a goods (0xB0) handle occupying two separate rows of the
// SAME container. The game tolerates it, but the Rust ER-Save-Editor refuses
// such saves outright ("Save file has irregular data!"), so the scanner
// surfaces it with remove_record as the default for every row after the first.
// Observed in the wild on Seamless Co-op saves whose mod items were re-granted.

func TestScanRepairIssues_DuplicateGoodsRow_SameContainer(t *testing.T) {
	slot := &SaveSlot{
		GaMap: map[uint32]uint32{},
		Inventory: EquipInventoryData{
			CommonItems: []InventoryItem{
				{GaItemHandle: testHandleSmithingStone, Quantity: 10, Index: 500},
				{GaItemHandle: 0xB0002775, Quantity: 3, Index: 501}, // unrelated goods row in between
				{GaItemHandle: testHandleSmithingStone, Quantity: 1, Index: 502},
				{GaItemHandle: testHandleSmithingStone, Quantity: 1, Index: 503},
			},
		},
	}

	var got []RepairIssue
	for _, iss := range ScanRepairIssues(0, slot) {
		if iss.Key.Code == RepairCodeDuplicateGoodsRow {
			got = append(got, iss)
		}
	}
	if len(got) != 2 {
		t.Fatalf("want 2 duplicate_goods_row issues (rows 2 and 3), got %d: %+v", len(got), got)
	}
	for i, iss := range got {
		wantRow := 2 + i
		if iss.Key.Row != wantRow {
			t.Errorf("issue %d: row = %d, want %d (first row must never be flagged)", i, iss.Key.Row, wantRow)
		}
		if iss.Key.Scope != repairScopeInventoryCommon {
			t.Errorf("issue %d: scope = %q, want %q", i, iss.Key.Scope, repairScopeInventoryCommon)
		}
		if iss.Key.Handle != testHandleSmithingStone {
			t.Errorf("issue %d: handle = 0x%08X", i, iss.Key.Handle)
		}
		if iss.Severity != repairSeverityWarning {
			t.Errorf("issue %d: severity = %q, want warning (the game accepts these rows)", i, iss.Severity)
		}
		if iss.DefaultAction != RepairActionRemoveRecord {
			t.Errorf("issue %d: default action = %q, want %q", i, iss.DefaultAction, RepairActionRemoveRecord)
		}
		if iss.Fingerprint == "" || iss.IssueID == "" {
			t.Errorf("issue %d: fingerprint/issueID must be set for the apply path", i)
		}
	}
}

// Inventory and storage are separate containers: the same goods handle in both
// is the normal transfer state (see TestScanRepairIssues_GoodsDuplicateHandle_NotAnError)
// and must not be reported as a duplicate goods row.
func TestScanRepairIssues_DuplicateGoodsRow_AcrossContainersIsClean(t *testing.T) {
	slot := &SaveSlot{
		GaMap: map[uint32]uint32{},
		Inventory: EquipInventoryData{
			CommonItems: []InventoryItem{{GaItemHandle: testHandleSmithingStone, Quantity: 10, Index: 500}},
			KeyItems:    []InventoryItem{{GaItemHandle: testHandleSmithingStone, Quantity: 1, Index: 501}},
		},
		Storage: EquipInventoryData{
			CommonItems: []InventoryItem{{GaItemHandle: testHandleSmithingStone, Quantity: 10, Index: 502}},
		},
	}
	for _, iss := range ScanRepairIssues(0, slot) {
		if iss.Key.Code == RepairCodeDuplicateGoodsRow {
			t.Errorf("goods handle repeated across containers must not be flagged: %q", iss.DebugKey)
		}
	}
}

// Only goods (0xB0) rows are subject to the rule: talismans are handle-encoded
// too but legitimately occupy one row per copy, and the Rust validator that
// motivates this code checks the 0xB0 prefix alone.
func TestScanRepairIssues_DuplicateGoodsRow_TalismansExempt(t *testing.T) {
	const talisman = uint32(0xA00017B6)
	slot := &SaveSlot{
		GaMap: map[uint32]uint32{},
		Inventory: EquipInventoryData{
			CommonItems: []InventoryItem{
				{GaItemHandle: talisman, Quantity: 1, Index: 500},
				{GaItemHandle: talisman, Quantity: 1, Index: 501},
			},
		},
	}
	for _, iss := range ScanRepairIssues(0, slot) {
		if iss.Key.Code == RepairCodeDuplicateGoodsRow {
			t.Errorf("talisman rows must not be flagged as duplicate goods rows: %q", iss.DebugKey)
		}
	}
}

// An unknown (mod) goods item is exactly the case seen in the wild; the rule
// keys on the handle prefix, not on DB resolution, so it still fires.
func TestScanRepairIssues_DuplicateGoodsRow_UnknownItemStillFlagged(t *testing.T) {
	const modHandle = uint32(0xB07FDE61)
	slot := &SaveSlot{
		GaMap: map[uint32]uint32{},
		Inventory: EquipInventoryData{
			CommonItems: []InventoryItem{
				{GaItemHandle: modHandle, Quantity: 1, Index: 500},
				{GaItemHandle: modHandle, Quantity: 1, Index: 501},
			},
		},
	}
	n := 0
	for _, iss := range ScanRepairIssues(0, slot) {
		if iss.Key.Code == RepairCodeDuplicateGoodsRow {
			n++
			if iss.Key.Row != 1 {
				t.Errorf("row = %d, want 1", iss.Key.Row)
			}
		}
	}
	if n != 1 {
		t.Fatalf("want exactly 1 duplicate_goods_row issue, got %d", n)
	}
}
