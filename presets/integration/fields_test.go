package integration_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/goplaid/multipartestutils"
	"github.com/goplaid/web"
	. "github.com/goplaid/x/presets"
	"github.com/sunfmin/reflectutils"
	h "github.com/theplant/htmlgo"
	"github.com/theplant/testingutils"
)

type Company struct {
	Name      string
	FoundedAt time.Time
}

type Media string

type User struct {
	ID      int
	Int1    int
	Float1  float32
	String1 string
	Bool1   bool
	Time1   time.Time
	Company *Company
	Media1  Media
}

func TestFields(t *testing.T) {

	vd := &web.ValidationErrors{}
	vd.FieldError("String1", "too small")

	ft := NewFieldDefaults(WRITE).Exclude("ID")
	ft.FieldType(time.Time{}).ComponentFunc(func(obj interface{}, field *FieldContext, ctx *web.EventContext) h.HTMLComponent {
		return h.Div().Class("time-control").
			Text(field.Value(obj).(time.Time).Format("2006-01-02")).
			Attr(web.VFieldName(field.Name)...)
	})

	ft.FieldType(Media("")).ComponentFunc(func(obj interface{}, field *FieldContext, ctx *web.EventContext) h.HTMLComponent {
		if field.ContextValue("a") == nil {
			return h.Text("")
		}
		return h.Text(field.ContextValue("a").(string) + ", " + field.ContextValue("b").(string))
	})

	r := httptest.NewRequest("GET", "/hello", nil)

	ctx := &web.EventContext{R: r, Flash: vd}

	user := &User{
		ID:      1,
		Int1:    2,
		Float1:  23.1,
		String1: "hello",
		Bool1:   true,
		Time1:   time.Unix(1567048169, 0),
		Company: &Company{
			Name:      "Company1",
			FoundedAt: time.Unix(1567048169, 0),
		},
	}
	mb := New().Model(&User{})

	ftRead := NewFieldDefaults(LIST)

	var cases = []struct {
		name           string
		toComponentFun func() h.HTMLComponent
		expect         string
	}{
		{
			name: "Only with additional nested object",
			toComponentFun: func() h.HTMLComponent {
				return ft.InspectFields(&User{}).
					Labels("Int1", "整数1", "Company.Name", "公司名").
					Only("Int1", "Float1", "String1", "Bool1", "Time1", "Company.Name", "Company.FoundedAt").
					ToComponent(
						mb.Info(),
						user,
						ctx)
			},
			expect: `
<v-text-field type='number' v-field-name='[plaidForm, "Int1"]' label='整数1' :value='"2"' :disabled='false'></v-text-field>

<v-text-field type='number' v-field-name='[plaidForm, "Float1"]' label='Float1' :value='"23.1"' :disabled='false'></v-text-field>

<v-text-field type='text' v-field-name='[plaidForm, "String1"]' label='String1' :value='"hello"' :error-messages='["too small"]' :disabled='false'></v-text-field>

<v-checkbox v-field-name='[plaidForm, "Bool1"]' label='Bool1' :input-value='true' :disabled='false'></v-checkbox>

<div v-field-name='[plaidForm, "Time1"]' class='time-control'>2019-08-29</div>

<v-text-field type='text' v-field-name='[plaidForm, "Company.Name"]' label='公司名' :value='"Company1"' :disabled='false'></v-text-field>

<div v-field-name='[plaidForm, "Company.FoundedAt"]' class='time-control'>2019-08-29</div>
`,
		},

		{
			name: "Except with file glob pattern",
			toComponentFun: func() h.HTMLComponent {
				return ft.InspectFields(&User{}).
					Except("Bool*").
					ToComponent(mb.Info(), user, ctx)
			},
			expect: `
<v-text-field type='number' v-field-name='[plaidForm, "Int1"]' label='Int1' :value='"2"' :disabled='false'></v-text-field>

<v-text-field type='number' v-field-name='[plaidForm, "Float1"]' label='Float1' :value='"23.1"' :disabled='false'></v-text-field>

<v-text-field type='text' v-field-name='[plaidForm, "String1"]' label='String1' :value='"hello"' :error-messages='["too small"]' :disabled='false'></v-text-field>

<div v-field-name='[plaidForm, "Time1"]' class='time-control'>2019-08-29</div>
`,
		},

		{
			name: "Read Except with file glob pattern",
			toComponentFun: func() h.HTMLComponent {
				return ftRead.InspectFields(&User{}).
					Except("Float*").ToComponent(mb.Info(), user, ctx)
			},
			expect: `
<td>
<a @click='$plaid().vars(vars).form(plaidForm).eventFunc("presets_Edit").query("id", "1").go()'>1</a>
</td>

<td>2</td>

<td>hello</td>

<td>true</td>
`,
		},

		{
			name: "Read for a time field",
			toComponentFun: func() h.HTMLComponent {
				return ftRead.InspectFields(&User{}).
					Only("Time1", "Int1").ToComponent(mb.Info(), user, ctx)
			},
			expect: `
<td>2019-08-29 11:09:29 +0800 CST</td>

<td>2</td>
`,
		},

		{
			name: "pass in context",
			toComponentFun: func() h.HTMLComponent {
				fb := ft.InspectFields(&User{}).
					Only("Media1")
				fb.Field("Media1").
					WithContextValue("a", "context value1").
					WithContextValue("b", "context value2")
				return fb.ToComponent(mb.Info(), user, ctx)
			},
			expect: `context value1, context value2`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			output := h.MustString(c.toComponentFun(), web.WrapEventContext(context.TODO(), ctx))
			diff := testingutils.PrettyJsonDiff(c.expect, output)
			if len(diff) > 0 {
				t.Error(c.name, diff)
			}
		})
	}

}

type Person struct {
	Addresses []*Org
}

type Org struct {
	Name        string
	PeopleCount int
	Departments []*Department
}

type Department struct {
	Name      string
	Employees []*Employee
	DBStatus  string
}

type Employee struct {
	Number int
}

func TestFieldsBuilder(t *testing.T) {

	defaults := NewFieldDefaults(WRITE)

	employeeFbs := NewFieldsBuilder().Model(&Employee{}).Defaults(defaults)
	employeeFbs.Field("Number").ComponentFunc(func(obj interface{}, field *FieldContext, ctx *web.EventContext) h.HTMLComponent {
		return h.Input(field.FormKey).Type("text").Value(field.StringValue(obj))
	})

	employeeFbs.Field("FakeNumber").ComponentFunc(func(obj interface{}, field *FieldContext, ctx *web.EventContext) h.HTMLComponent {
		return h.Input(field.FormKey).Type("text").Value(fmt.Sprintf("900%v", reflectutils.MustGet(obj, "Number")))
	}).SetterFunc(func(obj interface{}, field *FieldContext, ctx *web.EventContext) (err error) {
		v := ctx.R.FormValue(field.FormKey)
		if v == "" {
			return
		}
		return reflectutils.Set(obj, "Number", "900"+v)
	})

	deptFbs := NewFieldsBuilder().Model(&Department{}).Defaults(defaults)
	deptFbs.Field("Name").ComponentFunc(func(obj interface{}, field *FieldContext, ctx *web.EventContext) h.HTMLComponent {
		// [0].Departments[0].Name
		// [0].Departments[1].Name
		// [1].Departments[0].Name
		return h.Input(field.FormKey).Type("text").Value(field.StringValue(obj))
	}).SetterFunc(func(obj interface{}, field *FieldContext, ctx *web.EventContext) (err error) {
		reflectutils.Set(obj, field.Name, ctx.R.FormValue(field.FormKey)+"!!!")
		// panic("dept name setter")
		return
	})

	deptFbs.ListField("Employees", employeeFbs).ComponentFunc(func(obj interface{}, field *FieldContext, ctx *web.EventContext) h.HTMLComponent {
		return h.Div(
			field.ListItemBuilder.ToComponentForEach(field, obj.(*Department).Employees, ctx, nil),
			h.Button("Add Employee"),
		).Class("employees")
	})

	fbs := NewFieldsBuilder().Model(&Org{}).Defaults(defaults)
	fbs.Field("Name").ComponentFunc(func(obj interface{}, field *FieldContext, ctx *web.EventContext) h.HTMLComponent {
		// [0].Name
		return h.Input(field.Name).Type("text").Value(field.StringValue(obj))
	})
	// .SetterFunc(func(obj interface{}, field *FieldContext, ctx *web.EventContext) (err error) {
	// 	return
	// })

	fbs.ListField("Departments", deptFbs).ComponentFunc(func(obj interface{}, field *FieldContext, ctx *web.EventContext) h.HTMLComponent {
		// [0].Departments
		return h.Div(
			field.ListItemBuilder.ToComponentForEach(field, obj.(*Org).Departments, ctx, nil),
			h.Button("Add Department"),
		).Class("departments")
	})

	fbs.Field("PeopleCount").SetterFunc(func(obj interface{}, field *FieldContext, ctx *web.EventContext) (err error) {
		reflectutils.Set(obj, field.Name, ctx.R.FormValue(field.FormKey))
		return
	})

	formObj := &Org{
		Name: "Name 1",
		Departments: []*Department{
			{
				Name: "11111",
				Employees: []*Employee{
					{Number: 111},
					{Number: 222},
					{Number: 333},
				},
			},
			{
				Name: "22222",
				Employees: []*Employee{
					{Number: 333},
					{Number: 444},
				},
			},
		},
	}

	ctx := &web.EventContext{
		R: httptest.NewRequest("POST", "/", nil),
	}

	ContextDeletedIndexesBuilder(ctx).
		Append("Departments[0].Employees", 1).
		Append("Departments[0].Employees", 5)

	result := fbs.ToComponent(nil, formObj, ctx)
	actual1 := h.MustString(result, context.TODO())

	expected1 := `
<input v-field-name='[plaidForm, "__Deleted.Departments[0].Employees"]' value='1,5'>

<input name='Name' type='text' value='Name 1'>

<div class='departments'>
<input name='Departments[0].Name' type='text' value='11111'>

<div class='employees'>
<input name='Departments[0].Employees[0].Number' type='text' value='111'>

<input name='Departments[0].Employees[0].FakeNumber' type='text' value='900111'>

<input name='Departments[0].Employees[2].Number' type='text' value='333'>

<input name='Departments[0].Employees[2].FakeNumber' type='text' value='900333'>

<button>Add Employee</button>
</div>

<input name='Departments[1].Name' type='text' value='22222'>

<div class='employees'>
<input name='Departments[1].Employees[0].Number' type='text' value='333'>

<input name='Departments[1].Employees[0].FakeNumber' type='text' value='900333'>

<input name='Departments[1].Employees[1].Number' type='text' value='444'>

<input name='Departments[1].Employees[1].FakeNumber' type='text' value='900444'>

<button>Add Employee</button>
</div>

<button>Add Department</button>
</div>

<v-text-field type='number' v-field-name='[plaidForm, "PeopleCount"]' label='PeopleCount' :value='"0"' :disabled='false'></v-text-field>
`
	diff := testingutils.PrettyJsonDiff(expected1, actual1)
	if diff != "" {
		t.Error(diff)
	}

	var unmarshalCases = []struct {
		name         string
		initial      *Org
		expected     *Org
		req          *http.Request
		deletedAsNil bool
	}{
		{
			name: "case with deleted",
			initial: &Org{
				Departments: []*Department{
					{
						Name: "Department A",
						Employees: []*Employee{
							{
								Number: 0,
							},
							{
								Number: 1,
							},
						},
					},
					{
						Name: "Department B",
						Employees: []*Employee{
							{Number: 0},
							{Number: 1},
							{Number: 2},
						},
					},
					{
						Name: "Department C",
					},
				},
			},
			req: multipartestutils.NewMultipartBuilder().
				AddField("Name", "Org 1").
				AddField("PeopleCount", "420").
				AddField("Departments[1].Name", "Department 1").
				AddField("Departments[1].Employees[0].Number", "888").
				AddField("Departments[1].Employees[2].Number", "999").
				AddField("Departments[1].Employees[1].FakeNumber", "666").
				AddField("Departments[1].DBStatus", "Verified").
				AddField("__Deleted.Departments[0].Employees", "1,5").
				AddField("__Deleted.Departments[1].Employees", "0").
				BuildEventFuncRequest(),
			deletedAsNil: true,
			expected: &Org{
				Name:        "Org 1",
				PeopleCount: 420,
				Departments: []*Department{
					{
						Name: "!!!",
						Employees: []*Employee{
							{
								Number: 0,
							},
							nil,
						},
					},
					{
						Name: "Department 1!!!",
						Employees: []*Employee{
							nil,
							{
								Number: 900666,
							},
							{
								Number: 999,
							},
						},
					},
					{
						Name: "Department C",
					},
				},
			},
		},

		{
			name: "deletedAsNil false",
			initial: &Org{
				Departments: []*Department{
					{
						Name: "Department A",
						Employees: []*Employee{
							{
								Number: 0,
							},
							{
								Number: 1,
							},
						},
					},
					{
						Name: "Department B",
						Employees: []*Employee{
							{Number: 0},
							{Number: 1},
							{Number: 2},
						},
					},
					{
						Name: "Department C",
					},
				},
			},
			req: multipartestutils.NewMultipartBuilder().
				AddField("Name", "Org 1").
				AddField("PeopleCount", "420").
				AddField("Departments[1].Name", "Department 1").
				AddField("Departments[1].Employees[0].Number", "888").
				AddField("Departments[1].Employees[2].Number", "999").
				AddField("Departments[1].Employees[1].FakeNumber", "666").
				AddField("Departments[1].DBStatus", "Verified").
				AddField("__Deleted.Departments[0].Employees", "1,5").
				AddField("__Deleted.Departments[1].Employees", "0").
				BuildEventFuncRequest(),
			deletedAsNil: false,
			expected: &Org{
				Name:        "Org 1",
				PeopleCount: 420,
				Departments: []*Department{
					{
						Name: "!!!",
						Employees: []*Employee{
							{
								Number: 0,
							},
						},
					},
					{
						Name: "Department 1!!!",
						Employees: []*Employee{
							{
								Number: 900666,
							},
							{
								Number: 999,
							},
						},
					},
					{
						Name: "Department C",
					},
				},
			},
		},
	}

	for _, c := range unmarshalCases {
		t.Run(c.name, func(t *testing.T) {
			ctx2 := &web.EventContext{R: c.req}
			_ = ctx2.R.ParseMultipartForm(128 << 20)
			actual2 := c.initial
			vErr := fbs.Unmarshal(actual2, nil, c.deletedAsNil, ctx2)
			if vErr.HaveErrors() {
				t.Error(vErr.Error())
			}
			diff = testingutils.PrettyJsonDiff(c.expected, actual2)
			if diff != "" {
				t.Error(diff)
			}
		})

	}

}
