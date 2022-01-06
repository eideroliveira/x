package presets

import (
	"net/http"
	"net/url"

	"github.com/goplaid/web"
	"github.com/goplaid/x/vuetifyx"
	h "github.com/theplant/htmlgo"
)

type ComponentFunc func(ctx *web.EventContext) h.HTMLComponent
type ObjectComponentFunc func(obj interface{}, ctx *web.EventContext) h.HTMLComponent

type FieldComponentFunc func(obj interface{}, field *FieldContext, ctx *web.EventContext) h.HTMLComponent

type ActionComponentFunc func(selectedIds []string, ctx *web.EventContext) h.HTMLComponent
type ActionUpdateFunc func(selectedIds []string, ctx *web.EventContext) (err error)

type MessagesFunc func(r *http.Request) *Messages

// Data Layer
type DataOperator interface {
	Search(obj interface{}, params *SearchParams, ctx *web.EventContext) (r interface{}, totalCount int, err error)
	Fetch(obj interface{}, id string, ctx *web.EventContext) (r interface{}, err error)
	Save(obj interface{}, id string, ctx *web.EventContext) (err error)
	Delete(obj interface{}, id string, ctx *web.EventContext) (err error)
}

type SetterFunc func(obj interface{}, ctx *web.EventContext)
type FieldSetterFunc func(obj interface{}, field *FieldContext, ctx *web.EventContext) (err error)
type ValidateFunc func(obj interface{}, ctx *web.EventContext) (err web.ValidationErrors)

type SearchFunc func(model interface{}, params *SearchParams, ctx *web.EventContext) (r interface{}, totalCount int, err error)
type FetchFunc func(obj interface{}, id string, ctx *web.EventContext) (r interface{}, err error)
type SaveFunc func(obj interface{}, id string, ctx *web.EventContext) (err error)
type DeleteFunc func(obj interface{}, id string, ctx *web.EventContext) (err error)

type FilterDataFunc func(ctx *web.EventContext) vuetifyx.FilterData

type FilterTab struct {
	Label string
	// render AdvancedLabel if it is not nil
	AdvancedLabel h.HTMLComponent
	Query         url.Values
}

type FilterTabsFunc func(ctx *web.EventContext) []*FilterTab

type SQLCondition struct {
	Query string
	Args  []interface{}
}

type SearchParams struct {
	KeywordColumns []string
	Keyword        string
	SQLConditions  []*SQLCondition
	PerPage        int64
	Page           int64
	OrderBy        string
}
