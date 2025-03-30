package web

import "github.com/a-h/templ"

func Base(title string, body templ.Component) templ.Component {
	return base(title, body)
}

func Index() templ.Component {
	return index()
}

func ErrorPage(msg string) templ.Component {
	return errorPage(msg)
}

func Bench() templ.Component {
	return bench()
}
