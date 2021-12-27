package notion

func NewDatabase(parent *Page, title string) *Database {
	return &Database{
		Parent: &PageParent{
			Type:   "page_id",
			PageID: parent.ID,
		},
		Title: []*RichTextObject{
			{
				Type: "text",
				Text: &Text{
					Content: title,
				},
			},
		},
		Properties: make(map[string]*PropertyMetadata),
	}
}

func (db *Database) SetProperty(key string, meta *PropertyMetadata) {
	if db.Properties == nil {
		db.Properties = make(map[string]*PropertyMetadata)
	}

	db.Properties[key] = meta
}
