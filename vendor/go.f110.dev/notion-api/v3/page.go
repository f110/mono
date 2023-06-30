package notion

import (
	"bytes"
	"encoding/json"
	"errors"
)

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
	if value.Type == "" {
		value.Type = schema.Type
	}

	p.Properties[key] = value
}

func (p *Page) New() *Page {
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(p); err != nil {
		return nil
	}
	newPage := new(Page)
	if err := json.NewDecoder(buf).Decode(newPage); err != nil {
		return nil
	}
	newPage.ID = ""
	newPage.CreatedTime = nil
	newPage.LastEditedTime = nil
	newPage.URL = ""

	for k, v := range newPage.Properties {
		switch v.Type {
		case PropertyTypeNumber:
			if v.Number == nil {
				delete(newPage.Properties, k)
			}
		case PropertyTypeDate:
			if v.Date == nil {
				delete(newPage.Properties, k)
			}
		case PropertyTypePeople:
			if len(v.People) == 0 {
				delete(newPage.Properties, k)
			}
		case PropertyTypeCheckbox:
			if !v.Checkbox {
				delete(newPage.Properties, k)
			}
		case PropertyTypeURL:
			if v.URL == "" {
				delete(newPage.Properties, k)
			}
		case PropertyTypeEmail:
			if v.Email == "" {
				delete(newPage.Properties, k)
			}
		case PropertyTypePhoneNumber:
			if v.PhoneNumber == "" {
				delete(newPage.Properties, k)
			}
		case PropertyTypeRelation:
			if len(v.Relation) == 0 {
				delete(newPage.Properties, k)
			}
		case PropertyTypeLastEditedTime, PropertyTypeLastEditedBy,
			PropertyTypeCreatedTime, PropertyTypeCreatedBy:
			delete(newPage.Properties, k)
		case PropertyTypeSelect:
			if v.Select == nil {
				delete(newPage.Properties, k)
			}
		case PropertyTypeMultiSelect:
			if len(v.MultiSelect) == 0 {
				delete(newPage.Properties, k)
			}
		case PropertyTypeRollup:
			if v.RollupProperty == nil {
				delete(newPage.Properties, k)
				continue
			}
			switch v.RollupProperty.Type {
			case RollupTypeArray:
				if len(v.RollupProperty.Array) == 0 {
					delete(newPage.Properties, k)
				}
			case RollupTypeNumber:
				if v.RollupProperty.Number == nil {
					delete(newPage.Properties, k)
				}
			case RollupTypeDate:
				if v.RollupProperty.Date == nil {
					delete(newPage.Properties, k)
				}
			}
		case PropertyTypeUniqueID:
			if v.UniqueID != nil {
				delete(newPage.Properties, k)
			}
		}
	}

	return newPage
}
