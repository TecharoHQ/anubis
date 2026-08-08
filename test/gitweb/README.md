# gitweb

Tests [gitweb](https://git-scm.com/docs/gitweb), a Perl CGI script that gives web browsers an interface to git repositories.

## Tale of woe

Gitweb is a CGI script from before the turn of the century. As a result it has the unique legacy behaviour of [using semicolons as parameter separators](https://stackoverflow.com/questions/3481664/semicolon-as-url-query-separator). This is notably a WAF bypass technique in the hacker underground, as some poorly configured web applications will accept semicolons to separate URL parameters but the WAF only looks for ampersands.

Either way, when [v1.26.0](https://github.com/TecharoHQ/anubis/releases/tag/v1.26.0) was released, Anubis switched to using `httputil.ReverseProxy.Rewrite` instead of the deprecated `Director`. According to the [`Rewrite` documentation comment](https://pkg.go.dev/net/http/httputil#ReverseProxy), it's a lot more militant about correctness and conforming to what HTTP does in practice today, not what it did years ago.

This smoke test will make sure that using semicolons as URL query parameters continues working.
