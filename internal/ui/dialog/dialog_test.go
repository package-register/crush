package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

// fakeDialog implements Dialog for testing Overlay.
type fakeDialog struct {
	id string
}

func (f fakeDialog) ID() string                               { return f.id }
func (f fakeDialog) HandleMsg(tea.Msg) Action                 { return nil }
func (f fakeDialog) Draw(uv.Screen, uv.Rectangle) *tea.Cursor { return nil }

func TestOverlay_HasDialogs(t *testing.T) {
	o := NewOverlay()
	if o.HasDialogs() {
		t.Error("new overlay should have no dialogs")
	}
	o.OpenDialog(fakeDialog{id: "test"})
	if !o.HasDialogs() {
		t.Error("overlay should have dialogs after OpenDialog")
	}
	o.CloseDialog("test")
	if o.HasDialogs() {
		t.Error("overlay should have no dialogs after CloseDialog")
	}
}

func TestOverlay_ContainsDialog(t *testing.T) {
	o := NewOverlay()
	if o.ContainsDialog("a") {
		t.Error("empty overlay should not contain any dialog")
	}
	o.OpenDialog(fakeDialog{id: "a"})
	o.OpenDialog(fakeDialog{id: "b"})
	if !o.ContainsDialog("a") {
		t.Error("overlay should contain dialog a")
	}
	if !o.ContainsDialog("b") {
		t.Error("overlay should contain dialog b")
	}
	if o.ContainsDialog("c") {
		t.Error("overlay should not contain dialog c")
	}
	o.CloseDialog("a")
	if o.ContainsDialog("a") {
		t.Error("overlay should not contain a after CloseDialog")
	}
}

func TestOverlay_Dialog(t *testing.T) {
	o := NewOverlay()
	d1 := fakeDialog{id: "first"}
	o.OpenDialog(d1)
	if got := o.Dialog("first"); got != d1 {
		t.Errorf("Dialog(\"first\") = %v, want %v", got, d1)
	}
	if o.Dialog("missing") != nil {
		t.Error("Dialog(\"missing\") should return nil")
	}
}

func TestOverlay_DialogLast(t *testing.T) {
	o := NewOverlay()
	if o.DialogLast() != nil {
		t.Error("DialogLast on empty overlay should return nil")
	}
	d1 := fakeDialog{id: "first"}
	d2 := fakeDialog{id: "second"}
	o.OpenDialog(d1)
	o.OpenDialog(d2)
	if got := o.DialogLast(); got != d2 {
		t.Errorf("DialogLast = %v, want second dialog", got)
	}
	o.CloseDialog("second")
	if got := o.DialogLast(); got != d1 {
		t.Errorf("after close second, DialogLast = %v, want first", got)
	}
}

func TestOverlay_CloseFrontDialog(t *testing.T) {
	o := NewOverlay(fakeDialog{id: "a"})
	o.CloseFrontDialog()
	if o.HasDialogs() {
		t.Error("CloseFrontDialog should remove the only dialog")
	}
	o.OpenDialog(fakeDialog{id: "a"})
	o.OpenDialog(fakeDialog{id: "b"})
	o.CloseFrontDialog()
	if !o.ContainsDialog("a") || o.ContainsDialog("b") {
		t.Error("CloseFrontDialog should remove front (b), not a")
	}
}

func TestOverlay_BringToFront(t *testing.T) {
	o := NewOverlay()
	d1 := fakeDialog{id: "a"}
	d2 := fakeDialog{id: "b"}
	o.OpenDialog(d1)
	o.OpenDialog(d2)
	o.BringToFront("a")
	if o.DialogLast() != d1 {
		t.Error("BringToFront(\"a\") should move a to front")
	}
}

func TestDialogConstants(t *testing.T) {
	if WebDAVConfigID != "webdav_config" {
		t.Errorf("WebDAVConfigID = %q, want webdav_config", WebDAVConfigID)
	}
	if AguiConfigID != "agui_config" {
		t.Errorf("AguiConfigID = %q, want agui_config", AguiConfigID)
	}
}
