package vuetifyx

import (
	"context"

	"github.com/qor5/x/v3/ui/vuetify"
	h "github.com/theplant/htmlgo"
)

type VXAutocompleteBuilder struct {
	tag           *h.HTMLTagBuilder
	selectedItems interface{}
	items         interface{}
}

func VXAutocomplete(children ...h.HTMLComponent) (r *VXAutocompleteBuilder) {
	r = &VXAutocompleteBuilder{
		tag: h.Tag("vx-autocomplete").Children(children...),
	}
	r.Multiple(true).Chips(true).DeletableChips(true).Clearable(true)

	return
}

func (b *VXAutocompleteBuilder) ErrorMessages(v ...string) (r *VXAutocompleteBuilder) {
	vuetify.SetErrorMessages(b.tag, v)
	return b
}

func (b *VXAutocompleteBuilder) SelectedItems(v interface{}) (r *VXAutocompleteBuilder) {
	b.selectedItems = v
	return b
}

func (b *VXAutocompleteBuilder) HasIcon(v bool) (r *VXAutocompleteBuilder) {
	b.tag.Attr("has-icon", v)
	return b
}

func (b *VXAutocompleteBuilder) Sorting(v bool) (r *VXAutocompleteBuilder) {
	b.tag.Attr("sorting", v)
	return b
}

func (b *VXAutocompleteBuilder) Variant(v string) (r *VXAutocompleteBuilder) {
	b.tag.Attr("variant", v)
	return b
}

func (b *VXAutocompleteBuilder) Density(v string) (r *VXAutocompleteBuilder) {
	b.tag.Attr("density", v)
	return b
}

func (b *VXAutocompleteBuilder) Items(v interface{}) (r *VXAutocompleteBuilder) {
	b.items = v
	return b
}

func (b *VXAutocompleteBuilder) ChipColor(v string) (r *VXAutocompleteBuilder) {
	b.tag.Attr("chip-color", v)
	return b
}

func (b *VXAutocompleteBuilder) ChipTextColor(v string) (r *VXAutocompleteBuilder) {
	b.tag.Attr("chip-text-color", v)
	return b
}

func (b *VXAutocompleteBuilder) PageKey(v string) (r *VXAutocompleteBuilder) {
	b.tag.Attr("page-key", v)
	return b
}
func (b *VXAutocompleteBuilder) PagesKey(v string) (r *VXAutocompleteBuilder) {
	b.tag.Attr("pages-key", v)
	return b
}
func (b *VXAutocompleteBuilder) PageSizeKey(v string) (r *VXAutocompleteBuilder) {
	b.tag.Attr("page-size-key", v)
	return b
}
func (b *VXAutocompleteBuilder) TotalKey(v string) (r *VXAutocompleteBuilder) {
	b.tag.Attr("total-key", v)
	return b
}
func (b *VXAutocompleteBuilder) ItemsKey(v string) (r *VXAutocompleteBuilder) {
	b.tag.Attr("items-key", v)
	return b
}
func (b *VXAutocompleteBuilder) CurrentKey(v string) (r *VXAutocompleteBuilder) {
	b.tag.Attr("current-key", v)
	return b
}
func (b *VXAutocompleteBuilder) SearchKey(v string) (r *VXAutocompleteBuilder) {
	b.tag.Attr("search-key", v)
	return b
}
func (b *VXAutocompleteBuilder) Page(v int) (r *VXAutocompleteBuilder) {
	b.tag.Attr("page", v)
	return b
}

func (b *VXAutocompleteBuilder) PageSize(v int) (r *VXAutocompleteBuilder) {
	b.tag.Attr("page-size", v)
	return b
}

func (b *VXAutocompleteBuilder) SetDataSource(ds *AutocompleteDataSource) (r *VXAutocompleteBuilder) {
	b.tag.Attr("remote-url", ds.RemoteURL)
	b.tag.Attr("is-paging", ds.IsPaging)
	b.tag.Attr("has-icon", ds.HasIcon)
	return b
}

func (b *VXAutocompleteBuilder) MarshalHTML(ctx context.Context) (r []byte, err error) {
	if b.items == nil {
		b.items = b.selectedItems
	}
	b.tag.Attr(":items", b.items)
	b.tag.Attr(":selected-items", b.selectedItems)
	return b.tag.MarshalHTML(ctx)
}

type AutocompleteDataSource struct {
	RemoteURL   string `json:"remoteUrl"`
	IsPaging    bool   `json:"isPaging"`
	HasIcon     bool   `json:"hasIcon"`
	ItemTitle   string `json:"itemTitle"`
	ItemValue   string `json:"itemValue"`
	ItemIcon    string `json:"itemIcon"`
	PageKey     string `json:"pageKey"`
	PagesKey    string `json:"pagesKey"`
	PageSizeKey string `json:"pageSizeKey"`
	TotalKey    string `json:"totalKey"`
	ItemsKey    string `json:"itemsKey"`
	CurrentKey  string `json:"currentKey"`
	SearchKey   string `json:"searchKey"`
	Page        int    `json:"page"`
	PageSize    int    `json:"pageSize"`
}
