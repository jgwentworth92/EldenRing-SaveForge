package data

import "testing"

func TestSeamlessCoopItems_KnownIDs(t *testing.T) {
	cases := []struct {
		id   uint32
		name string
	}{
		{0x407FDE61, "Tiny Great Pot"},
		{0x407FDE62, "Effigy of Malenia"},
		{0x407FDE63, "Challenger's Lynchpin"},
		{0x407FDE64, "Separation Mist"},
		{0x407FDE65, "Judicator's Rulebook"},
		{0x407FDE66, "Rune Decanter"},
		{0x407FDE6C, "Seamless Co-op item 8380012"},
	}
	for _, c := range cases {
		if !IsSeamlessCoopItemID(c.id) {
			t.Errorf("0x%08X should be a Seamless Co-op item", c.id)
		}
		if name, ok := SeamlessCoopItemName(c.id); !ok || name != c.name {
			t.Errorf("0x%08X name = %q ok=%v, want %q", c.id, name, ok, c.name)
		}
	}
}

// IDs inside the reserved range but not yet tabled still count as the mod's,
// with a numbered placeholder name.
func TestSeamlessCoopItems_RangeFallback(t *testing.T) {
	const untabled = uint32(0x407FDE70) // 8380016
	if !IsSeamlessCoopItemID(untabled) {
		t.Fatalf("0x%08X inside the reserved range should be a Seamless Co-op item", untabled)
	}
	name, ok := SeamlessCoopItemName(untabled)
	if !ok || name != "Seamless Co-op item 8380016" {
		t.Errorf("name = %q ok=%v", name, ok)
	}
}

func TestSeamlessCoopItems_VanillaIDsExcluded(t *testing.T) {
	for _, id := range []uint32{
		0x400000FB, // Flask of Wondrous Physick
		0x4000006D, // Small Golden Effigy (vanilla multiplayer item)
		0x40002774, // Smithing Stone [1]
		0x407FDE60, // one below the range
		0x407FDE81, // one above the range
		0xB07FDE61, // a handle, not an itemID
	} {
		if IsSeamlessCoopItemID(id) {
			t.Errorf("0x%08X must not be classified as a Seamless Co-op item", id)
		}
		if _, ok := SeamlessCoopItemName(id); ok {
			t.Errorf("0x%08X must not have a Seamless Co-op name", id)
		}
	}
}

func TestSeamlessCoopItems_IDsSorted(t *testing.T) {
	ids := SeamlessCoopItemIDs()
	if len(ids) != 12 {
		t.Fatalf("len = %d, want 12", len(ids))
	}
	for i := 1; i < len(ids); i++ {
		if ids[i-1] >= ids[i] {
			t.Fatalf("not ascending at %d: 0x%08X >= 0x%08X", i, ids[i-1], ids[i])
		}
	}
}
