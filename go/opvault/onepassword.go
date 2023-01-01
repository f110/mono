package opvault

type ItemDetail struct {
	Form   *HTMLForm `json:"htmlForm,omitempty"`
	Fields []*Field  `json:"fields,omitempty"`
}

type HTMLForm struct {
	Method string `json:"htmlMethod"`
	Action string `json:"htmlAction"`
}

type Field struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Value       string `json:"value"`
	Designation string `json:"designation"`
}

type ItemOverview struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}
