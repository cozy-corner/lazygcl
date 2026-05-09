package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cozy-corner/lazygcl/internal/gcp"
)

// TestHandleKey_OpensPickers verifies that Ctrl+R / Ctrl+L from the main
// view switch currentView to viewPicker with the right kind. The Cmd that
// fetches items is intentionally not invoked here — that path requires a
// live gcp.Client.
func TestHandleKey_OpensPickers(t *testing.T) {
	cases := []struct {
		name string
		key  tea.KeyType
		want pickerKind
	}{
		{"resource", tea.KeyCtrlR, pickerResource},
		{"logName", tea.KeyCtrlL, pickerLogName},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := Model{currentView: viewMain, focus: paneQuery}
			out, _ := m.handleKey(tea.KeyMsg{Type: c.key})
			got, ok := out.(Model)
			if !ok {
				t.Fatalf("handleKey returned %T, want Model", out)
			}
			if got.currentView != viewPicker {
				t.Errorf("currentView = %v, want viewPicker", got.currentView)
			}
			if got.pickerKind != c.want {
				t.Errorf("pickerKind = %v, want %v", got.pickerKind, c.want)
			}
			if !got.pickerLoading {
				t.Error("pickerLoading = false, want true (fetch in flight)")
			}
		})
	}
}

func TestApplyPickerSelection_AppendsClause(t *testing.T) {
	ta := textarea.New()
	ti := textinput.New()
	m := Model{
		currentView:  viewPicker,
		pickerKind:   pickerResource,
		pickerItems:  []pickerItem{{Display: "gce_instance", FilterKey: "gce_instance", Value: "gce_instance"}},
		pickerCursor: 0,
		query:        ta,
		pickerInput:  ti,
	}

	out, _ := m.applyPickerSelection()
	if got := out.query.Value(); got != `resource.type = "gce_instance"` {
		t.Errorf("query = %q, want %q", got, `resource.type = "gce_instance"`)
	}
	if out.currentView != viewMain {
		t.Errorf("currentView = %v, want viewMain", out.currentView)
	}

	// Append onto an existing query.
	m.query.SetValue(`severity >= "ERROR"`)
	out, _ = m.applyPickerSelection()
	want := "severity >= \"ERROR\"\nAND resource.type = \"gce_instance\""
	if got := out.query.Value(); got != want {
		t.Errorf("query = %q, want %q", got, want)
	}
}

func TestShortLogName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"projects/p/logs/stdout", "stdout"},
		{"projects/p/logs/cloudaudit.googleapis.com%2Factivity", "cloudaudit.googleapis.com/activity"},
		{"weird-no-prefix", "weird-no-prefix"},
	}
	for _, c := range cases {
		if got := shortLogName(c.in); got != c.want {
			t.Errorf("shortLogName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFilteredPickerItems_Fuzzy(t *testing.T) {
	m := Model{
		pickerItems: []pickerItem{
			{FilterKey: "gce_instance vm instance"},
			{FilterKey: "k8s_container"},
			{FilterKey: "cloud_run_revision"},
			{FilterKey: "aws_ec2_instance"},
		},
		pickerInput: textinput.New(),
	}

	// "gci" should fuzzy-match gce_instance (g_c_i in order).
	m.pickerInput.SetValue("gci")
	idx := m.filteredPickerItems()
	if len(idx) == 0 || idx[0] != 0 {
		t.Errorf("query 'gci' top match index = %v, want 0 (gce_instance)", idx)
	}

	// "run" should match cloud_run_revision.
	m.pickerInput.SetValue("run")
	idx = m.filteredPickerItems()
	if len(idx) == 0 || idx[0] != 2 {
		t.Errorf("query 'run' top match index = %v, want 2 (cloud_run_revision)", idx)
	}

	// Empty query returns all items in original order.
	m.pickerInput.SetValue("")
	idx = m.filteredPickerItems()
	if len(idx) != 4 || idx[0] != 0 || idx[3] != 3 {
		t.Errorf("empty query = %v, want [0 1 2 3]", idx)
	}

	// Nonsense query returns no matches.
	m.pickerInput.SetValue("zzzz")
	if got := m.filteredPickerItems(); len(got) != 0 {
		t.Errorf("nonsense query = %v, want empty", got)
	}
}

func TestHandlePickerKey_TextinputEditing(t *testing.T) {
	cases := []struct {
		name string
		key  tea.KeyType
		in   string
		want string
	}{
		{"ctrl+w deletes last word", tea.KeyCtrlW, "k8s_container foo", "k8s_container "},
		{"ctrl+u clears from cursor to start", tea.KeyCtrlU, "k8s_container", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ti := textinput.New()
			ti.Focus()
			ti.SetValue(c.in)
			ti.SetCursor(len(c.in))
			m := Model{currentView: viewPicker, pickerKind: pickerResource, pickerInput: ti}

			out, _ := m.handlePickerKey(tea.KeyMsg{Type: c.key})
			got, ok := out.(Model)
			if !ok {
				t.Fatalf("handlePickerKey returned %T, want Model", out)
			}
			if g := got.pickerInput.Value(); g != c.want {
				t.Errorf("input = %q, want %q", g, c.want)
			}
		})
	}
}

func TestResourceItems(t *testing.T) {
	in := []gcp.ResourceDescriptor{
		{Type: "gce_instance", DisplayName: "VM Instance"},
		{Type: "k8s_container"},
	}
	got := resourceItems(in)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Value != "gce_instance" || got[1].Value != "k8s_container" {
		t.Errorf("values = %q,%q", got[0].Value, got[1].Value)
	}
}
