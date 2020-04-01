package template

import (
	"io"
	"text/template"

	"fmt"

	"k8s.io/gengo/generator"
)

type apiGenerator struct {
	generator.DefaultGen
	apis *APIs
}

var _ generator.Generator = &apiGenerator{}

func CreateApisGenerator(apis *APIs, filename string) generator.Generator {
	return &apiGenerator{
		generator.DefaultGen{OptionalName: filename},
		apis,
	}
}

func (d *apiGenerator) Imports(c *generator.Context) []string {
	imports := []string{
		"sigs.k8s.io/apiserver-builder-alpha/pkg/builders",
		"k8s.io/apimachinery/pkg/runtime",
	}
	for _, group := range d.apis.Groups {
		imports = append(imports, group.PkgPath)
		for _, version := range group.Versions {
			imports = append(imports, fmt.Sprintf(
				"%s%s \"%s\"", group.Group, version.Version, version.Pkg.Path))
		}
	}
	for _, group := range d.apis.Groups {
		imports = append(imports, fmt.Sprintf(
			"_ \"%s/install\" // Install the %s group", group.Pkg.Path, group.Group))
	}

	return imports
}

func (d *apiGenerator) Finalize(context *generator.Context, w io.Writer) error {
	temp := template.Must(template.New("apis-template").Parse(APIsTemplate))
	err := temp.Execute(w, d.apis)
	if err != nil {
		return err
	}
	return err
}

var APIsTemplate = `
var (
	localSchemeBuilder = runtime.SchemeBuilder{
{{ range $group := .Groups -}}
	{{ range $version := $group.Versions -}}
		{{ $group.Group }}{{ $version.Version }}.AddToScheme,
	{{ end -}}
{{ end -}}
	}
	AddToScheme = localSchemeBuilder.AddToScheme
)

// GetAllApiBuilders returns all known APIGroupBuilders
// so they can be registered with the apiserver
func GetAllApiBuilders() []*builders.APIGroupBuilder {
	return []*builders.APIGroupBuilder{
		{{ range $group := .Groups -}}
		Get{{ $group.GroupTitle }}APIBuilder(),
		{{ end -}}
	}
}

{{ range $group := .Groups -}}
var {{ $group.Group }}ApiGroup = builders.NewApiGroupBuilder(
	"{{ $group.Group }}.{{ $group.Domain }}",
	"{{ $group.PkgPath}}").
	WithUnVersionedApi({{ $group.Group }}.ApiVersion).
	WithVersionedApis(
		{{ range $version := $group.Versions -}}
		{{ $group.Group }}{{ $version.Version }}.ApiVersion,
		{{ end -}}
	).
	WithRootScopedKinds(
		{{ range $version := $group.Versions -}}
		{{ range $res := $version.Resources -}}
		{{ if $res.NonNamespaced -}}
		"{{ $res.Kind }}",
		{{ end -}}
		{{ end -}}
		{{ end -}}
	)

func Get{{ $group.GroupTitle }}APIBuilder() *builders.APIGroupBuilder {
	return {{ $group.Group }}ApiGroup
}
{{ end -}}
`
