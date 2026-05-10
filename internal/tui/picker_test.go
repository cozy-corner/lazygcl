package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cozy-corner/lazygcl/internal/gcp"
)

// TestHandleKey_OpensFieldPicker verifies Ctrl+F from the main view opens
// the in-memory field picker.
func TestHandleKey_OpensFieldPicker(t *testing.T) {
	m := Model{currentView: viewMain, focus: paneQuery}
	out, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlF})
	got, ok := out.(Model)
	if !ok {
		t.Fatalf("handleKey returned %T, want Model", out)
	}
	if got.currentView != viewPicker {
		t.Errorf("currentView = %v, want viewPicker", got.currentView)
	}
	if got.pickerKind != pickerField {
		t.Errorf("pickerKind = %v, want pickerField", got.pickerKind)
	}
	if got.pickerLoading {
		t.Error("pickerLoading = true, want false (field picker is in-memory)")
	}
	if len(got.pickerItems) == 0 {
		t.Error("pickerItems is empty, want top-level fields")
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

func TestApplyPickerSelection_ResourceTypeClosesPicker(t *testing.T) {
	ta := textarea.New()
	ti := textinput.New()
	m := Model{
		currentView:  viewPicker,
		pickerKind:   pickerResource,
		pickerItems:  []pickerItem{{Display: "cloud_run_revision", FilterKey: "cloud_run_revision", Value: "cloud_run_revision"}},
		pickerCursor: 0,
		query:        ta,
		pickerInput:  ti,
	}
	out, _ := m.applyPickerSelection()
	if out.currentView != viewMain {
		t.Errorf("currentView = %v, want viewMain", out.currentView)
	}
	if got := out.query.Value(); got != `resource.type = "cloud_run_revision"` {
		t.Errorf("query = %q, want resource.type clause", got)
	}
}

func TestLabelKeyItems(t *testing.T) {
	in := []gcp.LabelDescriptor{
		{Key: "service_name", Description: "The name of the service"},
		{Key: "location"},
	}
	got := labelKeyItems(in)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Value != "service_name" || got[1].Value != "location" {
		t.Errorf("values = %q,%q", got[0].Value, got[1].Value)
	}
}

// fieldPickerModelWithSelection builds a model whose pickerField items
// contain a single item for `path` so applyPickerSelection picks it.
func fieldPickerModelWithSelection(path string) Model {
	ta := textarea.New()
	ta.SetWidth(200)
	ti := textinput.New()
	return Model{
		currentView:  viewPicker,
		pickerKind:   pickerField,
		pickerItems:  []pickerItem{{Display: path, FilterKey: path, Value: path}},
		pickerCursor: 0,
		query:        ta,
		pickerInput:  ti,
	}
}

func TestApplyFieldSelection_SkeletonInsertsClauseWithCursor(t *testing.T) {
	m := fieldPickerModelWithSelection("trace")
	out, _ := m.applyPickerSelection()
	if out.currentView != viewMain {
		t.Errorf("currentView = %v, want viewMain", out.currentView)
	}
	want := `trace = ""`
	if got := out.query.Value(); got != want {
		t.Errorf("query = %q, want %q", got, want)
	}
	out.query.InsertRune('X')
	if got := out.query.Value(); got != `trace = "X"` {
		t.Errorf("after InsertRune, query = %q, want trace = \"X\" (cursor not between quotes)", got)
	}
}

func TestApplyFieldSelection_TimestampUsesGreaterEqual(t *testing.T) {
	m := fieldPickerModelWithSelection("timestamp")
	out, _ := m.applyPickerSelection()
	want := `timestamp >= ""`
	if got := out.query.Value(); got != want {
		t.Errorf("query = %q, want %q", got, want)
	}
}

func TestApplyFieldSelection_TextPayloadUsesRegexOp(t *testing.T) {
	m := fieldPickerModelWithSelection("textPayload")
	out, _ := m.applyPickerSelection()
	want := `textPayload =~ ""`
	if got := out.query.Value(); got != want {
		t.Errorf("query = %q, want %q", got, want)
	}
}

func TestApplyFieldSelection_SeverityChainsIntoEnumPicker(t *testing.T) {
	m := fieldPickerModelWithSelection("severity")
	out, _ := m.applyPickerSelection()
	if out.currentView != viewPicker {
		t.Errorf("currentView = %v, want viewPicker (enum chain)", out.currentView)
	}
	if out.pickerKind != pickerEnumValue {
		t.Errorf("pickerKind = %v, want pickerEnumValue", out.pickerKind)
	}
	if len(out.pickerItems) != len(severityLevels) {
		t.Errorf("enum item count = %d, want %d", len(out.pickerItems), len(severityLevels))
	}
	if out.pickerEnumField != "severity" || out.pickerEnumOp != ">=" || !out.pickerEnumQuoted {
		t.Errorf("enum ctx = (%q, %q, quoted=%v), want (severity, >=, true)",
			out.pickerEnumField, out.pickerEnumOp, out.pickerEnumQuoted)
	}
	if out.query.Value() != "" {
		t.Errorf("query = %q, want empty (no clause inserted until value chosen)", out.query.Value())
	}
}

func TestApplyFieldSelection_TraceSampledChainsIntoEnumPickerUnquoted(t *testing.T) {
	m := fieldPickerModelWithSelection("traceSampled")
	out, _ := m.applyPickerSelection()
	if out.pickerKind != pickerEnumValue {
		t.Errorf("pickerKind = %v, want pickerEnumValue", out.pickerKind)
	}
	if out.pickerEnumQuoted {
		t.Error("pickerEnumQuoted = true, want false for traceSampled (bool)")
	}
}

func TestApplyEnumValueSelection_SeverityInsertsClause(t *testing.T) {
	ta := textarea.New()
	ta.SetWidth(200)
	ti := textinput.New()
	m := Model{
		currentView:      viewPicker,
		pickerKind:       pickerEnumValue,
		pickerItems:      []pickerItem{{Display: "ERROR", FilterKey: "error", Value: "ERROR"}},
		pickerCursor:     0,
		query:            ta,
		pickerInput:      ti,
		pickerEnumField:  "severity",
		pickerEnumOp:     ">=",
		pickerEnumQuoted: true,
	}
	out, _ := m.applyPickerSelection()
	want := `severity >= "ERROR"`
	if got := out.query.Value(); got != want {
		t.Errorf("query = %q, want %q", got, want)
	}
	if out.currentView != viewMain {
		t.Errorf("currentView = %v, want viewMain", out.currentView)
	}
}

func TestApplyEnumValueSelection_TraceSampledUnquoted(t *testing.T) {
	ta := textarea.New()
	ta.SetWidth(200)
	ti := textinput.New()
	m := Model{
		currentView:      viewPicker,
		pickerKind:       pickerEnumValue,
		pickerItems:      []pickerItem{{Display: "true", FilterKey: "true", Value: "true"}},
		pickerCursor:     0,
		query:            ta,
		pickerInput:      ti,
		pickerEnumField:  "traceSampled",
		pickerEnumOp:     "=",
		pickerEnumQuoted: false,
	}
	out, _ := m.applyPickerSelection()
	want := `traceSampled = true`
	if got := out.query.Value(); got != want {
		t.Errorf("query = %q, want %q", got, want)
	}
}

func TestApplyFieldSelection_LogNameDispatchesToDynamicPicker(t *testing.T) {
	m := fieldPickerModelWithSelection("logName")
	out, cmd := m.applyPickerSelection()
	if out.pickerKind != pickerLogName {
		t.Errorf("pickerKind = %v, want pickerLogName", out.pickerKind)
	}
	if !out.pickerLoading {
		t.Error("pickerLoading = false, want true (API fetch in flight)")
	}
	if cmd == nil {
		t.Error("cmd = nil, want non-nil fetch Cmd")
	}
}

func TestApplyFieldSelection_ResourceOpensSubFieldPicker(t *testing.T) {
	m := fieldPickerModelWithSelection("resource")
	out, cmd := m.applyPickerSelection()
	if out.currentView != viewPicker {
		t.Errorf("currentView = %v, want viewPicker (sub-field picker)", out.currentView)
	}
	if out.pickerKind != pickerObjectSubField {
		t.Errorf("pickerKind = %v, want pickerObjectSubField", out.pickerKind)
	}
	if out.pickerObjectParent != "resource" {
		t.Errorf("pickerObjectParent = %q, want %q", out.pickerObjectParent, "resource")
	}
	if cmd != nil {
		t.Error("cmd = non-nil, want nil (sub-field picker is in-memory)")
	}
	if len(out.pickerItems) != 2 {
		t.Errorf("sub-field items len = %d, want 2 (type + labels)", len(out.pickerItems))
	}
}

func TestApplyResourceSubFieldSelection_TypeDispatchesToResourcePicker(t *testing.T) {
	ta := textarea.New()
	ti := textinput.New()
	m := Model{
		currentView:        viewPicker,
		pickerKind:         pickerObjectSubField,
		pickerObjectParent: "resource",
		pickerItems:        []pickerItem{{Display: "type", FilterKey: "type", Value: "type"}},
		pickerCursor:       0,
		query:              ta,
		pickerInput:        ti,
	}
	out, cmd := m.applyPickerSelection()
	if out.pickerKind != pickerResource {
		t.Errorf("pickerKind = %v, want pickerResource", out.pickerKind)
	}
	if !out.pickerLoading {
		t.Error("pickerLoading = false, want true (API fetch in flight)")
	}
	if cmd == nil {
		t.Error("cmd = nil, want non-nil fetch Cmd")
	}
}

func TestApplyResourceSubFieldSelection_LabelsDispatchesToLabelsAllPicker(t *testing.T) {
	ta := textarea.New()
	ti := textinput.New()
	m := Model{
		currentView:        viewPicker,
		pickerKind:         pickerObjectSubField,
		pickerObjectParent: "resource",
		pickerItems:        []pickerItem{{Display: "labels", FilterKey: "labels", Value: "labels"}},
		pickerCursor:       0,
		query:              ta,
		pickerInput:        ti,
	}
	out, cmd := m.applyPickerSelection()
	if out.pickerKind != pickerResourceLabelsAll {
		t.Errorf("pickerKind = %v, want pickerResourceLabelsAll", out.pickerKind)
	}
	if !out.pickerLoading {
		t.Error("pickerLoading = false, want true")
	}
	if cmd == nil {
		t.Error("cmd = nil, want non-nil fetch Cmd")
	}
}

func TestApplyFieldSelection_HttpRequestOpensSubFieldPicker(t *testing.T) {
	m := fieldPickerModelWithSelection("httpRequest")
	out, cmd := m.applyPickerSelection()
	if out.currentView != viewPicker {
		t.Errorf("currentView = %v, want viewPicker", out.currentView)
	}
	if out.pickerKind != pickerObjectSubField {
		t.Errorf("pickerKind = %v, want pickerObjectSubField", out.pickerKind)
	}
	if out.pickerObjectParent != "httpRequest" {
		t.Errorf("pickerObjectParent = %q, want %q", out.pickerObjectParent, "httpRequest")
	}
	if cmd != nil {
		t.Error("cmd = non-nil, want nil (sub-field picker is in-memory)")
	}
	if got, want := len(out.pickerItems), len(objectSubFields["httpRequest"]); got != want {
		t.Errorf("sub-field items len = %d, want %d", got, want)
	}
}

func TestApplyHttpRequestSubFieldSelection_StatusInsertsSkeletonUnquoted(t *testing.T) {
	ta := textarea.New()
	ta.SetWidth(200)
	ti := textinput.New()
	m := Model{
		currentView:        viewPicker,
		pickerKind:         pickerObjectSubField,
		pickerObjectParent: "httpRequest",
		pickerItems:        []pickerItem{{Display: "status", FilterKey: "status", Value: "status"}},
		pickerCursor:       0,
		query:              ta,
		pickerInput:        ti,
	}
	out, _ := m.applyPickerSelection()
	if out.currentView != viewMain {
		t.Errorf("currentView = %v, want viewMain", out.currentView)
	}
	want := `httpRequest.status >= `
	if got := out.query.Value(); got != want {
		t.Errorf("query = %q, want %q", got, want)
	}
	out.query.InsertRune('5')
	wantAfter := `httpRequest.status >= 5`
	if got := out.query.Value(); got != wantAfter {
		t.Errorf("after InsertRune, query = %q, want %q (cursor at end-of-line for unquoted)", got, wantAfter)
	}
}

func TestApplyHttpRequestSubFieldSelection_RequestUrlInsertsSkeletonQuoted(t *testing.T) {
	ta := textarea.New()
	ta.SetWidth(200)
	ti := textinput.New()
	m := Model{
		currentView:        viewPicker,
		pickerKind:         pickerObjectSubField,
		pickerObjectParent: "httpRequest",
		pickerItems:        []pickerItem{{Display: "requestUrl", FilterKey: "requesturl", Value: "requestUrl"}},
		pickerCursor:       0,
		query:              ta,
		pickerInput:        ti,
	}
	out, _ := m.applyPickerSelection()
	want := `httpRequest.requestUrl =~ ""`
	if got := out.query.Value(); got != want {
		t.Errorf("query = %q, want %q", got, want)
	}
	out.query.InsertRune('X')
	wantAfter := `httpRequest.requestUrl =~ "X"`
	if got := out.query.Value(); got != wantAfter {
		t.Errorf("after InsertRune, query = %q, want %q (cursor between quotes)", got, wantAfter)
	}
}

func TestApplyHttpRequestSubFieldSelection_RequestMethodChainsToEnumPicker(t *testing.T) {
	ta := textarea.New()
	ti := textinput.New()
	m := Model{
		currentView:        viewPicker,
		pickerKind:         pickerObjectSubField,
		pickerObjectParent: "httpRequest",
		pickerItems:        []pickerItem{{Display: "requestMethod", FilterKey: "requestmethod", Value: "requestMethod"}},
		pickerCursor:       0,
		query:              ta,
		pickerInput:        ti,
	}
	out, _ := m.applyPickerSelection()
	if out.currentView != viewPicker {
		t.Errorf("currentView = %v, want viewPicker (enum chain)", out.currentView)
	}
	if out.pickerKind != pickerEnumValue {
		t.Errorf("pickerKind = %v, want pickerEnumValue", out.pickerKind)
	}
	if out.pickerEnumField != "httpRequest.requestMethod" {
		t.Errorf("pickerEnumField = %q, want %q", out.pickerEnumField, "httpRequest.requestMethod")
	}
	if out.pickerEnumOp != "=" || !out.pickerEnumQuoted {
		t.Errorf("enum ctx = (op=%q, quoted=%v), want (=, true)", out.pickerEnumOp, out.pickerEnumQuoted)
	}
	if len(out.pickerItems) != len(httpRequestMethods) {
		t.Errorf("enum item count = %d, want %d", len(out.pickerItems), len(httpRequestMethods))
	}
}

func TestApplyHttpRequestSubFieldSelection_CacheHitChainsToEnumPickerUnquoted(t *testing.T) {
	ta := textarea.New()
	ti := textinput.New()
	m := Model{
		currentView:        viewPicker,
		pickerKind:         pickerObjectSubField,
		pickerObjectParent: "httpRequest",
		pickerItems:        []pickerItem{{Display: "cacheHit", FilterKey: "cachehit", Value: "cacheHit"}},
		pickerCursor:       0,
		query:              ta,
		pickerInput:        ti,
	}
	out, _ := m.applyPickerSelection()
	if out.pickerKind != pickerEnumValue {
		t.Errorf("pickerKind = %v, want pickerEnumValue", out.pickerKind)
	}
	if out.pickerEnumField != "httpRequest.cacheHit" {
		t.Errorf("pickerEnumField = %q, want %q", out.pickerEnumField, "httpRequest.cacheHit")
	}
	if out.pickerEnumQuoted {
		t.Error("pickerEnumQuoted = true, want false for bool sub-field")
	}
}

func TestApplyPickerSelection_LabelsAllInsertsClauseWithCursor(t *testing.T) {
	ta := textarea.New()
	ta.SetWidth(200)
	ti := textinput.New()
	m := Model{
		currentView:  viewPicker,
		pickerKind:   pickerResourceLabelsAll,
		pickerItems:  []pickerItem{{Display: "service_name", FilterKey: "service_name", Value: "service_name"}},
		pickerCursor: 0,
		query:        ta,
		pickerInput:  ti,
	}
	out, _ := m.applyPickerSelection()
	want := `resource.labels.service_name = ""`
	if got := out.query.Value(); got != want {
		t.Errorf("query = %q, want %q", got, want)
	}
	out.query.InsertRune('X')
	wantAfter := `resource.labels.service_name = "X"`
	if got := out.query.Value(); got != wantAfter {
		t.Errorf("after InsertRune, query = %q, want %q", got, wantAfter)
	}
}

func TestUnionLabels_Dedup(t *testing.T) {
	rs := []gcp.ResourceDescriptor{
		{Type: "cloud_run_revision", Labels: []gcp.LabelDescriptor{{Key: "service_name"}, {Key: "location"}}},
		{Type: "gce_instance", Labels: []gcp.LabelDescriptor{{Key: "instance_id"}, {Key: "location"}}},
	}
	got := unionLabels(rs)
	keys := make([]string, len(got))
	for i, l := range got {
		keys[i] = l.Key
	}
	want := []string{"instance_id", "location", "service_name"}
	if len(keys) != len(want) {
		t.Fatalf("union len = %d, want %d (deduped & sorted)", len(keys), len(want))
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Errorf("keys[%d] = %q, want %q (sorted)", i, keys[i], want[i])
		}
	}
}

func TestFieldItems_CoversAllCatalog(t *testing.T) {
	got := fieldItems()
	if len(got) != len(topLevelFields) {
		t.Errorf("fieldItems len = %d, want %d", len(got), len(topLevelFields))
	}
	for i, f := range topLevelFields {
		if got[i].Value != f.path {
			t.Errorf("fieldItems[%d].Value = %q, want %q", i, got[i].Value, f.path)
		}
	}
}

func TestOpenPicker_ResourceUsesCacheWhenAvailable(t *testing.T) {
	cached := []gcp.ResourceDescriptor{
		{Type: "cloud_run_revision", Labels: []gcp.LabelDescriptor{{Key: "service_name"}}},
	}
	m := Model{resourceDescriptors: cached}
	out, cmd := m.openPicker(pickerResource)
	if cmd != nil {
		t.Error("cmd = non-nil, want nil (cache hit should skip the fetch Cmd)")
	}
	if out.pickerLoading {
		t.Error("pickerLoading = true, want false (no fetch in flight)")
	}
	if len(out.pickerItems) != 1 || out.pickerItems[0].Value != "cloud_run_revision" {
		t.Errorf("pickerItems = %+v, want one item with Value=cloud_run_revision", out.pickerItems)
	}
}

func TestOpenPicker_LabelsAllUsesCacheWhenAvailable(t *testing.T) {
	cached := []gcp.ResourceDescriptor{
		{Type: "cloud_run_revision", Labels: []gcp.LabelDescriptor{{Key: "service_name"}, {Key: "location"}}},
		{Type: "gce_instance", Labels: []gcp.LabelDescriptor{{Key: "instance_id"}, {Key: "location"}}},
	}
	m := Model{resourceDescriptors: cached}
	out, cmd := m.openPicker(pickerResourceLabelsAll)
	if cmd != nil {
		t.Error("cmd = non-nil, want nil (cache hit should skip the fetch Cmd)")
	}
	if out.pickerLoading {
		t.Error("pickerLoading = true, want false")
	}
	if len(out.pickerItems) != 3 {
		t.Errorf("pickerItems len = %d, want 3 (deduped union of 4 keys → 3)", len(out.pickerItems))
	}
}

func TestApplyFieldSelection_OperationOpensSubFieldPicker(t *testing.T) {
	m := fieldPickerModelWithSelection("operation")
	out, cmd := m.applyPickerSelection()
	if out.currentView != viewPicker {
		t.Errorf("currentView = %v, want viewPicker", out.currentView)
	}
	if out.pickerKind != pickerObjectSubField {
		t.Errorf("pickerKind = %v, want pickerObjectSubField", out.pickerKind)
	}
	if out.pickerObjectParent != "operation" {
		t.Errorf("pickerObjectParent = %q, want %q", out.pickerObjectParent, "operation")
	}
	if cmd != nil {
		t.Error("cmd = non-nil, want nil (sub-field picker is in-memory)")
	}
	if got, want := len(out.pickerItems), len(objectSubFields["operation"]); got != want {
		t.Errorf("sub-field items len = %d, want %d", got, want)
	}
}

func TestApplyOperationSubFieldSelection_FirstChainsToEnumPickerUnquoted(t *testing.T) {
	ta := textarea.New()
	ti := textinput.New()
	m := Model{
		currentView:        viewPicker,
		pickerKind:         pickerObjectSubField,
		pickerObjectParent: "operation",
		pickerItems:        []pickerItem{{Display: "first", FilterKey: "first", Value: "first"}},
		pickerCursor:       0,
		query:              ta,
		pickerInput:        ti,
	}
	out, _ := m.applyPickerSelection()
	if out.pickerKind != pickerEnumValue {
		t.Errorf("pickerKind = %v, want pickerEnumValue", out.pickerKind)
	}
	if out.pickerEnumField != "operation.first" {
		t.Errorf("pickerEnumField = %q, want %q", out.pickerEnumField, "operation.first")
	}
	if out.pickerEnumQuoted {
		t.Error("pickerEnumQuoted = true, want false for bool sub-field")
	}
	if len(out.pickerItems) != len(boolEnumValues) {
		t.Errorf("enum item count = %d, want %d", len(out.pickerItems), len(boolEnumValues))
	}
}

func TestApplyFieldSelection_SourceLocationOpensSubFieldPicker(t *testing.T) {
	m := fieldPickerModelWithSelection("sourceLocation")
	out, cmd := m.applyPickerSelection()
	if out.pickerKind != pickerObjectSubField {
		t.Errorf("pickerKind = %v, want pickerObjectSubField", out.pickerKind)
	}
	if out.pickerObjectParent != "sourceLocation" {
		t.Errorf("pickerObjectParent = %q, want %q", out.pickerObjectParent, "sourceLocation")
	}
	if cmd != nil {
		t.Error("cmd = non-nil, want nil (sub-field picker is in-memory)")
	}
	if got, want := len(out.pickerItems), len(objectSubFields["sourceLocation"]); got != want {
		t.Errorf("sub-field items len = %d, want %d", got, want)
	}
}

func TestApplySourceLocationSubFieldSelection_LineInsertsSkeletonUnquoted(t *testing.T) {
	ta := textarea.New()
	ta.SetWidth(200)
	ti := textinput.New()
	m := Model{
		currentView:        viewPicker,
		pickerKind:         pickerObjectSubField,
		pickerObjectParent: "sourceLocation",
		pickerItems:        []pickerItem{{Display: "line", FilterKey: "line", Value: "line"}},
		pickerCursor:       0,
		query:              ta,
		pickerInput:        ti,
	}
	out, _ := m.applyPickerSelection()
	if out.currentView != viewMain {
		t.Errorf("currentView = %v, want viewMain", out.currentView)
	}
	want := `sourceLocation.line = `
	if got := out.query.Value(); got != want {
		t.Errorf("query = %q, want %q", got, want)
	}
	out.query.InsertRune('4')
	out.query.InsertRune('2')
	wantAfter := `sourceLocation.line = 42`
	if got := out.query.Value(); got != wantAfter {
		t.Errorf("after InsertRune, query = %q, want %q (cursor at end-of-line for unquoted)", got, wantAfter)
	}
}

func TestApplyFieldSelection_SplitOpensSubFieldPicker(t *testing.T) {
	m := fieldPickerModelWithSelection("split")
	out, cmd := m.applyPickerSelection()
	if out.pickerKind != pickerObjectSubField {
		t.Errorf("pickerKind = %v, want pickerObjectSubField", out.pickerKind)
	}
	if out.pickerObjectParent != "split" {
		t.Errorf("pickerObjectParent = %q, want %q", out.pickerObjectParent, "split")
	}
	if cmd != nil {
		t.Error("cmd = non-nil, want nil (sub-field picker is in-memory)")
	}
	if got, want := len(out.pickerItems), len(objectSubFields["split"]); got != want {
		t.Errorf("sub-field items len = %d, want %d", got, want)
	}
}

func TestApplySplitSubFieldSelection_UidInsertsSkeletonQuoted(t *testing.T) {
	ta := textarea.New()
	ta.SetWidth(200)
	ti := textinput.New()
	m := Model{
		currentView:        viewPicker,
		pickerKind:         pickerObjectSubField,
		pickerObjectParent: "split",
		pickerItems:        []pickerItem{{Display: "uid", FilterKey: "uid", Value: "uid"}},
		pickerCursor:       0,
		query:              ta,
		pickerInput:        ti,
	}
	out, _ := m.applyPickerSelection()
	want := `split.uid = ""`
	if got := out.query.Value(); got != want {
		t.Errorf("query = %q, want %q", got, want)
	}
	out.query.InsertRune('X')
	wantAfter := `split.uid = "X"`
	if got := out.query.Value(); got != wantAfter {
		t.Errorf("after InsertRune, query = %q, want %q (cursor between quotes)", got, wantAfter)
	}
}
