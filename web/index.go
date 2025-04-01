package web

import (
	"fmt"
	"github.com/a-h/templ"
)

func Base(title string, body templ.Component) templ.Component {
	return base(title, body)
}

func Index(ogTags map[string]string) templ.Component {
	fmt.Println("ogTags", ogTags)
	return index(ogTags)
}

func ErrorPage(msg string) templ.Component {
	return errorPage(msg)
}

func Bench() templ.Component {
	return bench()
}
