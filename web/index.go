package web

import "github.com/a-h/templ"

func Base(title string, body templ.Component) templ.Component {
	return base(title, body)
}

func Index(redirectUrl string) templ.Component {
	return index(redirectUrl)
}

func ErrorPage(msg string) templ.Component {
	return errorPage(msg)
}
