package spew

import (
	"fmt"
	"log"
	"path"
	"reflect"
	"strings"
)

func k8sType(v reflect.Value) string {
	typeStr := v.Type().String()
	log.Printf(typeStr)
	if strings.HasPrefix(typeStr, "v1.") {
		k8sapi := path.Base(path.Dir(v.Type().PkgPath()))
		typeStr = k8sapi + typeStr

	}
	if strings.HasPrefix(typeStr, "[]v1.") {
		k8sapi := path.Base(path.Dir(v.Index(0).Type().PkgPath()))
		typeStr = strings.Replace(typeStr, "[]v1.", fmt.Sprintf("[]%sv1.", k8sapi), 1)
	}
	return typeStr
}
