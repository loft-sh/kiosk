package template

import (
	"k8s.io/klog"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/gengo/args"
	"k8s.io/gengo/generator"
	"k8s.io/gengo/namer"
	"k8s.io/gengo/types"
)

// CustomArgs is used tby the go2idl framework to pass args specific to this
// generator.
type CustomArgs struct{}

type Gen struct {
	p []generator.Package
}

func (g *Gen) Execute(arguments *args.GeneratorArgs) error {
	return arguments.Execute(
		g.NameSystems(),
		g.DefaultNameSystem(),
		g.Packages)
}

// DefaultNameSystem returns the default name system for ordering the types to be
// processed by the generators in this package.
func (g *Gen) DefaultNameSystem() string {
	return "public"
}

// NameSystems returns the name system used by the generators in this package.
func (g *Gen) NameSystems() namer.NameSystems {
	return namer.NameSystems{
		"public": namer.NewPublicNamer(1),
		"raw":    namer.NewRawNamer("", nil),
	}
}

func (g *Gen) ParsePackages(context *generator.Context, arguments *args.GeneratorArgs) (sets.String, sets.String, string, string) {
	versionedPkgs := sets.NewString()
	unversionedPkgs := sets.NewString()
	mainPkg := ""
	apisPkg := ""
	for _, o := range context.Order {
		if IsAPIResource(o) {
			versioned := o.Name.Package
			versionedPkgs.Insert(versioned)
			unversioned := filepath.Dir(versioned)
			unversionedPkgs.Insert(unversioned)

			if apis := filepath.Dir(unversioned); apis != apisPkg && len(apisPkg) > 0 {
				panic(errors.Errorf(
					"Found multiple apis directory paths: %v and %v", apisPkg, apis))
			} else {
				apisPkg = apis
				mainPkg = filepath.Dir(apisPkg)
			}
		}
	}
	return versionedPkgs, unversionedPkgs, apisPkg, mainPkg
}

func (g *Gen) Packages(context *generator.Context, arguments *args.GeneratorArgs) generator.Packages {
	boilerplate, err := arguments.LoadGoBoilerplate()
	if err != nil {
		klog.Warningf("failed loading boilerplate, fallback to default boilerplate: %v", err)
		boilerplate = []byte{}
	}
	g.p = generator.Packages{}

	b := NewAPIsBuilder(context, arguments)
	for _, apigroup := range b.APIs.Groups {
		for _, apiversion := range apigroup.Versions {
			factory := &packageFactory{apiversion.Pkg.Path, arguments, boilerplate}
			// Add generators for versioned types
			gen := CreateVersionedGenerator(apiversion, apigroup, arguments.OutputFileBaseName)
			g.p = append(g.p, factory.createPackage(gen))
		}

		factory := &packageFactory{apigroup.Pkg.Path, arguments, boilerplate}
		gen := CreateUnversionedGenerator(apigroup, arguments.OutputFileBaseName)
		g.p = append(g.p, factory.createPackage(gen))

		factory = &packageFactory{path.Join(apigroup.Pkg.Path, "install"), arguments, boilerplate}
		gen = CreateInstallGenerator(apigroup, arguments.OutputFileBaseName)
		g.p = append(g.p, factory.createPackage(gen))
	}

	apisFactory := &packageFactory{b.APIs.Pkg.Path, arguments, boilerplate}
	gen := CreateApisGenerator(b.APIs, arguments.OutputFileBaseName)
	g.p = append(g.p, apisFactory.createPackage(gen))

	return g.p
}

type packageFactory struct {
	path       string
	arguments  *args.GeneratorArgs
	headerText []byte
}

// Creates a package with a generator
func (f *packageFactory) createPackage(gen generator.Generator) generator.Package {
	path := f.path
	name := strings.Split(filepath.Base(f.path), ".")[0]
	return &generator.DefaultPackage{
		PackageName: name,
		PackagePath: path,
		HeaderText:  f.headerText,
		GeneratorFunc: func(c *generator.Context) (generators []generator.Generator) {
			return []generator.Generator{gen}
		},
		FilterFunc: func(c *generator.Context, t *types.Type) bool {
			return t.Name.Package == f.path
		},
	}
}
