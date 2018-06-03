package kew

import (
	"fmt"
	"path"
	"reflect"
	"strings"
)

func k8sType(d *dumpState, v reflect.Value) string {
	typeStr := v.Type().String()
	if strings.HasPrefix(typeStr, "v1.") {
		pkgPath := v.Type().PkgPath()
		k8sapi := path.Base(path.Dir(pkgPath))
		typeStr = k8sapi + typeStr
		d.addImport(pkgPath, k8sapi+"v1")

	}
	if strings.HasPrefix(typeStr, "[]v1.") {
		pkgPath := v.Index(0).Type().PkgPath()
		k8sapi := path.Base(path.Dir(pkgPath))
		d.addImport(pkgPath, k8sapi+"v1")
		typeStr = strings.Replace(typeStr, "[]v1.", fmt.Sprintf("[]%sv1.", k8sapi), 1)
	}
	pkgPath := v.Type().PkgPath()
	d.addImport(pkgPath, "")
	return typeStr
}
