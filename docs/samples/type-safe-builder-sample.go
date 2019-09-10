package samples

// @snippet_begin(TypeSafeBuilderSample)
import (
	"github.com/sunfmin/bran/ui"
	. "github.com/theplant/htmlgo"
)

func result(args ...HTMLComponent) HTMLComponent {

	var converted []HTMLComponent
	for _, arg := range args {
		converted = append(converted, Div(arg).Class("wrapped"))
	}

	return HTML(
		Head(
			Title("XML encoding with Go"),
		),
		Body(
			H1("XML encoding with Go"),
			P().Text("this format can be used as an alternative markup to XML"),
			A().Href("http://golang.org").Text("Go"),
			P(
				Text("this is some"),
				B("mixed"),
				Text("text. For more see the"),
				A().Href("http://golang.org").Text("Go"),
				Text("project"),
			),
			P().Text("some text"),

			P(converted...),
		),
	)
}

func TypeSafeBuilderSamplePF(ctx *ui.EventContext) (pr ui.PageResponse, err error) {
	pr.Schema = result(H5("1"), B("2"), Strong("3"))
	return
}

// @snippet_end

const TypeSafeBuilderSamplePath = "/samples/type_safe_builder_sample"
