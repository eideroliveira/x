package presets

import (
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/goplaid/web"
	"github.com/goplaid/x/presets/actions"
	. "github.com/goplaid/x/vuetify"
	"github.com/goplaid/x/vuetifyx"
	"github.com/iancoleman/strcase"
	"github.com/sunfmin/reflectutils"
	h "github.com/theplant/htmlgo"
)

type FieldDefaultBuilder struct {
	valType    reflect.Type
	compFunc   FieldComponentFunc
	setterFunc FieldSetterFunc
}

type FieldMode int

const (
	WRITE FieldMode = iota
	LIST
	DETAIL
)

func NewFieldDefault(t reflect.Type) (r *FieldDefaultBuilder) {
	r = &FieldDefaultBuilder{valType: t}
	return
}

func (b *FieldDefaultBuilder) ComponentFunc(v FieldComponentFunc) (r *FieldDefaultBuilder) {
	b.compFunc = v
	return b
}

func (b *FieldDefaultBuilder) SetterFunc(v FieldSetterFunc) (r *FieldDefaultBuilder) {
	b.setterFunc = v
	return b
}

var numberVals = []interface{}{
	int(0), int8(0), int16(0), int32(0), int64(0),
	uint(0), uint(8), uint16(0), uint32(0), uint64(0),
	float32(0.0), float64(0.0),
}

var stringVals = []interface{}{
	string(""),
	[]rune(""),
	[]byte(""),
}

type FieldDefaults struct {
	mode             FieldMode
	fieldTypes       []*FieldDefaultBuilder
	excludesPatterns []string
}

func NewFieldDefaults(t FieldMode) (r *FieldDefaults) {
	r = &FieldDefaults{
		mode: t,
	}
	r.builtInFieldTypes()
	return
}

func (b *FieldDefaults) FieldType(v interface{}) (r *FieldDefaultBuilder) {
	return b.fieldTypeByTypeOrCreate(reflect.TypeOf(v))
}

func (b *FieldDefaults) Exclude(patterns ...string) (r *FieldDefaults) {
	b.excludesPatterns = patterns
	return b
}

func (b *FieldDefaults) InspectFields(val interface{}) (r *FieldsBuilder) {
	r, _ = b.inspectFieldsAndCollectName(val, nil)
	r.Model(val)
	return
}

func (b *FieldDefaults) inspectFieldsAndCollectName(val interface{}, collectType reflect.Type) (r *FieldsBuilder, names []string) {
	v := reflect.ValueOf(val)

	for v.Elem().Kind() == reflect.Ptr {
		v = v.Elem()
	}
	v = v.Elem()

	t := v.Type()

	r = &FieldsBuilder{
		model: val,
	}
	r.Defaults(b)

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		ft := b.fieldTypeByType(f.Type)

		if !hasMatched(b.excludesPatterns, f.Name) && ft != nil {
			r.Field(f.Name).
				ComponentFunc(ft.compFunc).
				SetterFunc(ft.setterFunc)
		}

		if collectType != nil && f.Type == collectType {
			names = append(names, strcase.ToSnake(f.Name))
		}
	}

	return
}

func hasMatched(patterns []string, name string) bool {
	for _, p := range patterns {
		ok, err := filepath.Match(p, name)
		if err != nil {
			panic(err)
		}
		if ok {
			return true
		}
	}
	return false
}

func (b *FieldDefaults) String() string {
	var types []string
	for _, t := range b.fieldTypes {
		types = append(types, fmt.Sprint(t.valType))
	}
	return fmt.Sprintf("mode: %d, types %v", b.mode, types)
}

func (b *FieldDefaults) fieldTypeByType(tv reflect.Type) (r *FieldDefaultBuilder) {
	for _, ft := range b.fieldTypes {
		if ft.valType == tv {
			return ft
		}
	}
	return nil
}

func (b *FieldDefaults) fieldTypeByTypeOrCreate(tv reflect.Type) (r *FieldDefaultBuilder) {
	if r = b.fieldTypeByType(tv); r != nil {
		return
	}

	r = NewFieldDefault(tv)

	if b.mode == LIST {
		r.ComponentFunc(cfTextTd)
	} else {
		r.ComponentFunc(cfTextField)
	}
	b.fieldTypes = append(b.fieldTypes, r)
	return
}

func cfTextTd(obj interface{}, field *FieldContext, ctx *web.EventContext) h.HTMLComponent {
	if field.Name == "ID" {
		id := field.StringValue(obj)
		if len(id) > 0 {
			mi := field.ModelInfo
			if mi == nil {
				return h.Td().Text(id)
			}

			var a h.HTMLComponent
			if mi.HasDetailing() && !mi.DetailingInDrawer() {
				a = h.A().Text(id).Attr("@click", web.Plaid().
					PushStateURL(mi.DetailingHref(id)).
					Go(),
				)
			} else {
				if field.ModelInfo.Verifier().Do(PermUpdate).ObjectOn(obj).WithReq(ctx.R).IsAllowed() == nil {
					a = h.A().Text(id).Attr("@click",
						web.Plaid().EventFunc(actions.Edit).
							Query(ParamID, id).
							Go())
				} else {
					a = h.Text(id)
				}
			}
			return h.Td(a)
		}
	}
	return h.Td(h.Text(field.StringValue(obj)))
}

func cfCheckbox(obj interface{}, field *FieldContext, ctx *web.EventContext) h.HTMLComponent {
	return VCheckbox().
		FieldName(field.FormKey).
		Label(field.Label).
		InputValue(reflectutils.MustGet(obj, field.Name).(bool)).
		ErrorMessages(field.Errors...).
		Disabled(field.Disabled)
}

func cfNumber(obj interface{}, field *FieldContext, ctx *web.EventContext) h.HTMLComponent {
	return VTextField().
		Type("number").
		FieldName(field.FormKey).
		Label(field.Label).
		Value(fmt.Sprint(reflectutils.MustGet(obj, field.Name))).
		ErrorMessages(field.Errors...).
		Disabled(field.Disabled)
}

func cfTextField(obj interface{}, field *FieldContext, ctx *web.EventContext) h.HTMLComponent {
	return VTextField().
		Type("text").
		FieldName(field.FormKey).
		Label(field.Label).
		Value(fmt.Sprint(reflectutils.MustGet(obj, field.Name))).
		ErrorMessages(field.Errors...).
		Disabled(field.Disabled)
}

func cfReadonlyText(obj interface{}, field *FieldContext, ctx *web.EventContext) h.HTMLComponent {
	return vuetifyx.VXReadonlyField().
		Label(field.Label).
		Value(fmt.Sprint(reflectutils.MustGet(obj, field.Name)))
}

func cfReadonlyCheckbox(obj interface{}, field *FieldContext, ctx *web.EventContext) h.HTMLComponent {
	return vuetifyx.VXReadonlyField().
		Label(field.Label).
		Value(reflectutils.MustGet(obj, field.Name)).
		Checkbox(true)
}

func (b *FieldDefaults) builtInFieldTypes() {

	if b.mode == LIST {
		b.FieldType(true).
			ComponentFunc(cfTextTd)

		for _, v := range numberVals {
			b.FieldType(v).
				ComponentFunc(cfTextTd)
		}

		for _, v := range stringVals {
			b.FieldType(v).
				ComponentFunc(cfTextTd)
		}
		return
	}

	if b.mode == DETAIL {
		b.FieldType(true).
			ComponentFunc(cfReadonlyCheckbox)

		for _, v := range numberVals {
			b.FieldType(v).
				ComponentFunc(cfReadonlyText)
		}

		for _, v := range stringVals {
			b.FieldType(v).
				ComponentFunc(cfReadonlyText)
		}
		return
	}

	b.FieldType(true).
		ComponentFunc(cfCheckbox)

	for _, v := range numberVals {
		b.FieldType(v).
			ComponentFunc(cfNumber)
	}

	for _, v := range stringVals {
		b.FieldType(v).
			ComponentFunc(cfTextField)
	}

	b.Exclude("ID")
	return
}
