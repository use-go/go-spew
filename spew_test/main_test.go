package spew_test

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/use-go/spew"
	istiov3 "github.com/weaveworks/flagger/pkg/apis/istio/v1alpha3"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
)

var (
	pkg = flag.String("pkg", "k8s", "go package for output go file")
)

func TestMainSpew(t *testing.T) {
	flag.Parse()
	ymls := flag.Args()
	if len(ymls) == 0 {
		fmt.Println("yml required")
		flag.Usage()
		os.Exit(1)
	}

	for _, file := range ymls {
		fmt.Println(file)
		f, err := os.Open(file)
		if err != nil {
			panic(err)
		}

		r := yaml.NewYAMLReader(bufio.NewReader(f))
		objs, err := parse(r)
		if err != nil {
			panic(err)
		}

		if err := f.Close(); err != nil {
			panic(err)
		}

		if err := output(file, *pkg, objs); err != nil {
			panic(err)
		}
	}
}

// parse reads the yaml and places all objects into the Resources
func parse(r *yaml.YAMLReader) ([]runtime.Object, error) {
	objs := []runtime.Object{}
	for {
		doc, err := r.Read()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		sch := runtime.NewScheme()
		scheme.AddToScheme(sch)
		sch.AddKnownTypes(apiextensionsv1beta1.SchemeGroupVersion, &apiextensionsv1beta1.CustomResourceDefinition{})
		sch.AddKnownTypes(istiov3.SchemeGroupVersion, &istiov3.VirtualService{}, &istiov3.DestinationRule{})
		d := serializer.NewCodecFactory(sch).UniversalDeserializer()

		obj, _, err := d.Decode(doc, nil, nil)
		if err != nil {
			return nil, err
		}

		objs = append(objs, obj)
	}
	return objs, nil

}

func output(src, pkg string, objs []runtime.Object) error {

	converter := spew.NewConfig(pkg)

	dump := make([]interface{}, len(objs))
	for i := range objs {
		dump[i] = objs[i]
	}

	structs := []string{}
	structs = append(structs, converter.Sdump(dump...))
	gofile := src + ".go"
	fmt.Printf("writing to %s", gofile)
	return ioutil.WriteFile(gofile, []byte(strings.Join(structs, "\n")), 0600)
}
