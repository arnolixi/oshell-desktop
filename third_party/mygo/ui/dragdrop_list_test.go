package ui

import (
	"fmt"
	"testing"
)

func TestTableDragHandlePreservesRowInput(t *testing.T) {
	selected, submits, edits, dropped := -1, 0, 0, -1
	var chosen Selection[int]
	s := ListState{Selected: &selected, Selection: &chosen}
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Fill().Children(func() {
			table := coreTable(c, &s, []TableColumn{{Title: "Name", Width: 150}, {Title: "Detail", Width: 100}, {Title: "Edit", Width: 100}}, 4, func(row, col int) {
				switch col {
				case 0:
					coreBox(c).FillWidth().Height(32).Label(fmt.Sprintf("handle %d", row)).Drag(row)
				case 1:
					coreTextf(c, "detail %d", row)
				case 2:
					if coreButton(c, fmt.Sprintf("edit %d", row)).Drag(row).Clicked() {
						edits++
					}
				}
			}).Grow(1)
			if table.Submitted() {
				submits++
			}
			if value, ok := coreDrop[int](coreBox(c).Height(40).FillWidth().Label("target")); ok {
				dropped = value
			}
		})
	}, 400, 360)
	click := func(mods Modifiers, label string) {
		t.Helper()
		if err := tt.ClickWith(mods, label); err != nil {
			t.Fatal(err)
		}
	}
	click(0, "handle 0")
	if selected != 0 || !chosen.Has(0) {
		t.Fatal("drag handle did not select row")
	}
	click(Cmd, "handle 2")
	if chosen.Len() != 2 || !chosen.Has(0) || !chosen.Has(2) {
		t.Fatal("modified handle click lost multi-selection")
	}
	click(Shift, "detail 3")
	if chosen.Len() != 2 || !chosen.Has(2) || !chosen.Has(3) {
		t.Fatal("handle did not set selection anchor")
	}
	click(0, "detail 0")
	click(Shift, "handle 2")
	if chosen.Len() != 3 {
		t.Fatal("handle did not extend selection from another cell")
	}
	click(0, "handle 1")
	click(0, "handle 1")
	if submits != 1 {
		t.Fatalf("double click submissions = %d", submits)
	}
	tt.Key(0, KeyEnter)
	if submits != 2 {
		t.Fatal("drag handle did not focus table")
	}
	click(0, "edit 3")
	if edits != 1 || selected != 1 || submits != 2 {
		t.Fatal("button drag handle changed row action")
	}
	src, _ := tt.Find("handle 0")
	target, _ := tt.Find("target")
	drag(tt, src.X+10, src.Y+16, target.X+10, target.Y+20)
	if dropped != 0 || selected != 1 || submits != 2 {
		t.Fatalf("drag changed row input: dropped=%d selected=%d submits=%d", dropped, selected, submits)
	}
}
