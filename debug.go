package tsrpc

import (
	"fmt"
	"html/template"
	"net/http"
)

// debugText defines the HTML template used for debugging display.
const debugText = `<html>
	<body>
	<title>TSRPC Services</title>
	{{range .}}
	<hr>
	Service {{.Name}}
	<hr>
		<table>
		<th align=center>Method</th><th align=center>Calls</th>
		{{range $name, $mtype := .Method}}
			<tr>
			<td align=left font=fixed>{{$name}}({{$mtype.ArgType}}, {{$mtype.ReplyType}}) error</td>
			<td align=center>{{$mtype.NumCalls}}</td>
			</tr>
		{{end}}
		</table>
	{{end}}
	</body>
	</html>
`

// debug is a template object used for debugging display.
var debug = template.Must(template.New("RPC debug").Parse(debugText))

// debugHTTP encapsulates an HTTP server with debug information.
type debugHTTP struct {
	*Server // Inherits from Server
}

// debugService defines debug information for a service.
type debugService struct {
	Name   string                 // Service name
	Method map[string]*methodType // Method map, with method names as keys and method type information as values
}

// ServeHTTP implements the ServeHTTP method of the HTTP server for handling debug information HTTP requests.
func (s debugHTTP) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var services []debugService
	s.serviceMap.Range(func(namei interface{}, svci interface{}) bool {
		svc := svci.(*service)
		services = append(services, debugService{
			Name:   namei.(string),
			Method: svc.method,
		})
		return true
	})

	err := debug.Execute(w, services) // Execute the HTML template and render the debug information into the response
	if err != nil {
		_, _ = fmt.Fprintln(w, "rpc: error executing template:", err.Error())
	}
}
