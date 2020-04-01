package template

import (
	"io"
	"text/template"

	"path"

	"k8s.io/gengo/generator"
)

type installGenerator struct {
	generator.DefaultGen
	apigroup *APIGroup
}

var _ generator.Generator = &unversionedGenerator{}

func CreateInstallGenerator(apigroup *APIGroup, filename string) generator.Generator {
	return &installGenerator{
		generator.DefaultGen{OptionalName: filename},
		apigroup,
	}
}

func (d *installGenerator) Imports(c *generator.Context) []string {
	apisPkg := path.Dir(d.apigroup.Pkg.Path)
	imports := []string{
		"sigs.k8s.io/apiserver-builder-alpha/pkg/builders",
		`utilruntime "k8s.io/apimachinery/pkg/util/runtime"`,
		"k8s.io/apimachinery/pkg/runtime",
		`metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"`,
	}
	for _, version := range d.apigroup.Versions {
		imports = append(imports, path.Join(apisPkg, version.Group, version.Version))
	}
	imports = append(imports, path.Join(apisPkg, d.apigroup.Group))
	return imports
}

func (d *installGenerator) Finalize(context *generator.Context, w io.Writer) error {
	temp := template.Must(template.New("install-template").Parse(InstallAPITemplate))
	err := temp.Execute(w, d.apigroup)
	if err != nil {
		return err
	}
	return err
}

var InstallAPITemplate = `
func init() {
	Install(builders.Scheme)
}

func Install(scheme *runtime.Scheme) {
{{ range $version := .Versions -}}
	utilruntime.Must({{ $version.Version }}.AddToScheme(scheme))
{{ end -}}
	utilruntime.Must({{ $.Group }}.AddToScheme(scheme))
	utilruntime.Must(addKnownTypes(scheme))
}


func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes({{ $.Group }}.SchemeGroupVersion,
{{ range $api := .UnversionedResources -}}
		&{{ $.Group }}.{{ $api.Kind }}{},
		&{{ $.Group }}.{{ $api.Kind }}List{},
  {{ range $subresource := $api.Subresources -}}
		&{{ $.Group }}.{{ $subresource.Kind }}{},
  {{ end -}}
{{ end -}}
	)
	return nil
}
`
