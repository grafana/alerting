package templates

import (
	"text/template"

	"github.com/Masterminds/sprig/v3"
	alertmanager "github.com/prometheus/alertmanager/template"
)

var (
	DefaultFuncs = funcMap()
)

func funcMap() template.FuncMap {
	f := sprig.TxtFuncMap()
	delete(f, "env")
	delete(f, "expandenv")

	for k, v := range alertmanager.DefaultFuncs {
		f[k] = v
	}

	return f
}
