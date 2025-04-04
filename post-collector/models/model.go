package models

import "github.com/shopspring/decimal"

type WidgetList struct {
	ListWidgets []*Widget `json:"list_widgets"`
}

type Widget struct {
	Data       WidgetData `json:"data"`
	WidgetType string     `json:"widget_type"`
}

type WidgetData struct {
	Title                 string
	Value                 string
	ImageUrl              string            `json:"image_url"`
	TopDescriptionText    string            `json:"top_description_text"`
	MiddleDescriptionText string            `json:"middle_description_text"`
	BottomDescriptionText string            `json:"bottom_description_text"`
	Token                 string            `json:"token"`
	Items                 []*WidgetDataItem `json:"items"`
	Credit                *ValueAndTransformedValue
	Rent                  *ValueAndTransformedValue
}

type ValueAndTransformedValue struct {
	Value            string `json:"value"`
	TransformedValue string `json:"transformed_value"`
}

type WidgetDataItem struct {
	Title     string
	Available bool
	Value     string
}

type Post struct {
	Token                 string `gorm:"primaryKey"`
	Title                 string
	TopDescriptionText    string
	MiddleDescriptionText string
	BottomDescriptionText string
}

type Section struct {
	SectionName string    `json:"section_name"`
	Widgets     []*Widget `json:"widgets"`
}

type PostDetailResponse struct {
	Sections []*Section `json:"sections"`
}

type PostDetail struct {
	Token            string `gorm:"primaryKey"`
	Title            string
	Region           string
	Meterage         decimal.Decimal
	Mortage          decimal.Decimal
	Rent             decimal.Decimal
	Rooms            int
	ConstructionYear int
	Elevator         bool
	Parking          bool
	Warehouse        bool
}

type SearchData struct {
	FormData FormData `json:"form_data"`
}

type FormData struct {
	Data Data `json:"data"`
}

type Data struct {
	Category Category `json:"category"`
}

type Category struct {
	Str Str `json:"str"`
}

type Str struct {
	Value string `json:"value"`
}
