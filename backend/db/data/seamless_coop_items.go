package data

// Seamless Co-op (ERSC, nexusmods.com/eldenring/mods/510) adds its own goods
// rows to the regulation it ships. They live in EquipParamGoods at 8380001+
// (itemID 0x407FDE61+) and appear only in the mod's ER0000.co2 saves. The
// vanilla item database knows nothing about them, so without this table every
// one of them scans as unknown_item_id.
//
// Names for 8380001-8380006 come from the mod's FAQ (ersc-docs.github.io/faq).
// 8380007-8380012 were observed on real .co2 saves (2026-09-10) in the same
// contiguous range: five of them sit in inventory KeyItems with quantity 0,
// which is how the mod records them, and one in CommonItems. Their names are
// not published, so they are labelled by number.
var seamlessCoopItems = map[uint32]string{
	0x407FDE61: "Tiny Great Pot",        // 8380001 — open a co-op session
	0x407FDE62: "Effigy of Malenia",     // 8380002 — join a session as cooperator
	0x407FDE63: "Challenger's Lynchpin", // 8380003 — join a session as invader
	0x407FDE64: "Separation Mist",       // 8380004 — leave the session
	0x407FDE65: "Judicator's Rulebook",  // 8380005 — friendly-fire modes
	0x407FDE66: "Rune Decanter",         // 8380006 — rune arc shop
	0x407FDE67: "Seamless Co-op item 8380007",
	0x407FDE68: "Seamless Co-op item 8380008",
	0x407FDE69: "Seamless Co-op item 8380009",
	0x407FDE6A: "Seamless Co-op item 8380010",
	0x407FDE6B: "Seamless Co-op item 8380011",
	0x407FDE6C: "Seamless Co-op item 8380012",
}

// SeamlessCoopItemRangeStart / End bound the goods IDs the mod reserves. IDs
// inside the range that are not in the table are still treated as the mod's
// (a newer mod version can add rows without a SaveForge release).
const (
	SeamlessCoopItemRangeStart = uint32(0x407FDE61) // 8380001
	SeamlessCoopItemRangeEnd   = uint32(0x407FDE80) // 8380032, generous headroom
)

// IsSeamlessCoopItemID reports whether id is a goods row added by the
// Seamless Co-op mod.
func IsSeamlessCoopItemID(id uint32) bool {
	if _, ok := seamlessCoopItems[id]; ok {
		return true
	}
	return id >= SeamlessCoopItemRangeStart && id <= SeamlessCoopItemRangeEnd
}

// SeamlessCoopItemName returns the display name for a Seamless Co-op item, or
// a numbered placeholder for an ID inside the reserved range that has no
// published name. ok is false for anything outside the mod's range.
func SeamlessCoopItemName(id uint32) (name string, ok bool) {
	if n, found := seamlessCoopItems[id]; found {
		return n, true
	}
	if id >= SeamlessCoopItemRangeStart && id <= SeamlessCoopItemRangeEnd {
		return "Seamless Co-op item " + itoa(id&0x0FFFFFFF), true
	}
	return "", false
}

// SeamlessCoopItemIDs returns every tabled ID in ascending order.
func SeamlessCoopItemIDs() []uint32 {
	out := make([]uint32, 0, len(seamlessCoopItems))
	for id := range seamlessCoopItems {
		out = append(out, id)
	}
	sortUint32s(out)
	return out
}

func itoa(v uint32) string {
	if v == 0 {
		return "0"
	}
	var b [10]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}

func sortUint32s(s []uint32) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
