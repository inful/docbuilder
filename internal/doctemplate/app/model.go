package app

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	goslug "github.com/gosimple/slug"

	"git.home.luguber.info/inful/docbuilder/internal/doctemplate/field"
	"git.home.luguber.info/inful/docbuilder/internal/doctemplate/service"
	"git.home.luguber.info/inful/docbuilder/internal/doctemplate/suggest"
	templating "git.home.luguber.info/inful/docbuilder/internal/templates"
)

type step int

const (
	stepBaseURL step = iota
	stepSelectTemplate
	stepForm
	stepPreview
	stepDone
)

type model struct {
	service *service.TemplateService
	width   int

	step    step
	loading bool
	err     string

	baseURL string
	docsDir string
	baseIn  textinput.Model

	tplList list.Model

	page     *templating.TemplatePage
	schema   templating.TemplateSchema
	defaults map[string]any

	fields     []formField
	fieldIndex int
	autoSlug   string

	suggIndex   suggest.Index
	suggestions []string
	suggCursor  int
	taxTags     []string
	taxCats     []string

	previewPath string
	previewBody string
	previewVP   viewport.Model
	writtenPath string

	help help.Model
	keys keyMap
}

type formField struct {
	spec      templating.SchemaField
	textValue string
	boolValue field.BoolField
}

type templatesMsg struct {
	templates []service.TemplateRef
	err       error
}

type pageMsg struct {
	page     *templating.TemplatePage
	schema   templating.TemplateSchema
	defaults map[string]any
	err      error
}

type previewMsg struct {
	path string
	body string
	err  error
}

type writeMsg struct {
	path string
	err  error
}

type taxonomiesMsg struct {
	tags       []string
	categories []string
	err        error
}

type templateItem struct {
	ref service.TemplateRef
}

func (i templateItem) Title() string       { return i.ref.Type }
func (i templateItem) Description() string { return i.ref.URL }
func (i templateItem) FilterValue() string { return i.ref.Type + " " + i.ref.Name + " " + i.ref.URL }

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Accept  key.Binding
	Apply   key.Binding
	Preview key.Binding
	Back    key.Binding
	Write   key.Binding
	Quit    key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Accept:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "accept/next")),
		Apply:   key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "apply suggestion")),
		Preview: key.NewBinding(key.WithKeys("ctrl+g"), key.WithHelp("ctrl+g", "preview")),
		Back:    key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "back")),
		Write:   key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "write")),
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Accept, k.Apply, k.Preview, k.Back, k.Write, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Up, k.Down, k.Accept, k.Apply}, {k.Preview, k.Back, k.Write, k.Quit}}
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	stepStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("8")).
			Padding(0, 1)

	focusedRowStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	unfocusedRowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	metaStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	helpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	boolTrueStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	boolFalseStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	boolUnsetStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	outerStyle        = lipgloss.NewStyle().Padding(1, 2)
)

// NewModel creates a new Bubble Tea model for doctemplate.
func NewModel(svc *service.TemplateService, baseURL string) tea.Model {
	docsDir, _ := svc.ResolveDocsDir()
	sidx, _ := suggest.BuildFromDocs(docsDir)
	baseIn := textinput.New()
	baseIn.SetValue(baseURL)
	baseIn.Placeholder = "https://docs.example.com"
	baseIn.Focus()
	baseIn.Prompt = "> "

	listDelegate := list.NewDefaultDelegate()
	tplList := list.New(nil, listDelegate, 0, 8)
	tplList.Title = "Templates"
	tplList.SetShowStatusBar(false)
	tplList.SetFilteringEnabled(true)
	tplList.SetShowHelp(false)

	h := help.New()
	h.ShowAll = false

	vp := viewport.New(80, 12)

	return &model{
		service:   svc,
		step:      stepBaseURL,
		baseURL:   baseURL,
		docsDir:   docsDir,
		baseIn:    baseIn,
		defaults:  map[string]any{},
		suggIndex: sidx,
		tplList:   tplList,
		previewVP: vp,
		help:      h,
		keys:      defaultKeyMap(),
	}
}

func (m *model) Init() tea.Cmd { return nil }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.KeyMsg:
		return m.updateKey(typed)
	case tea.WindowSizeMsg:
		m.width = typed.Width
		usableWidth := typed.Width - outerStyle.GetHorizontalFrameSize()
		if usableWidth <= 0 {
			usableWidth = typed.Width
		}
		panelInnerWidth := max(usableWidth-panelStyle.GetHorizontalFrameSize(), 20)

		m.tplList.SetSize(panelInnerWidth, 12)
		m.previewVP.Width = panelInnerWidth
		m.previewVP.Height = 14
		m.baseIn.Width = max(10, panelInnerWidth-2)
		if strings.TrimSpace(m.previewBody) != "" {
			m.previewVP.SetContent(renderMarkdownForPreview(m.previewBody, m.previewVP.Width))
		}
		return m, nil
	case templatesMsg:
		m.loading = false
		if typed.err != nil {
			m.err = typed.err.Error()
			return m, nil
		}
		m.err = ""
		items := make([]list.Item, 0, len(typed.templates))
		for _, tmpl := range typed.templates {
			items = append(items, templateItem{ref: tmpl})
		}
		_ = m.tplList.SetItems(items)
		m.tplList.Select(0)
		m.step = stepSelectTemplate
		return m, nil
	case pageMsg:
		m.loading = false
		if typed.err != nil {
			m.err = typed.err.Error()
			return m, nil
		}
		m.err = ""
		m.page = typed.page
		m.schema = typed.schema
		m.defaults = typed.defaults
		m.fields = buildFields(m.schema, m.defaults)
		m.fieldIndex = 0
		m.autoSlug = ""
		m.suggCursor = 0
		m.refreshSuggestions()
		m.step = stepForm
		return m, nil
	case taxonomiesMsg:
		if typed.err == nil {
			m.taxTags = typed.tags
			m.taxCats = typed.categories
			if m.step == stepForm {
				m.refreshSuggestions()
			}
		}
		return m, nil
	case previewMsg:
		m.loading = false
		if typed.err != nil {
			m.err = typed.err.Error()
			return m, nil
		}
		m.err = ""
		m.previewPath = typed.path
		m.previewBody = typed.body
		m.previewVP.SetContent(renderMarkdownForPreview(typed.body, m.previewVP.Width))
		m.previewVP.GotoTop()
		m.step = stepPreview
		return m, nil
	case writeMsg:
		m.loading = false
		if typed.err != nil {
			m.err = typed.err.Error()
			return m, nil
		}
		m.err = ""
		m.writtenPath = typed.path
		m.step = stepDone
		return m, nil
	}
	return m, nil
}

func (m *model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if msg.String() == "q" && !m.isTextEntryContext() {
		return m, tea.Quit
	}
	if m.loading {
		return m, nil
	}

	switch m.step {
	case stepBaseURL:
		return m.updateBaseURL(msg)
	case stepSelectTemplate:
		return m.updateSelect(msg)
	case stepForm:
		return m.updateForm(msg)
	case stepPreview:
		return m.updatePreview(msg)
	case stepDone:
		if msg.String() == "enter" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *model) isTextEntryContext() bool {
	switch m.step {
	case stepBaseURL:
		return true
	case stepForm:
		if len(m.fields) == 0 || m.fieldIndex < 0 || m.fieldIndex >= len(m.fields) {
			return false
		}
		current := m.fields[m.fieldIndex]
		return current.spec.Type != templating.FieldTypeBool
	case stepSelectTemplate, stepPreview, stepDone:
		return false
	default:
		return false
	}
}

func (m *model) updateBaseURL(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.baseIn, cmd = m.baseIn.Update(msg)
	m.baseURL = m.baseIn.Value()

	if msg.String() == "enter" {
		if strings.TrimSpace(m.baseIn.Value()) == "" {
			m.err = "base URL is required"
			return m, nil
		}
		baseURL := strings.TrimSpace(m.baseIn.Value())
		m.loading = true
		m.err = ""
		return m, tea.Batch(
			func() tea.Msg {
				templates, err := m.service.Discover(context.Background(), baseURL)
				return templatesMsg{templates: templates, err: err}
			},
			func() tea.Msg {
				tags, categories, err := fetchTaxonomies(context.Background(), baseURL)
				return taxonomiesMsg{tags: tags, categories: categories, err: err}
			},
		)
	}
	return m, cmd
}

func (m *model) updateSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.tplList, cmd = m.tplList.Update(msg)

	if msg.String() == "enter" {
		selectedItem, ok := m.tplList.SelectedItem().(templateItem)
		if !ok {
			return m, nil
		}
		selected := selectedItem.ref
		m.loading = true
		m.err = ""
		return m, func() tea.Msg {
			page, err := m.service.FetchPage(context.Background(), selected.URL)
			if err != nil {
				return pageMsg{err: err}
			}
			schema, err := m.service.ParseSchema(page.Meta.Schema)
			if err != nil {
				return pageMsg{err: err}
			}
			defaults, err := m.service.ParseDefaults(page.Meta.Defaults)
			if err != nil {
				return pageMsg{err: err}
			}
			return pageMsg{page: page, schema: schema, defaults: defaults}
		}
	}
	return m, cmd
}

func (m *model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.fields) == 0 {
		if msg.String() == "enter" {
			return m, m.previewCmd()
		}
		return m, nil
	}

	current := &m.fields[m.fieldIndex]

	switch msg.String() {
	case "up", "shift+tab":
		m.moveField(-1)
		return m, nil
	case "down":
		m.moveField(1)
		return m, nil
	case "ctrl+n":
		if len(m.suggestions) > 0 {
			m.suggCursor = (m.suggCursor + 1) % len(m.suggestions)
		}
		return m, nil
	case "ctrl+b":
		if len(m.suggestions) > 0 {
			m.suggCursor--
			if m.suggCursor < 0 {
				m.suggCursor = len(m.suggestions) - 1
			}
		}
		return m, nil
	case "enter":
		if len(m.suggestions) > 0 && current.spec.Type != templating.FieldTypeBool {
			applySuggestion(current, m.suggestions[m.suggCursor])
			m.maybeAutoSuggestSlug()
			m.refreshSuggestions()
			return m, nil
		}
		if m.fieldIndex < len(m.fields)-1 {
			m.moveField(1)
			return m, nil
		}
		return m, m.previewCmd()
	case "tab":
		if len(m.suggestions) > 0 {
			applySuggestion(current, m.suggestions[m.suggCursor])
			m.maybeAutoSuggestSlug()
			m.refreshSuggestions()
		}
		return m, nil
	case "ctrl+g":
		return m, m.previewCmd()
	}

	m.handleFormFieldInput(current, msg)
	return m, nil
}

func (m *model) moveField(delta int) {
	if delta == 0 || len(m.fields) == 0 {
		return
	}
	next := m.fieldIndex + delta
	if next < 0 || next >= len(m.fields) {
		return
	}
	m.fieldIndex = next
	m.suggCursor = 0
	m.refreshSuggestions()
}

func (m *model) handleFormFieldInput(current *formField, msg tea.KeyMsg) {
	switch msg.String() {
	case "left":
		switch current.spec.Type {
		case templating.FieldTypeBool:
			current.boolValue.SetFalse()
		case templating.FieldTypeStringEnum:
			cycleEnum(current, -1)
			m.refreshSuggestions()
		case templating.FieldTypeString, templating.FieldTypeStringList:
			// No left-action for free text field types.
		}
	case "right":
		switch current.spec.Type {
		case templating.FieldTypeBool:
			current.boolValue.SetTrue()
		case templating.FieldTypeStringEnum:
			cycleEnum(current, 1)
			m.refreshSuggestions()
		case templating.FieldTypeString, templating.FieldTypeStringList:
			// No right-action for free text field types.
		}
	case "backspace", "delete":
		if current.spec.Type == templating.FieldTypeBool {
			current.boolValue.Clear()
			return
		}
		if len(current.textValue) > 0 {
			current.textValue = current.textValue[:len(current.textValue)-1]
			m.maybeAutoSuggestSlug()
			m.refreshSuggestions()
		}
	default:
		if current.spec.Type != templating.FieldTypeBool && (msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace) {
			if msg.Type == tea.KeySpace {
				current.textValue += " "
			} else {
				current.textValue += string(msg.Runes)
			}
			m.maybeAutoSuggestSlug()
			m.refreshSuggestions()
		}
	}
}

func (m *model) maybeAutoSuggestSlug() {
	titleIdx := findFieldIndexByToken(m.fields, "title")
	slugIdx := findFieldIndexByToken(m.fields, "slug")
	if titleIdx < 0 || slugIdx < 0 {
		return
	}

	titleValue := strings.TrimSpace(m.fields[titleIdx].textValue)
	nextSlug := goslug.Make(titleValue)

	slugField := &m.fields[slugIdx]
	currentSlug := strings.TrimSpace(slugField.textValue)
	if currentSlug != "" && currentSlug != m.autoSlug {
		return
	}

	slugField.textValue = nextSlug
	m.autoSlug = nextSlug
}

func findFieldIndexByToken(fields []formField, token string) int {
	lowerToken := strings.ToLower(strings.TrimSpace(token))
	if lowerToken == "" {
		return -1
	}

	for i, f := range fields {
		if strings.EqualFold(strings.TrimSpace(f.spec.Key), lowerToken) {
			return i
		}
	}
	for i, f := range fields {
		if strings.Contains(strings.ToLower(strings.TrimSpace(f.spec.Key)), lowerToken) {
			return i
		}
	}
	return -1
}

func (m *model) updatePreview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.previewVP, cmd = m.previewVP.Update(msg)

	switch msg.String() {
	case "b":
		m.step = stepForm
		return m, nil
	case "y", "enter":
		if m.previewPath == "" {
			m.err = "preview path is empty"
			return m, nil
		}
		m.loading = true
		m.err = ""
		return m, func() tea.Msg {
			path, err := m.service.Write(m.docsDir, m.previewPath, m.previewBody)
			return writeMsg{path: path, err: err}
		}
	}
	return m, cmd
}

func (m *model) previewCmd() tea.Cmd {
	data, err := collectData(m.fields, m.defaults)
	if err != nil {
		m.err = err.Error()
		return nil
	}
	m.loading = true
	m.err = ""
	return func() tea.Msg {
		resolver, err := m.service.BuildSequenceResolver(m.page, m.docsDir)
		if err != nil {
			return previewMsg{err: err}
		}
		path, body, err := m.service.RenderPreview(m.page, data, resolver)
		return previewMsg{path: path, body: body, err: err}
	}
}

func (m *model) View() string {
	if m.loading {
		return panelStyle.Render("Loading...") + "\n"
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("DocTemplate"))
	b.WriteString("\n")
	b.WriteString(stepStyle.Render(m.stepLabel()))
	b.WriteString("\n\n")
	if m.err != "" {
		b.WriteString(errorStyle.Render("Error: "+m.err) + "\n\n")
	}

	switch m.step {
	case stepBaseURL:
		content := "Enter template base URL and press Enter:\n"
		if strings.TrimSpace(m.baseURL) == "" {
			content += "\n" + metaStyle.Render("Tip: set DOCBUILDER_TEMPLATE_BASE_URL to skip this step")
		}
		b.WriteString(panelStyle.Render(content+"\n\n"+m.baseIn.View()) + "\n")
		b.WriteString(m.help.View(m.keys))
	case stepSelectTemplate:
		b.WriteString(panelStyle.Render(m.tplList.View()))
		b.WriteString("\n" + m.help.View(m.keys))
	case stepForm:
		if len(m.fields) == 0 {
			b.WriteString(panelStyle.Render("Template has no schema fields. Press Enter to preview."))
			break
		}
		b.WriteString(panelStyle.Render("Fill template fields"))
		b.WriteString("\n")
		for i := range m.fields {
			f := m.fields[i]
			prefix := "  "
			rowStyle := unfocusedRowStyle
			if i == m.fieldIndex {
				prefix = "> "
				rowStyle = focusedRowStyle
			}
			value := f.textValue
			if f.spec.Type == templating.FieldTypeBool {
				switch f.boolValue.State {
				case field.TriTrue:
					value = boolTrueStyle.Render("true")
				case field.TriFalse:
					value = boolFalseStyle.Render("false")
				case field.TriUnset:
					value = boolUnsetStyle.Render("<unset>")
				default:
					value = boolUnsetStyle.Render("<unset>")
				}
			}
			req := ""
			if f.spec.Required {
				req = "*"
			}
			fmt.Fprintf(&b, "%s\n", rowStyle.Render(fmt.Sprintf("%s%s%s [%s]: %s", prefix, f.spec.Key, req, f.spec.Type, value)))
			if f.spec.Type == templating.FieldTypeStringEnum && len(f.spec.Options) > 0 && i == m.fieldIndex {
				fmt.Fprintf(&b, "%s\n", metaStyle.Render("    options: "+strings.Join(f.spec.Options, ", ")))
			}
		}
		if len(m.suggestions) > 0 {
			b.WriteString("\n" + panelStyle.Render("Suggestions") + "\n")
			for i, suggestion := range m.suggestions {
				prefix := "  "
				rowStyle := unfocusedRowStyle
				if i == m.suggCursor {
					prefix = "> "
					rowStyle = focusedRowStyle
				}
				fmt.Fprintf(&b, "%s\n", rowStyle.Render(prefix+suggestion))
			}
		}
		b.WriteString("\n" + helpStyle.Render("Keys: up/down field, left/right bool or enum, ctrl+n/ctrl+b suggestion, tab or enter apply, ctrl+g preview, q"))
	case stepPreview:
		b.WriteString(panelStyle.Render("Preview"))
		b.WriteString("\n")
		b.WriteString(stepStyle.Render("Output: " + filepath.ToSlash(filepath.Join("docs", m.previewPath))))
		b.WriteString("\n\n")
		b.WriteString(panelStyle.Render(m.previewVP.View()) + "\n")
		b.WriteString(m.help.View(m.keys))
	case stepDone:
		b.WriteString(panelStyle.Render("Created file:\n" + m.writtenPath))
		b.WriteString("\n")
		b.WriteString(m.help.View(m.keys))
	}

	out := b.String()
	if m.width > 0 {
		out = outerStyle.Render(out)
	}
	return out
}

func buildFields(schema templating.TemplateSchema, defaults map[string]any) []formField {
	fields := make([]formField, 0, len(schema.Fields))
	for _, spec := range schema.Fields {
		fieldState := formField{spec: spec}
		if defaultValue, ok := defaults[spec.Key]; ok {
			switch spec.Type {
			case templating.FieldTypeBool:
				if v, okBool := defaultValue.(bool); okBool {
					fieldState.boolValue = field.NewBoolField(spec.Required, &v)
				} else {
					fieldState.boolValue = field.NewBoolField(spec.Required, nil)
				}
			case templating.FieldTypeString, templating.FieldTypeStringEnum:
				fieldState.textValue = fmt.Sprintf("%v", defaultValue)
			case templating.FieldTypeStringList:
				switch v := defaultValue.(type) {
				case []string:
					fieldState.textValue = strings.Join(v, ",")
				case []any:
					parts := make([]string, 0, len(v))
					for _, item := range v {
						parts = append(parts, fmt.Sprintf("%v", item))
					}
					fieldState.textValue = strings.Join(parts, ",")
				default:
					fieldState.textValue = fmt.Sprintf("%v", defaultValue)
				}
			default:
				fieldState.textValue = fmt.Sprintf("%v", defaultValue)
			}
		} else if spec.Type == templating.FieldTypeBool {
			fieldState.boolValue = field.NewBoolField(spec.Required, nil)
		}
		fields = append(fields, fieldState)
	}
	return fields
}

func collectData(fields []formField, defaults map[string]any) (map[string]any, error) {
	result := make(map[string]any, len(defaults)+len(fields))
	maps.Copy(result, defaults)

	for _, f := range fields {
		switch f.spec.Type {
		case templating.FieldTypeBool:
			if err := f.boolValue.Validate(); err != nil {
				return nil, fmt.Errorf("%s: required boolean value is missing", f.spec.Key)
			}
			if value, ok := f.boolValue.Value(); ok {
				result[f.spec.Key] = value
			} else {
				delete(result, f.spec.Key)
			}
		case templating.FieldTypeString:
			value := strings.TrimSpace(f.textValue)
			if value == "" {
				if f.spec.Required {
					return nil, fmt.Errorf("%s: required value is missing", f.spec.Key)
				}
				delete(result, f.spec.Key)
				continue
			}
			result[f.spec.Key] = value
		case templating.FieldTypeStringEnum:
			value := strings.TrimSpace(f.textValue)
			if value == "" {
				if f.spec.Required {
					return nil, fmt.Errorf("%s: required value is missing", f.spec.Key)
				}
				delete(result, f.spec.Key)
				continue
			}
			if len(f.spec.Options) > 0 && !slices.Contains(f.spec.Options, value) {
				return nil, fmt.Errorf("%s: invalid option %q", f.spec.Key, value)
			}
			result[f.spec.Key] = value
		case templating.FieldTypeStringList:
			raw := strings.TrimSpace(f.textValue)
			if raw == "" {
				if f.spec.Required {
					return nil, fmt.Errorf("%s: required value is missing", f.spec.Key)
				}
				delete(result, f.spec.Key)
				continue
			}
			parts := strings.Split(raw, ",")
			items := make([]string, 0, len(parts))
			for _, part := range parts {
				item := strings.TrimSpace(part)
				if item != "" {
					items = append(items, item)
				}
			}
			if f.spec.Required && len(items) == 0 {
				return nil, fmt.Errorf("%s: required value is missing", f.spec.Key)
			}
			if len(items) > 0 {
				result[f.spec.Key] = items
			} else {
				delete(result, f.spec.Key)
			}
		default:
			return nil, fmt.Errorf("%s: unsupported field type %q", f.spec.Key, f.spec.Type)
		}
	}

	return result, nil
}

func (m *model) refreshSuggestions() {
	m.suggestions = nil
	input := ""
	if len(m.fields) == 0 || m.fieldIndex >= len(m.fields) {
		m.suggCursor = 0
		return
	}
	current := m.fields[m.fieldIndex]
	input = current.textValue
	if current.spec.Type == templating.FieldTypeStringList || current.spec.Type == templating.FieldTypeStringEnum {
		input = currentToken(input)
	}

	// For taxonomy fields, prioritize remote taxonomy values over filesystem guesses.
	if m.refreshTaxonomySuggestions(current, input) {
		return
	}

	if strings.TrimSpace(current.spec.GlobSuggestion) != "" {
		m.suggestions = m.suggIndex.SuggestFromGlob(current.spec.GlobSuggestion, input, 8)
		if len(m.suggestions) == 0 && strings.TrimSpace(input) != "" {
			// When the current input doesn't match any glob-derived values,
			// still show available values so users can discover valid options.
			m.suggestions = m.suggIndex.SuggestFromGlob(current.spec.GlobSuggestion, "", 8)
		}
		m.suggestions = mergeSuggestions(m.suggestions, m.taxonomySuggestionsForField(current.spec.Key, input), 8)
		if len(m.suggestions) == 0 {
			m.suggCursor = 0
			return
		}
		if m.suggCursor >= len(m.suggestions) {
			m.suggCursor = 0
		}
		return
	}

	if current.spec.Type == templating.FieldTypeStringEnum && len(current.spec.Options) > 0 {
		m.suggestions = filterOptions(current.spec.Options, current.textValue, 8)
		m.suggestions = mergeSuggestions(m.suggestions, m.taxonomySuggestionsForField(current.spec.Key, input), 8)
		if len(m.suggestions) == 0 {
			m.suggCursor = 0
			return
		}
		if m.suggCursor >= len(m.suggestions) {
			m.suggCursor = 0
		}
		return
	}
	m.suggestions = m.taxonomySuggestionsForField(current.spec.Key, input)
	if len(m.suggestions) == 0 {
		m.suggCursor = 0
		return
	}
	if m.suggCursor >= len(m.suggestions) {
		m.suggCursor = 0
	}
}

func (m *model) taxonomySuggestionsForField(fieldKey, input string) []string {
	const taxonomySuggestionLimit = 512
	lower := strings.ToLower(strings.TrimSpace(fieldKey))
	if strings.Contains(lower, "tag") {
		return filterOptions(m.taxTags, input, taxonomySuggestionLimit)
	}
	if strings.Contains(lower, "categor") {
		return filterOptions(m.taxCats, input, taxonomySuggestionLimit)
	}
	return nil
}

func isTaxonomyField(fieldKey string) bool {
	lower := strings.ToLower(strings.TrimSpace(fieldKey))
	return strings.Contains(lower, "tag") || strings.Contains(lower, "categor")
}

func (m *model) refreshTaxonomySuggestions(current formField, input string) bool {
	if !isTaxonomyField(current.spec.Key) {
		return false
	}

	m.suggestions = m.taxonomySuggestionsForField(current.spec.Key, input)
	if len(m.suggestions) == 0 {
		m.suggestions = m.fallbackSuggestions(current, input)
	}
	if len(m.suggestions) == 0 {
		m.suggCursor = 0
		return true
	}
	if m.suggCursor >= len(m.suggestions) {
		m.suggCursor = 0
	}
	return true
}

func (m *model) fallbackSuggestions(current formField, input string) []string {
	if strings.TrimSpace(current.spec.GlobSuggestion) != "" {
		return m.suggIndex.SuggestFromGlob(current.spec.GlobSuggestion, input, 8)
	}
	return nil
}

func mergeSuggestions(primary, secondary []string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	if len(primary) == 0 && len(secondary) == 0 {
		return nil
	}
	result := make([]string, 0, limit)
	seen := map[string]struct{}{}
	appendUnique := func(items []string) {
		for _, item := range items {
			if len(result) >= limit {
				return
			}
			key := strings.ToLower(strings.TrimSpace(item))
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, item)
		}
	}
	appendUnique(primary)
	appendUnique(secondary)
	return result
}

func cycleEnum(f *formField, delta int) {
	if f.spec.Type != templating.FieldTypeStringEnum || len(f.spec.Options) == 0 {
		return
	}
	idx := slices.Index(f.spec.Options, strings.TrimSpace(f.textValue))
	if idx < 0 {
		if delta < 0 {
			idx = len(f.spec.Options)
		} else {
			idx = -1
		}
	}
	idx += delta
	if idx < 0 {
		idx = len(f.spec.Options) - 1
	}
	if idx >= len(f.spec.Options) {
		idx = 0
	}
	f.textValue = f.spec.Options[idx]
}

func filterOptions(options []string, input string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	needle := strings.ToLower(strings.TrimSpace(input))
	prefix := make([]string, 0, limit)
	contains := make([]string, 0, limit)
	for _, option := range options {
		lower := strings.ToLower(option)
		if needle == "" || strings.HasPrefix(lower, needle) {
			prefix = append(prefix, option)
			continue
		}
		if strings.Contains(lower, needle) {
			contains = append(contains, option)
		}
	}
	result := make([]string, 0, limit)
	for _, item := range prefix {
		result = append(result, item)
		if len(result) == limit {
			return result
		}
	}
	for _, item := range contains {
		result = append(result, item)
		if len(result) == limit {
			return result
		}
	}
	return result
}

func (m *model) stepLabel() string {
	switch m.step {
	case stepBaseURL:
		return "Step 1/5 - Base URL"
	case stepSelectTemplate:
		return "Step 2/5 - Template Selection"
	case stepForm:
		return "Step 3/5 - Field Entry"
	case stepPreview:
		return "Step 4/5 - Preview"
	case stepDone:
		return "Step 5/5 - Complete"
	default:
		return ""
	}
}

func currentToken(value string) string {
	parts := strings.Split(value, ",")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[len(parts)-1])
}

func applySuggestion(field *formField, suggestion string) {
	if field.spec.Type == templating.FieldTypeStringList {
		parts := strings.Split(field.textValue, ",")
		if len(parts) == 0 {
			field.textValue = suggestion
			return
		}
		parts[len(parts)-1] = " " + suggestion
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		field.textValue = strings.Join(parts, ", ")
		return
	}
	field.textValue = suggestion
}

func renderMarkdownForPreview(markdown string, width int) string {
	if strings.TrimSpace(markdown) == "" {
		return ""
	}
	wrap := 80
	if width > 0 {
		wrap = max(40, width-2)
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(wrap),
	)
	if err != nil {
		return markdown
	}
	out, err := renderer.Render(markdown)
	if err != nil {
		return markdown
	}
	return out
}
