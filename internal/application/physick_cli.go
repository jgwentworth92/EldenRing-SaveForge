package application

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// PhysickObtainedFlag is the event flag the game sets when the Flask of
// Wondrous Physick is picked up at the Third Church of Marika. It gates the
// "Mix Wondrous Physick" site-of-grace menu. Verified against a save where a
// character who collected the flask normally has it set and one who never
// did has it clear. Source: soulsmodding.com event flag list ("60020 —
// Obtained Flask of Wondrous Physick").
const PhysickObtainedFlag uint32 = 60020

// physickCharacter is one active slot's Physick state as shown by the CLI.
type physickCharacter struct {
	Slot       int
	Name       string
	Level      uint32
	HasFlask   bool
	FlagSet    bool
	Duplicates int
	// DupGoods counts common-inventory/storage rows whose goods (0xB0) handle
	// already appeared in an earlier row of the same container. The Rust
	// ER-Save-Editor rejects such saves outright ("Save file has irregular
	// data!"); the game itself tolerates them.
	DupGoods int
}

// PhysickCLIMain is the entry point of the standalone "give Flask of Wondrous
// Physick" command. It loads a PC / Seamless Co-op save, lists the characters,
// picks one (flag, or interactive prompt), adds the flask if missing, sets
// PhysickObtainedFlag if clear, and writes a NEW file next to the input
// (or back in place with -in-place, after a timestamped backup).
//
// With -fix-duplicates it instead removes second copies of duplicated goods
// handles (see physickCharacter.DupGoods) and touches nothing else.
//
// Returns the process exit code. stdin/stdout/stderr are injected so tests
// can drive the prompt.
func PhysickCLIMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("physick-cli", flag.ContinueOnError)
	fs.SetOutput(stderr)
	character := fs.String("character", "", "character name (case-insensitive); prompts if omitted")
	slotFlag := fs.Int("slot", -1, "character slot 0-9 (alternative to -character)")
	out := fs.String("out", "", "output path (default: <input>.physick<ext>)")
	inPlace := fs.Bool("in-place", false, "overwrite the input file (a .bak copy is made first)")
	listOnly := fs.Bool("list", false, "only list characters and their Physick state")
	fixDup := fs.Bool("fix-duplicates", false, "remove second copies of duplicated goods rows (\"Dup goods\" column) instead of adding the flask")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "usage: physick-cli [flags] <ER0000.co2 | ER0000.sl2>\n\n")
		fmt.Fprintf(stderr, "Adds the Flask of Wondrous Physick to one character and sets event flag %d\n", PhysickObtainedFlag)
		fmt.Fprintf(stderr, "(\"obtained flask\", enables Mix Wondrous Physick at a grace).\n")
		fmt.Fprintf(stderr, "Quit Elden Ring before using the output file.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	reader := bufio.NewReader(stdin)
	interactive := false
	path := fs.Arg(0)
	if path == "" {
		interactive = true
		fmt.Fprint(stdout, "Path to save file (ER0000.co2 or ER0000.sl2): ")
		path = readLine(reader)
		if path == "" {
			fs.Usage()
			return 2
		}
	}
	if interactive {
		defer pauseForEnter(reader, stdout)
	}

	app := NewApp()
	platform, err := app.LoadSaveFromPath(path)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Loaded %s (platform %s)\n\n", path, platform)

	chars := app.physickCharacters()
	if len(chars) == 0 {
		fmt.Fprintln(stderr, "error: no active characters in this save")
		return 1
	}
	printPhysickTable(stdout, chars)
	if *listOnly {
		return 0
	}

	// Pick the target character.
	target := -1
	prompted := false
	switch {
	case *slotFlag >= 0:
		for _, c := range chars {
			if c.Slot == *slotFlag {
				target = c.Slot
			}
		}
		if target < 0 {
			fmt.Fprintf(stderr, "error: slot %d is not an active character\n", *slotFlag)
			return 1
		}
	case *character != "":
		for _, c := range chars {
			if strings.EqualFold(c.Name, *character) {
				target = c.Slot
			}
		}
		if target < 0 {
			fmt.Fprintf(stderr, "error: no character named %q\n", *character)
			return 1
		}
	default:
		prompted = true
		if *fixDup {
			fmt.Fprint(stdout, "\nSlot number of the character to fix duplicates for: ")
		} else {
			fmt.Fprint(stdout, "\nSlot number of the character to give the flask to: ")
		}
		n, convErr := strconv.Atoi(readLine(reader))
		if convErr != nil {
			fmt.Fprintln(stderr, "error: expected a slot number from the table")
			return 1
		}
		for _, c := range chars {
			if c.Slot == n {
				target = n
			}
		}
		if target < 0 {
			fmt.Fprintf(stderr, "error: slot %d is not an active character\n", n)
			return 1
		}
	}

	var chosen physickCharacter
	for _, c := range chars {
		if c.Slot == target {
			chosen = c
		}
	}
	fmt.Fprintf(stdout, "\nTarget: slot %d %q\n", chosen.Slot, chosen.Name)

	// Decide what to do. -fix-duplicates alone = dedupe only. Otherwise the
	// flask/flag path runs, and when the character also has duplicated goods
	// rows the interactive prompt offers to remove them in the same pass; a
	// scripted run (-character/-slot) only gets a hint, so its behaviour stays
	// predictable.
	doFlask, doDedupe := !*fixDup, *fixDup
	if !*fixDup && chosen.DupGoods > 0 {
		if prompted {
			fmt.Fprintf(stdout, "\nThis character has %d duplicated goods row(s). The Rust ER Save Editor\n"+
				"refuses such saves (\"Save file has irregular data!\"); the game does not mind.\n"+
				"Remove the second copies as well? [Y/n]: ", chosen.DupGoods)
			ans := strings.ToLower(readLine(reader))
			doDedupe = ans == "" || ans == "y" || ans == "yes"
		} else {
			fmt.Fprintf(stdout, "  note: %d duplicated goods row(s); rerun with -fix-duplicates to remove them\n", chosen.DupGoods)
		}
	}

	changed := false
	if doDedupe {
		lines, removed, err := app.dedupeGoodsRows(target)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		for _, l := range lines {
			fmt.Fprintf(stdout, "  %s\n", l)
		}
		if removed == 0 {
			fmt.Fprintln(stdout, "  duplicates: none found")
		} else {
			fmt.Fprintf(stdout, "  duplicates: removed %d row(s)\n", removed)
			changed = true
		}
	}
	if doFlask {
		if chosen.Duplicates > 1 {
			fmt.Fprintf(stderr, "error: this character already has %d Flask of Wondrous Physick records; repair it in SaveForge (Inventory Integrity) first\n", chosen.Duplicates)
			return 1
		}
		if chosen.HasFlask {
			fmt.Fprintln(stdout, "  flask:  already present, nothing to add")
		} else {
			res, err := app.AddItemsToCharacter(target, []uint32{db.ItemFlaskWondrousPhysickEmpty}, 0, 0, 0, 0, 1, 0)
			if err != nil {
				fmt.Fprintf(stderr, "error: add flask: %v\n", err)
				return 1
			}
			if res.Added != 1 {
				fmt.Fprintf(stderr, "error: flask was not added (added=%d, capHit=%q)\n", res.Added, res.CapHit)
				return 1
			}
			fmt.Fprintln(stdout, "  flask:  added (empty; it fills at the next site of grace)")
			changed = true
		}
		if chosen.FlagSet {
			fmt.Fprintf(stdout, "  flag %d: already set\n", PhysickObtainedFlag)
		} else {
			if err := app.setPhysickFlag(target); err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1
			}
			fmt.Fprintf(stdout, "  flag %d: set\n", PhysickObtainedFlag)
			changed = true
		}
	}
	if !changed {
		fmt.Fprintln(stdout, "\nNothing to do; no file written.")
		return 0
	}

	// Resolve destination and write.
	dest := *out
	if *inPlace {
		dest = path
		backup, err := core.CreateBackup(path)
		if err != nil {
			fmt.Fprintf(stderr, "error: backup failed, nothing written: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "\nBackup: %s\n", backup)
	} else if dest == "" {
		ext := filepath.Ext(path)
		suffix := ".physick"
		if !doFlask {
			suffix = ".dedupe"
		}
		dest = strings.TrimSuffix(path, ext) + suffix + ext
	}
	if err := app.writeSaveTo(dest); err != nil {
		fmt.Fprintf(stderr, "error: write %s: %v\n", dest, err)
		return 1
	}
	fmt.Fprintf(stdout, "Wrote:  %s\n\n", dest)

	// Verify by reloading the written file from disk.
	check := NewApp()
	if _, err := check.LoadSaveFromPath(dest); err != nil {
		fmt.Fprintf(stderr, "error: written file does not reload: %v\n", err)
		return 1
	}
	after := check.physickCharacters()
	printPhysickTable(stdout, after)
	for _, c := range after {
		if c.Slot != target {
			continue
		}
		ok := true
		if doFlask {
			ok = ok && c.HasFlask && c.FlagSet && c.Duplicates <= 1
		}
		if doDedupe {
			ok = ok && c.DupGoods == 0
		}
		if ok {
			fmt.Fprintln(stdout, "\nVerified. Quit Elden Ring, then copy the written file over the game's save file.")
			return 0
		}
		fmt.Fprintf(stderr, "error: verification failed for slot %d (flask=%v flag=%v duplicates=%d dupGoods=%d)\n", c.Slot, c.HasFlask, c.FlagSet, c.Duplicates, c.DupGoods)
		return 1
	}
	fmt.Fprintf(stderr, "error: slot %d missing after reload\n", target)
	return 1
}

// physickCharacters reports every active slot's flask + flag state.
func (a *App) physickCharacters() []physickCharacter {
	names := a.GetCharacterNames()
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return nil
	}
	var out []physickCharacter
	for i := 0; i < 10; i++ {
		if !a.save.ActiveSlots[i] {
			continue
		}
		slot := &a.save.Slots[i]
		c := physickCharacter{Slot: i, Name: names[i], Level: slot.Player.Level}
		for _, item := range slot.Inventory.CommonItems {
			if item.GaItemHandle == 0 || item.GaItemHandle == 0xFFFFFFFF {
				continue
			}
			id, ok := slot.GaMap[item.GaItemHandle]
			if !ok {
				id = db.HandleToItemID(item.GaItemHandle)
			}
			if db.IsWondrousPhysick(id) {
				c.HasFlask = true
				c.Duplicates++
			}
		}
		c.DupGoods = countDupGoodsRows(slot.Inventory.CommonItems) + countDupGoodsRows(slot.Storage.CommonItems)
		if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
			c.FlagSet, _ = db.GetEventFlag(slot.Data[slot.EventFlagsOffset:], PhysickObtainedFlag)
		}
		out = append(out, c)
	}
	return out
}

// countDupGoodsRows mirrors the Rust ER-Save-Editor's check_for_duplicate_items:
// number of rows whose 0xB0 handle already occurred earlier in the list.
func countDupGoodsRows(items []core.InventoryItem) int {
	seen := make(map[uint32]bool, len(items))
	n := 0
	for _, it := range items {
		if it.GaItemHandle>>24 != 0xB0 {
			continue
		}
		if seen[it.GaItemHandle] {
			n++
		}
		seen[it.GaItemHandle] = true
	}
	return n
}

// dedupeGoodsRows clears every second-or-later row of a duplicated goods
// handle in the slot's common inventory and storage, keeping the first row
// untouched, and decrements each container's count header. The row is cleared
// exactly the way core.RepairDuplicateWondrousPhysick clears one (handle 0,
// qty 0, index = row). GaMap/GaItems are untouched because the handle is still
// referenced by the kept row.
func (a *App) dedupeGoodsRows(idx int) (lines []string, removed int, err error) {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return nil, 0, fmt.Errorf("no save loaded")
	}
	a.slotMu[idx].Lock()
	defer a.slotMu[idx].Unlock()
	slot := &a.save.Slots[idx]
	sa := core.NewSlotAccessor(slot.Data)

	clear := func(label string, items []core.InventoryItem, start int) error {
		seen := map[uint32]int{}
		n := 0
		for row := range items {
			it := items[row]
			if it.GaItemHandle>>24 != 0xB0 {
				continue
			}
			first, dup := seen[it.GaItemHandle]
			if !dup {
				seen[it.GaItemHandle] = row
				continue
			}
			off := start + row*core.InvRecordLen
			if err := sa.CheckBounds(off, core.InvRecordLen, "dedupe/"+label); err != nil {
				return err
			}
			id := it.GaItemHandle
			if m, ok := slot.GaMap[it.GaItemHandle]; ok {
				id = m
			} else {
				id = db.HandleToItemID(it.GaItemHandle)
			}
			d, _ := db.GetItemDataFuzzy(id)
			name := d.Name
			if name == "" {
				name = "unknown item (mod?)"
			}
			lines = append(lines, fmt.Sprintf("%s row %d: %s (0x%08X) qty %d duplicates row %d qty %d -> removed",
				label, row, name, it.GaItemHandle, it.Quantity&0x7FFFFFFF, first, items[first].Quantity&0x7FFFFFFF))
			items[row] = core.InventoryItem{GaItemHandle: 0, Quantity: 0, Index: uint32(row)}
			binary.LittleEndian.PutUint32(slot.Data[off:], 0)
			binary.LittleEndian.PutUint32(slot.Data[off+4:], 0)
			binary.LittleEndian.PutUint32(slot.Data[off+8:], uint32(row))
			n++
		}
		if n > 0 {
			countOff := start - 4
			if err := sa.CheckBounds(countOff, 4, "dedupe/"+label+"/count"); err != nil {
				return err
			}
			cur := binary.LittleEndian.Uint32(slot.Data[countOff:])
			if cur >= uint32(n) {
				binary.LittleEndian.PutUint32(slot.Data[countOff:], cur-uint32(n))
			}
			removed += n
		}
		return nil
	}
	if err := clear("inventory", slot.Inventory.CommonItems, slot.MagicOffset+core.InvStartFromMagic); err != nil {
		return lines, removed, err
	}
	if slot.StorageBoxOffset > 0 {
		if err := clear("storage", slot.Storage.CommonItems, slot.StorageBoxOffset+core.StorageHeaderSkip); err != nil {
			return lines, removed, err
		}
	}
	return lines, removed, nil
}

// setPhysickFlag sets PhysickObtainedFlag on one slot, using the same
// primitive as the World tab's map-flag toggles.
func (a *App) setPhysickFlag(idx int) error {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	a.slotMu[idx].Lock()
	defer a.slotMu[idx].Unlock()
	slot := &a.save.Slots[idx]
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return fmt.Errorf("event flags offset not computed for slot %d", idx)
	}
	if err := db.SetEventFlag(slot.Data[slot.EventFlagsOffset:], PhysickObtainedFlag, true); err != nil {
		return fmt.Errorf("set flag %d: %w", PhysickObtainedFlag, err)
	}
	return nil
}

// writeSaveTo serialises the loaded save to dest without the Wails dialog.
func (a *App) writeSaveTo(dest string) error {
	a.saveMu.RLock()
	expected := a.save
	a.saveMu.RUnlock()
	if expected == nil {
		return fmt.Errorf("no save loaded")
	}
	_, err := a.writeSaveCore(dest, expected)
	return err
}

func printPhysickTable(w io.Writer, chars []physickCharacter) {
	fmt.Fprintf(w, "  %-4s %-18s %-6s %-8s %-11s %s\n", "Slot", "Name", "Level", "Flask", fmt.Sprintf("Flag %d", PhysickObtainedFlag), "Dup goods")
	for _, c := range chars {
		flask := "no"
		if c.HasFlask {
			flask = "yes"
			if c.Duplicates > 1 {
				flask = fmt.Sprintf("x%d!", c.Duplicates)
			}
		}
		fl := "clear"
		if c.FlagSet {
			fl = "set"
		}
		dup := "0"
		if c.DupGoods > 0 {
			dup = fmt.Sprintf("%d!", c.DupGoods)
		}
		fmt.Fprintf(w, "  %-4d %-18s %-6d %-8s %-11s %s\n", c.Slot, c.Name, c.Level, flask, fl, dup)
	}
}

// readLine reads one prompt answer, tolerating a UTF-8 BOM (Windows
// PowerShell prepends one when piping text to a native process), CRLF and
// surrounding quotes from drag-and-drop paths.
func readLine(r *bufio.Reader) string {
	line, _ := r.ReadString('\n')
	line = strings.TrimPrefix(line, string([]byte{0xEF, 0xBB, 0xBF}))
	return strings.Trim(strings.TrimSpace(line), `"`)
}

func pauseForEnter(r *bufio.Reader, w io.Writer) {
	fmt.Fprint(w, "\nPress Enter to exit.")
	_, _ = r.ReadString('\n')
}
