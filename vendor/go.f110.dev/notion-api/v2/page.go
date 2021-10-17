package notion

import "errors"

func NewPage(db *Database, title string, children []*Block) (*Page, error) {
	if db.ID == "" {
		return nil, errors.New("notion: not specified parent database")
	}
	var titleID string
	for _, prop := range db.Properties {
		if prop.Type != "title" {
			continue
		}
		titleID = prop.ID
	}
	if titleID == "" {
		return nil, errors.New("notion: title property can't be found")
	}

	return &Page{
		Parent: &PageParent{
			DatabaseID: db.ID,
			Database:   db,
		},
		Properties: map[string]*PropertyData{
			titleID: {
				Type: "title",
				Title: []*RichTextObject{
					{
						Type: "text",
						Text: &Text{
							Content: title,
						},
					},
				},
			},
		},
		Children: children,
	}, nil
}

func (p *Page) SetProperty(key string, value *PropertyData) {
	if p.Parent == nil || p.Parent.Database == nil {
		return
	}

	var schema *PropertyMetadata
	for k, v := range p.Parent.Database.Properties {
		if k == key {
			schema = v
			break
		}
	}
	if schema == nil {
		return
	}

	p.Properties[key] = value
}
