# Allow double-slashes in paths

This test ensures that paths of the form `//foo` get passed to the target server as `//foo`.

## Tale of woe

Gentoo's wiki has pages named after configuration files. Prior to this fix, Anubis passed everything through [net/http#ServeMux](https://pkg.go.dev/net/http#ServeMux). ServeMux cleans paths to remove common client errors like two slashes in a row. This is a problem for servers that actually want it.

To fix it, Anubis needed to use some prefix comparison logic to dispatch to ServeMux and then otherwise reverse proxy to the target service. This preserves the double slashes.

This smoke test reads [input.txt](./input.txt) and passes every path to httpdebug, making sure the path comes out right on the other end.
