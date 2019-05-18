package branoverlay

import (
	"context"
	"fmt"

	h "github.com/theplant/htmlgo"
)

type DialogBuilder struct {
	children []h.HTMLComponent

	trigger h.MutableAttrHTMLComponent
	tag     *h.HTMLTagBuilder
}

func Dialog(children ...h.HTMLComponent) (r *DialogBuilder) {
	r = &DialogBuilder{
		tag: h.Tag("bran-dialog"),
	}
	r.Animation("zoom")
	r.children = children
	return
}

func (b *DialogBuilder) Trigger(v h.MutableAttrHTMLComponent) (r *DialogBuilder) {
	b.trigger = v
	return b
}

func (b *DialogBuilder) Width(v int) (r *DialogBuilder) {
	b.tag.Attr(":width", fmt.Sprint(v))
	return b
}

func (b *DialogBuilder) Height(v int) (r *DialogBuilder) {
	b.tag.Attr(":height", fmt.Sprint(v))
	return b
}

func (b *DialogBuilder) DefaultVisible(v bool) (r *DialogBuilder) {
	b.tag.Attr(":visible", fmt.Sprint(v))
	return b
}

func (b *DialogBuilder) Closable(v bool) (r *DialogBuilder) {
	b.tag.Attr(":closable", fmt.Sprint(v))
	return b
}

func (b *DialogBuilder) MaskClosable(v bool) (r *DialogBuilder) {
	b.tag.Attr(":mask-closable", fmt.Sprint(v))
	return b
}

func (b *DialogBuilder) Keyboard(v bool) (r *DialogBuilder) {
	b.tag.Attr("keyboard", fmt.Sprint(v))
	return b
}

func (b *DialogBuilder) WrapClassName(v string) (r *DialogBuilder) {
	b.tag.Attr("wrap-class-name", v)
	return b
}

func (b *DialogBuilder) DialogClass(v string) (r *DialogBuilder) {
	b.tag.Attr("dialog-class", v)
	return b
}

func (b *DialogBuilder) Animation(v string) (r *DialogBuilder) {
	b.tag.Attr("animation", v)
	return b
}

func (b *DialogBuilder) PrefixClass(v string) (r *DialogBuilder) {
	b.tag.Attr("prefix-cls", v)
	return b
}

func (b *DialogBuilder) ZIndex(v int) (r *DialogBuilder) {
	b.tag.Attr(":z-index", fmt.Sprint(v))
	return b
}

func (b *DialogBuilder) MarshalHTML(ctx context.Context) (r []byte, err error) {

	if b.trigger != nil {
		b.trigger.SetAttr("@click", "parent.show")
	}

	b.tag.Children(
		h.If(b.trigger != nil, h.Template(b.trigger).Attr("v-slot:trigger", "{ parent }")),
		h.Template(b.children...).Attr("v-slot:dialog", "{ parent }"),
	)

	return b.tag.MarshalHTML(ctx)
}
