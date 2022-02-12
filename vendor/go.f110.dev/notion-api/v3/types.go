package notion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type Object interface{}

type ObjectType string

const (
	ObjectTypeDatabase ObjectType = "database"
	ObjectTypePage     ObjectType = "page"
	ObjectTypeBlock    ObjectType = "block"
	ObjectTypeList     ObjectType = "list"
)

type Meta struct {
	// Type of object
	Object ObjectType `json:"object,omitempty"`
	// Unique identifier
	ID string `json:"id,omitempty"`
}

type ListMeta struct {
	// Type of object
	Object     ObjectType `json:"object,omitempty"`
	HasMore    bool       `json:"has_more,omitempty"`
	NextCursor string     `json:"next_cursor,omitempty"`
}

type Time struct {
	time.Time
}

const ISO8601 = "2006-01-02T15:04:05.999-0700"

func (t *Time) UnmarshalJSON(data []byte) error {
	// {} is Zero.
	// []byte{123, 125} is "{}".
	if bytes.Equal(data, []byte{123, 125}) {
		t.Time = time.Time{}
		return nil
	}
	if bytes.HasSuffix(data, []byte("Z\"")) {
		p, err := time.Parse("\"2006-01-02T15:04:05.999Z\"", string(data))
		if err != nil {
			return err
		}
		t.Time = p
		return nil
	}

	p, err := time.Parse("\""+ISO8601+"\"", string(data))
	if err != nil {
		return err
	}
	t.Time = p
	return nil
}

func (t Time) MarshalJSON() ([]byte, error) {
	return []byte("\"" + t.Time.Format(ISO8601) + "\""), nil
}

type UserType string

const (
	UserTypePerson UserType = "person"
	UserTypeBot    UserType = "bot"
)

type User struct {
	*Meta

	// Type of the user.
	Type UserType `json:"type"`
	// Displayed name
	Name string `json:"name"`
	// Avatar image url
	AvatarURL string `json:"avatar_url"`

	Person *Person `json:"person"`
	Bot    *Bot    `json:"bot"`
}

func (u *User) String() string {
	var b strings.Builder
	b.WriteString(u.Name)
	if u.Person != nil && u.Person.Email != "" {
		b.WriteString(" <")
		b.WriteString(u.Person.Email)
		b.WriteString(">")
	}
	return b.String()
}

type Date struct {
	time.Time
}

func (d *Date) UnmarshalJSON(data []byte) error {
	t, err := time.Parse("\"2006-01-02\"", string(data))
	if err != nil {
		return err
	}
	d.Time = t
	return nil
}

func (d Date) MarshalJSON() ([]byte, error) {
	return []byte("\"" + d.Time.Format("2006-01-02") + "\""), nil
}

type Person struct {
	// Email address of people
	Email string `json:"email"`
}

type Bot struct{}

type UserList struct {
	*ListMeta
	Results []*User `json:"results"`
}

type RichTextObjectType string

const (
	RichTextObjectTypeText     RichTextObjectType = "text"
	RichTextObjectTypeMention  RichTextObjectType = "mention"
	RichTextObjectTypeEquation RichTextObjectType = "equation"
)

type RichTextObject struct {
	Type        RichTextObjectType `json:"type,omitempty"`
	PlainText   string             `json:"plain_text,omitempty"`
	Href        string             `json:"href,omitempty"`
	Annotations *TextAnnotation    `json:"annotations,omitempty"`

	Text     *Text     `json:"text,omitempty"`
	Mention  *Mention  `json:"mention,omitempty"`
	Equation *Equation `json:"equation,omitempty"`
}

type TextAnnotation struct {
	Bold          bool   `json:"bold"`
	Italic        bool   `json:"italic"`
	Strikethrough bool   `json:"strikethrough"`
	Underline     bool   `json:"underline"`
	Code          bool   `json:"code"`
	Color         string `json:"color"`
}

type Text struct {
	Content string `json:"content"`
	Link    *Link  `json:"link"`
}

type MentionType string

const (
	MentionTypeUser        MentionType = "user"
	MentionTypePage        MentionType = "page"
	MentionTypeDatabase    MentionType = "database"
	MentionTypeDate        MentionType = "date"
	MentionTypeLinkPreview MentionType = "link_preview"
)

type Mention struct {
	Type MentionType `json:"type"`

	User     *User `json:"user,omitempty"`
	Page     *Meta `json:"page,omitempty"`
	Database *Meta `json:"database,omitempty"`
}

type Equation struct {
	Expression string `json:"expression"`
}

type Link struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type Database struct {
	*Meta

	Parent         *PageParent                  `json:"parent,omitempty"`
	CreatedTime    Time                         `json:"created_time,omitempty"`
	LastEditedTime Time                         `json:"last_edited_time,omitempty"`
	Title          []*RichTextObject            `json:"title"`
	Properties     map[string]*PropertyMetadata `json:"properties"`
	URL            string                       `json:"url,omitempty"`
}

func (d *Database) decode() error {
	for _, v := range d.Properties {
		id, err := url.QueryUnescape(v.ID)
		if err != nil {
			return err
		}
		v.ID = id
	}

	return nil
}

type DatabaseList struct {
	*ListMeta
	Results []*Database `json:"results"`
}

type PropertyMetadata struct {
	ID   string       `json:"id,omitempty"`
	Type PropertyType `json:"type,omitempty"`
	Name string       `json:"name,omitempty"`

	Title          *RichTextObject      `json:"title,omitempty"`
	RichText       *struct{}            `json:"rich_text,omitempty"`
	Number         *NumberProperty      `json:"number,omitempty"`
	Select         *SelectProperty      `json:"select,omitempty"`
	MultiSelect    *MultiSelectProperty `json:"multi_select,omitempty"`
	Date           *struct{}            `json:"date,omitempty"`
	Formula        *struct{}            `json:"formula,omitempty"`
	Relation       *struct{}            `json:"relation,omitempty"`
	Checkbox       *struct{}            `json:"checkbox,omitempty"`
	Rollup         *RollupProperty      `json:"rollup,omitempty"`
	People         *struct{}            `json:"people,omitempty"`
	Files          *struct{}            `json:"files,omitempty"`
	URL            *struct{}            `json:"url,omitempty"`
	Email          *struct{}            `json:"email,omitempty"`
	PhoneNumber    *struct{}            `json:"phone_number,omitempty"`
	CreatedTime    *struct{}            `json:"created_time,omitempty"`
	CreatedBy      *struct{}            `json:"created_by,omitempty"`
	LastEditedTime *struct{}            `json:"last_edited_time,omitempty"`
	LastEditedBy   *struct{}            `json:"last_edited_by,omitempty"`
}

func (p *PropertyMetadata) String() string {
	b := new(strings.Builder)
	b.WriteString(string(p.Type))
	b.WriteString(": ")

	switch p.Type {
	case PropertyTypeTitle:
		b.WriteString(fmt.Sprintf("%v", p.Title))
	case PropertyTypeNumber:
		b.WriteString(fmt.Sprintf("%v", p.Number))
	case PropertyTypeSelect:
		b.WriteString(fmt.Sprintf("%v", p.Select))
	}

	return b.String()
}

type NumberProperty struct {
	Format string `json:"format"`
}

type SelectProperty struct {
	Options []*Option `json:"options"`
}

type MultiSelectProperty struct {
	Options []*Option `json:"options"`
}

type Option struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Color string `json:"color,omitempty"`
}

type DateProperty struct {
	Start *Date `json:"start,omitempty"`
	End   *Date `json:"end,omitempty"`
}

type FormulaType string

const (
	FormulaTypeString  FormulaType = "string"
	FormulaTypeNumber  FormulaType = "number"
	FormulaTypeBoolean FormulaType = "boolean"
	FormulaTypeDate    FormulaType = "date"
)

type FormulaProperty struct {
	Type FormulaType `json:"type"`

	String  string        `json:"string,omitempty"`
	Number  int           `json:"number,omitempty"`
	Boolean bool          `json:"boolean,omitempty"`
	Date    *DateProperty `json:"date,omitempty"`
}

type RollupFunction string

const (
	RollupFunctionCountAll          RollupFunction = "count_all"
	RollupFunctionCountValues       RollupFunction = "count_values"
	RollupFunctionCountUniqueValues RollupFunction = "count_unique_values"
	RollupFunctionCountEmpty        RollupFunction = "count_empty"
	RollupFunctionCountNotEmpty     RollupFunction = "count_not_empty"
	RollupFunctionPercentEmpty      RollupFunction = "percent_empty"
	RollupFunctionPercentNotEmpty   RollupFunction = "percent_not_empty"
	RollupFunctionSum               RollupFunction = "sum"
	RollupFunctionAverage           RollupFunction = "average"
	RollupFunctionMedian            RollupFunction = "median"
	RollupFunctionMin               RollupFunction = "min"
	RollupFunctionMax               RollupFunction = "max"
	RollupFunctionRange             RollupFunction = "range"
	RollupFunctionShowOriginal      RollupFunction = "show_original"
)

type RollupProperty struct {
	Name               string         `json:"rollup_property_name"`
	Relation           string         `json:"relation_property_name"`
	RollupPropertyID   string         `json:"rollup_property_id"`
	RelationPropertyID string         `json:"relation_property_id"`
	Function           RollupFunction `json:"function"`
}

type RollupType string

const (
	RollupTypeNumber RollupType = "number"
	RollupTypeDate   RollupType = "date"
	RollupTypeArray  RollupType = "array"
)

type Rollup struct {
	Type RollupType `json:"type"`

	Number *NumberProperty `json:"number,omitempty"`
	Date   *DateProperty   `json:"date,omitempty"`
	Array  []*PropertyData `json:"array,omitempty"`
}

// Page is a page object.
// ref: https://developers.notion.com/reference/page
type Page struct {
	*Meta

	CreatedTime    *Time                    `json:"created_time,omitempty"`
	LastEditedTime *Time                    `json:"last_edited_time,omitempty"`
	Archived       bool                     `json:"archived,omitempty"`
	Parent         *PageParent              `json:"parent,omitempty"`
	Properties     map[string]*PropertyData `json:"properties"`
	Children       []*Block                 `json:"children,omitempty"`
	URL            string                   `json:"url,omitempty"`
}

func (p *Page) decode() error {
	for _, v := range p.Properties {
		id, err := url.PathUnescape(v.ID)
		if err != nil {
			return err
		}
		v.ID = id
	}

	return nil
}

type PageList struct {
	*ListMeta
	Results []*Page `json:"results"`
}

type PageParent struct {
	Type       ObjectType `json:"type,omitempty"`
	DatabaseID string     `json:"database_id,omitempty"`
	PageID     string     `json:"page_id,omitempty"`

	Database *Database `json:"-"`
	Page     *Page     `json:"-"`
}

type PropertyType string

const (
	PropertyTypeTitle          PropertyType = "title"
	PropertyTypeText           PropertyType = "text"
	PropertyTypeRichText       PropertyType = "rich_text"
	PropertyTypeNumber         PropertyType = "number"
	PropertyTypeSelect         PropertyType = "select"
	PropertyTypeMultiSelect    PropertyType = "multi_select"
	PropertyTypeDate           PropertyType = "date"
	PropertyTypePeople         PropertyType = "people"
	PropertyTypeFiles          PropertyType = "files"
	PropertyTypeCheckbox       PropertyType = "checkbox"
	PropertyTypeURL            PropertyType = "url"
	PropertyTypeEmail          PropertyType = "email"
	PropertyTypePhoneNumber    PropertyType = "phone_number"
	PropertyTypeFormula        PropertyType = "formula"
	PropertyTypeRelation       PropertyType = "relation"
	PropertyTypeRollup         PropertyType = "rollup"
	PropertyTypeCreatedTime    PropertyType = "created_time"
	PropertyTypeCreatedBy      PropertyType = "created_by"
	PropertyTypeLastEditedTime PropertyType = "last_edited_time"
	PropertyTypeLastEditedBy   PropertyType = "last_edited_by"
)

type PropertyData struct {
	ID   string       `json:"id,omitempty"`
	Type PropertyType `json:"type"`

	Title          []*RichTextObject `json:"title,omitempty"`
	MultiSelect    []*Option         `json:"multi_select,omitempty"`
	Text           []*RichTextObject `json:"text,omitempty"`
	RichText       []*RichTextObject `json:"rich_text,omitempty"`
	Number         *int              `json:"number,omitempty"`
	Select         *Option           `json:"select,omitempty"`
	Date           *DateProperty     `json:"date,omitempty"`
	People         []*User           `json:"people,omitempty"`
	Files          []*File           `json:"files,omitempty"`
	Checkbox       bool              `json:"checkbox,omitempty"`
	URL            string            `json:"url,omitempty"`
	Email          string            `json:"email,omitempty"`
	PhoneNumber    string            `json:"phone_number,omitempty"`
	Formula        *FormulaProperty  `json:"formula,omitempty"`
	Relation       []*Meta           `json:"relation,omitempty"`
	RollupProperty *Rollup           `json:"rollup,omitempty"`
	CreatedTime    *Time             `json:"created_time,omitempty"`
	CreatedBy      *User             `json:"created_by,omitempty"`
	LastEditedTime *Time             `json:"last_edited_time,omitempty"`
	LastEditedBy   *User             `json:"last_edited_by,omitempty"`
}

// TODO: Support formula, relation and rollup
func (d *PropertyData) String() string {
	switch d.Type {
	case PropertyTypeTitle:
		if len(d.Title) == 0 {
			return ""
		}
		return d.Title[0].PlainText
	case PropertyTypeMultiSelect:
		var b strings.Builder
		for i, v := range d.MultiSelect {
			if i != 0 {
				b.WriteString(", ")
			}
			b.WriteString(v.Name)
		}
		return b.String()
	case PropertyTypeText:
		var b strings.Builder
		for _, v := range d.Text {
			b.WriteString(v.PlainText)
		}
		return b.String()
	case PropertyTypeRichText:
		var b strings.Builder
		for _, v := range d.RichText {
			b.WriteString(v.PlainText)
		}
		return b.String()
	case PropertyTypeNumber:
		return fmt.Sprintf("%d", d.Number)
	case PropertyTypeSelect:
		if d.Select == nil {
			return ""
		}
		return d.Select.Name
	case PropertyTypeDate:
		if d.Date.Start == nil {
			return ""
		}
		return d.Date.Start.Format("2006-01-02")
	case PropertyTypePeople:
		var b strings.Builder
		for i, p := range d.People {
			if i != 0 {
				b.WriteString(", ")
			}
			b.WriteString(p.String())
		}
		return b.String()
	case PropertyTypeFiles:
		var b strings.Builder
		for i, f := range d.Files {
			if i != 0 {
				b.WriteString(", ")
			}
			b.WriteString(f.Name)
		}
		return b.String()
	case PropertyTypeCheckbox:
		if d.Checkbox {
			return "checked"
		} else {
			return "not checked"
		}
	case PropertyTypeURL:
		return d.URL
	case PropertyTypePhoneNumber:
		return d.PhoneNumber
	case PropertyTypeCreatedTime:
		if d.CreatedTime == nil {
			return ""
		}
		return d.CreatedTime.Format(time.RFC3339)
	case PropertyTypeCreatedBy:
		return d.CreatedBy.String()
	case PropertyTypeLastEditedTime:
		if d.LastEditedTime == nil {
			return ""
		}
		return d.LastEditedTime.Format(time.RFC3339)
	case PropertyTypeLastEditedBy:
		return d.LastEditedBy.String()
	}

	return ""
}

type File struct {
	Name string `json:"name"`
}

type Filter struct {
	// Compound filter
	Or  []*Filter `json:"or,omitempty"`
	And []*Filter `json:"and,omitempty"`

	// Database property filter
	Property    string             `json:"property,omitempty"`
	Text        *TextFilter        `json:"text,omitempty"`
	Number      *NumberFilter      `json:"number,omitempty"`
	Checkbox    *CheckboxFilter    `json:"checkbox,omitempty"`
	Select      *SelectFilter      `json:"select,omitempty"`
	MultiSelect *MultiSelectFilter `json:"multi_select,omitempty"`
	Date        *DateFilter        `json:"date,omitempty"`
	People      *PeopleFilter      `json:"people,omitempty"`
	Files       *FilesFilter       `json:"files,omitempty"`
	Relation    *RelationFilter    `json:"relation,omitempty"`
	Formula     *FormulaFilter     `json:"formula,omitempty"`
}

type TextFilter struct {
	Equals         string `json:"equals,omitempty"`
	DoesNotEqual   string `json:"does_not_equal,omitempty"`
	Contains       string `json:"contains,omitempty"`
	DoesNotContain string `json:"does_not_contain,omitempty"`
	StartsWith     string `json:"starts_with,omitempty"`
	EndsWith       string `json:"ends_with,omitempty"`
	IsEmpty        bool   `json:"is_empty,omitempty"`
	IsNotEmpty     bool   `json:"is_not_empty,omitempty"`
}

type NumberFilter struct {
	Equals               int  `json:"equals,omitempty"`
	DoesNotEqual         int  `json:"does_not_equal,omitempty"`
	GreaterThan          int  `json:"greater_than,omitempty"`
	LessThan             int  `json:"less_than,omitempty"`
	GreaterThanOrEqualTo int  `json:"greater_than_or_equal_to,omitempty"`
	LessThanOrEqualTo    int  `json:"less_than_or_equal_to,omitempty"`
	IsEmpty              bool `json:"is_empty,omitempty"`
	IsNotEmpty           bool `json:"is_not_empty,omitempty"`
}

type CheckboxFilter struct {
	Equals       bool `json:"equals,omitempty"`
	DoesNotEqual bool `json:"does_not_equal,omitempty"`
}

type SelectFilter struct {
	Equals       string `json:"equals,omitempty"`
	DoesNotEqual string `json:"does_not_equal,omitempty"`
	IsEmpty      bool   `json:"is_empty,omitempty"`
	IsNotEmpty   bool   `json:"is_not_empty,omitempty"`
}

type MultiSelectFilter struct {
	Contains       string `json:"contains,omitempty"`
	DoesNotContain string `json:"does_not_contain,omitempty"`
	IsEmpty        bool   `json:"is_empty,omitempty"`
	IsNotEmpty     bool   `json:"is_not_empty,omitempty"`
}

type DateFilter struct {
	Equals     *Time     `json:"equals,omitempty"`
	Before     *Time     `json:"before,omitempty"`
	After      *Time     `json:"after,omitempty"`
	OnOrBefore *Time     `json:"on_or_before,omitempty"`
	OnOrAfter  *Time     `json:"on_or_after,omitempty"`
	IsEmpty    bool      `json:"is_empty,omitempty"`
	IsNotEmpty bool      `json:"is_not_empty,omitempty"`
	PastWeek   *struct{} `json:"past_week,omitempty"`
	PastMonth  *struct{} `json:"past_month,omitempty"`
	PastYear   *struct{} `json:"past_year,omitempty"`
	NextWeek   *struct{} `json:"next_week,omitempty"`
	NextMonth  *struct{} `json:"next_month,omitempty"`
	NextYear   *struct{} `json:"next_year,omitempty"`
}

type PeopleFilter struct {
	Contains       string `json:"contains,omitempty"`
	DoesNotContain string `json:"does_not_contain,omitempty"`
	IsEmpty        bool   `json:"is_empty,omitempty"`
	IsNotEmpty     bool   `json:"is_not_empty,omitempty"`
}

type FilesFilter struct {
	IsEmpty    bool `json:"is_empty,omitempty"`
	IsNotEmpty bool `json:"is_not_empty,omitempty"`
}

type RelationFilter struct {
	Contains       string `json:"contains,omitempty"`
	DoesNotContain string `json:"does_not_contain,omitempty"`
	IsEmpty        bool   `json:"is_empty,omitempty"`
	IsNotEmpty     bool   `json:"is_not_empty,omitempty"`
}

type FormulaFilter struct {
	Text     *TextFilter     `json:"text,omitempty"`
	Checkbox *CheckboxFilter `json:"checkbox,omitempty"`
	Number   *NumberFilter   `json:"number,omitempty"`
	Date     *DateFilter     `json:"date,omitempty"`
}

type Sort struct {
	Property  string `json:"property"`
	Timestamp string `json:"timestamp"`
	Direction string `json:"direction"`
}

type BlockType string

const (
	BlockTypeParagraph        BlockType = "paragraph"
	BlockTypeHeading1         BlockType = "heading_1"
	BlockTypeHeading2         BlockType = "heading_2"
	BlockTypeHeading3         BlockType = "heading_3"
	BlockTypeCallOut          BlockType = "callout"
	BlockTypeQuote            BlockType = "quote"
	BlockTypeBulletedListItem BlockType = "bulleted_list_item"
	BlockTypeNumberedListItem BlockType = "numbered_list_item"
	BlockTypeToDo             BlockType = "to_do"
	BlockTypeToggle           BlockType = "toggle"
	BlockTypeCode             BlockType = "code"
	BlockTypeChildPage        BlockType = "child_page"
	BlockTypeChildDatabase    BlockType = "child_database"
	BlockTypeEmbed            BlockType = "embed"
	BlockTypeImage            BlockType = "image"
	BlockTypeVideo            BlockType = "video"
	BlockTypeFile             BlockType = "file"
	BlockTypePDF              BlockType = "pdf"
	BlockTypeBookmark         BlockType = "bookmark"
	BlockTypeEquation         BlockType = "equation"
	BlockTypeDivider          BlockType = "divider"
	BlockTypeTableOfContent   BlockType = "table_of_contents"
	BlockTypeBreadcrumb       BlockType = "breadcrumb"
	BlockTypeColumnList       BlockType = "column_list"
	BlockTypeColumn           BlockType = "column"
	BlockTypeLinkPreview      BlockType = "link_preview"
	BlockTypeLinkToPage       BlockType = "link_to_page"
	BlockTypeSynced           BlockType = "synced_block"
)

// Block is a block object.
// ref: https://developers.notion.com/reference/block
type Block struct {
	*Meta

	CreatedTime    Time      `json:"created_time,omitempty"`
	LastEditedTime Time      `json:"last_edited_time,omitempty"`
	HasChildren    bool      `json:"has_children"`
	Archived       bool      `json:"archived"`
	Type           BlockType `json:"type"`

	Paragraph        *Paragraph `json:"paragraph,omitempty"`
	Heading1         *Heading   `json:"heading_1,omitempty"`
	Heading2         *Heading   `json:"heading_2,omitempty"`
	Heading3         *Heading   `json:"heading_3,omitempty"`
	BulletedListItem *Paragraph `json:"bulleted_list_item,omitempty"`
	NumberedListItem *Paragraph `json:"numbered_list_item,omitempty"`
	ToDo             *ToDo      `json:"to_do,omitempty"`
	Toggle           *Paragraph `json:"toggle,omitempty"`
	ChildPage        *ChildPage `json:"child_page,omitempty"`
	Divider          *struct{}  `json:"divider,omitempty"`
	Code             *Code      `json:"code,omitempty"`
	ColumnList       *struct{}  `json:"column_list,omitempty"`
	Breadcrumb       *struct{}  `json:"breadcrumb,omitempty"`
	TableOfContents  *struct{}  `json:"table_of_contents,omitempty"`
}

type BlockList struct {
	*ListMeta
	Results []*Block `json:"results"`
}

type Paragraph struct {
	Text     []*RichTextObject `json:"text,omitempty"`
	Children []*Block          `json:"children,omitempty"`
}

type Heading struct {
	Text []*RichTextObject `json:"text"`
}

type ToDo struct {
	Text     []*RichTextObject `json:"text"`
	Checked  bool              `json:"checked"`
	Children []*RichTextObject `json:"children"`
}

type ChildPage struct {
	Title string `json:"title"`
}

type Code struct {
	Text     []*RichTextObject `json:"text"`
	Language string            `json:"language"`
}

type SearchResult struct {
	*ListMeta
	Results []*json.RawMessage `json:"results"`
}

type Error struct {
	*Meta
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

var _ error = &Error{}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
