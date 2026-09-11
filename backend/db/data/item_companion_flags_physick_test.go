package data

import "testing"

// Flask of Wondrous Physick: adding the flask must also set the "obtained
// flask" flag (60020), which gates the Mix Wondrous Physick grace menu.
// Verified on a PC save: a character who collected the flask at the Third
// Church has it set, one who never did has it clear.
func TestCompanionEventFlagsForItem_WondrousPhysick(t *testing.T) {
	for _, id := range []uint32{ItemFlaskWondrousPhysick, ItemFlaskWondrousPhysickFilledRaw} {
		flags := CompanionEventFlagsForItem(id)
		if len(flags) != 1 || flags[0] != EventFlagObtainedWondrousPhysick {
			t.Errorf("0x%08X: flags = %v, want [%d]", id, flags, EventFlagObtainedWondrousPhysick)
		}
	}
	if EventFlagObtainedWondrousPhysick != 60020 {
		t.Errorf("EventFlagObtainedWondrousPhysick = %d, want 60020", EventFlagObtainedWondrousPhysick)
	}
}
