package ui

import (
	"context"
	"encoding/json"
	"fmt"

	h "github.com/theplant/htmlgo"
)

type VueEventTagBuilder struct {
	tag           h.MutableAttrHTMLComponent
	fieldName     *string
	onInputFuncID *EventFuncID
}

func Bind(b h.MutableAttrHTMLComponent) (r *VueEventTagBuilder) {
	r = &VueEventTagBuilder{}
	r.tag = b
	return
}

func (b *VueEventTagBuilder) OnInput(eventFuncId string, params ...string) (r *VueEventTagBuilder) {

	b.onInputFuncID = &EventFuncID{
		ID:     eventFuncId,
		Params: params,
	}

	return b
}

func (b *VueEventTagBuilder) OnClick(eventFuncId string, params ...string) (r *VueEventTagBuilder) {

	fid := &EventFuncID{
		ID:     eventFuncId,
		Params: params,
	}

	jb, err := json.Marshal(fid)
	if err != nil {
		panic(err)
	}

	b.tag.SetAttr("v-on:click", fmt.Sprintf("onclick(%s, $event)", string(jb)))
	return b
}

func (b *VueEventTagBuilder) FieldName(v string) (r *VueEventTagBuilder) {
	b.fieldName = &v
	return b
}

func (b *VueEventTagBuilder) setupChange() {
	if b.fieldName == nil && b.onInputFuncID == nil {
		return
	}

	jb, err := json.Marshal(b.onInputFuncID)
	if err != nil {
		panic(err)
	}

	fieldName, err := json.Marshal(b.fieldName)
	if err != nil {
		panic(err)
	}

	b.tag.SetAttr("v-on:input", fmt.Sprintf(`oninput(%s, %s, $event)`, string(jb), string(fieldName)))
}

func (b *VueEventTagBuilder) MarshalHTML(ctx context.Context) (r []byte, err error) {
	b.setupChange()
	return b.tag.MarshalHTML(ctx)
}
