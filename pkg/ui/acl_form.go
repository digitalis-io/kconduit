package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/digitalis-io/kconduit/pkg/kafka"
)

// Shared presentation for the three ACL dialogs.
//
// They are huh forms rather than hand-rolled inputs, because the operations
// field is a multi-select and huh does that well. What they lacked was the
// framing the rest of the app has: each drew its own title, its own error
// colour, and its own help line, left-aligned against the top of the terminal
// with hardcoded colour numbers. Everything here is the part that is not the
// form itself — the frame, the palette, and the plain-English summary of what
// the dialog is about to do.

// aclFrame is what surrounds a form.
type aclFrame struct {
	title   string
	summary string // what the dialog will do, in plain English
	form    string // the huh form's own view
	status  string // error or progress line, already styled
	help    []string
	danger  bool // frame the box as destructive
	width   int
	height  int
}

// render lays the frame out and centres it, matching the create-topic form.
func (f aclFrame) render() string {
	var sb strings.Builder

	titleStyle := sectionTitleStyle
	if f.danger {
		titleStyle = sectionTitleStyle.Foreground(theme.Error)
	}

	sb.WriteString(titleStyle.Render(f.title))
	sb.WriteString("\n\n")

	if f.summary != "" {
		sb.WriteString(f.summary)
		sb.WriteString("\n")
		sb.WriteString(helpSepStyle.Render(strings.Repeat("─", max(f.boxWidth()-6, 10))))
		sb.WriteString("\n\n")
	}

	sb.WriteString(f.form)

	if f.status != "" {
		sb.WriteString("\n\n")
		sb.WriteString(f.status)
	}

	box := overlayBoxStyle(f.boxWidth())
	if f.danger {
		box = box.BorderForeground(theme.Error)
	}

	content := lipgloss.JoinVertical(lipgloss.Center,
		box.Render(sb.String()),
		"",
		renderHelpBar(f.help...),
	)

	return renderOverlay(content, f.width, f.height)
}

// boxWidth keeps the frame readable on a narrow terminal and stops it sprawling
// on a wide one. ACL forms carry more fields than the topic form, so they are
// allowed to be wider.
func (f aclFrame) boxWidth() int {
	if f.width <= 0 {
		return 72
	}
	return min(max(f.width-8, 40), 84)
}

// aclHuhTheme dresses huh in the application palette.
//
// huh ships its own themes with their own colours; left alone, an ACL form sits
// inside a KConduit frame looking like a different program. This maps the parts
// that show onto the same theme every other view uses.
func aclHuhTheme() *huh.Theme {
	t := huh.ThemeCharm()

	t.Focused.Base = t.Focused.Base.BorderForeground(theme.Primary)
	t.Focused.Title = t.Focused.Title.Foreground(theme.Primary).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(theme.Muted)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(theme.Accent)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(theme.Primary)
	t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(theme.Primary)
	t.Focused.SelectedPrefix = t.Focused.SelectedPrefix.Foreground(theme.Success)
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(theme.SubText)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(theme.Error)
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(theme.Error)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(theme.Primary)
	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(theme.Primary)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(theme.Muted)
	t.Focused.FocusedButton = t.Focused.FocusedButton.
		Foreground(lipgloss.Color("229")).
		Background(theme.Secondary).
		Bold(true)
	t.Focused.BlurredButton = t.Focused.BlurredButton.
		Foreground(theme.SubText).
		Background(theme.BgDark)

	t.Blurred.Title = t.Blurred.Title.Foreground(theme.SubText)
	t.Blurred.Description = t.Blurred.Description.Foreground(theme.DimBorder)
	t.Blurred.SelectedOption = t.Blurred.SelectedOption.Foreground(theme.SubText)

	return t
}

// aclFormHeight is how tall the embedded huh form may be, given the terminal.
func aclFormHeight(terminalHeight int) int {
	// Border, padding, title, summary, rule, help line, and the status line the
	// form may need to show underneath it.
	return min(max(terminalHeight-14, 15), 40)
}

// aclSummary describes an ACL in the order an operator reads it: who, what they
// may do, and to which resource.
//
// The field names on their own ("Pattern: Prefixed") leave the reader to
// assemble the meaning. This assembles it for them, which matters most in the
// delete dialog, where the thing being confirmed is exactly this sentence.
func aclSummary(acl kafka.ACL) string {
	verb := "may"
	verbStyle := successStyle
	if strings.EqualFold(acl.PermissionType, "Deny") {
		verb = "may not"
		verbStyle = errorStyle
	}

	resource := describeResource(acl.ResourceType, acl.ResourceName, acl.PatternType)

	return valueStyle.Render(orDash(acl.Principal)) + " " +
		verbStyle.Render(verb+" "+strings.ToLower(orDash(acl.Operation))) + " " +
		labelStyle.Render("on ") + valueStyle.Render(resource) + "\n" +
		labelStyle.Render("from host ") + valueStyle.Render(orDash(acl.Host))
}

// draftACLSummary is aclSummary for a form still being filled in, where the
// operations field holds several values and any field may still be blank.
func draftACLSummary(principal, host, resourceType, resourceName, patternType, permission string, operations []string) string {
	action := "no operations selected"
	if len(operations) > 0 {
		action = strings.ToLower(strings.Join(operations, ", "))
	}

	verb := "may"
	verbStyle := successStyle
	if strings.EqualFold(permission, "Deny") {
		verb = "may not"
		verbStyle = errorStyle
	}

	resource := describeResource(resourceType, resourceName, patternType)

	return valueStyle.Render(orDash(principal)) + " " +
		verbStyle.Render(verb+" "+action) + " " +
		labelStyle.Render("on ") + valueStyle.Render(resource) + "\n" +
		labelStyle.Render("from host ") + valueStyle.Render(orDash(host))
}

// describeResource turns a resource type, name, and pattern into the phrase an
// operator would use for it.
func describeResource(resourceType, resourceName, patternType string) string {
	resourceType = strings.ToLower(orDash(resourceType))

	switch {
	case resourceName == "" || resourceName == "-":
		return "any " + resourceType
	case resourceName == "*":
		return "every " + resourceType
	case strings.EqualFold(patternType, "Prefixed"):
		return fmt.Sprintf("%ss starting with %q", resourceType, resourceName)
	case strings.EqualFold(patternType, "Any"):
		return fmt.Sprintf("%s matching %q", resourceType, resourceName)
	default:
		return fmt.Sprintf("%s %q", resourceType, resourceName)
	}
}

// aclProgress renders the spinner line a dialog shows while the cluster is
// working.
func aclProgress(spinnerView, message string) string {
	return spinnerView + "  " + labelStyle.Render(message)
}

// aclFieldWidth is the label column of the ACL field table.
const aclFieldWidth = 14

// aclFieldTable lists an ACL's fields in aligned rows.
//
// The plain-English summary says what the rule means; this says exactly what it
// is made of. The delete dialog shows both, because confirming a deletion wants
// the precise values as well as the meaning.
func aclFieldTable(acl kafka.ACL) string {
	fields := []struct{ label, value string }{
		{"Principal", acl.Principal},
		{"Host", acl.Host},
		{"Resource", strings.TrimSpace(acl.ResourceType + " " + acl.ResourceName)},
		{"Pattern", acl.PatternType},
		{"Operation", acl.Operation},
		{"Permission", acl.PermissionType},
	}

	var sb strings.Builder
	for i, f := range fields {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(labelStyle.Render(pad(f.label, aclFieldWidth)))
		sb.WriteString(valueStyle.Render(orDash(f.value)))
	}
	return sb.String()
}
