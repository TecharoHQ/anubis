package web

import (
	"github.com/a-h/templ"
)

func Base(title string, body templ.Component) templ.Component {
	return base(title, body, nil)
}

func BaseWithOGTags(title string, body templ.Component, ogTags map[string]string) templ.Component {
	return base(title, body, ogTags)
}

func Index() templ.Component {
	return index()
}

func ErrorPage(msg string, mail string) templ.Component {
	return errorPage(msg, mail)
}

func Bench() templ.Component {
	return bench()
}
