package terminalui

// CustomFieldChoice is an action, never a stored field name.
const CustomFieldChoice = "custom"

// FieldNameSelection supplies the same presets to command and browser flows.
func FieldNameSelection(context string, finish bool) SelectionSpec {
	options := []Option{
		{Label: "email", Value: "email"},
		{Label: "password", Value: "password"},
		{Label: "username", Value: "username"},
		{Label: "url", Value: "url"},
		{Label: "api_key", Value: "api_key"},
		{Label: "token", Value: "token"},
		{Label: "Custom field", Value: CustomFieldChoice},
	}
	if finish {
		options = append(options, Option{Label: "Finish", Value: ""})
	}
	return SelectionSpec{Context: context, Title: "Choose field name", Options: options}
}
