go-spew
=======

[![Build Status](https://img.shields.io/travis/use-go/spew.svg)](https://travis-ci.org/use-go/spew)
[![ISC License](http://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org)

Simply added IgnoreFieldByName(string) and IgnoreFieldByType(string) to suppress dumps.  IgnoreFieldByType will also add []name and *name so pointers to and arrays of the speciified name are also ignored.

## Update History

Merged: https://github.com/use-go/spew
Merged: https://github.com/goller/kew
Merged: https://github.com/use-go/go-spew
Merged: https://github.com/klaxxon/go-spew

## Documentation

[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](http://pkg.go.dev/github.com/use-go/spew)

Full `go doc` style documentation for the project can be viewed online without
installing this package by using the excellent GoDoc site here:
http://pkg.go.dev/github.com/use-go/spew

You can also view the documentation locally once the package is installed with
the `godoc` tool by running `godoc -http=":6060"` and pointing your browser to
http://localhost:6060/pkg/github.com/use-go/spew

## Installation

```bash
$ go get -u github.com/use-go/go-spew/spew
```

## Quick Start

Add this import line to the file you're working in:

```Go
import "github.com/use-go/go-spew/spew
```

To dump a variable with full newlines, indentation, type, and pointer
information use Dump, Fdump, or Sdump:

```Go
spew.Dump(myVar1, myVar2, ...)
spew.Fdump(someWriter, myVar1, myVar2, ...)
str := spew.Sdump(myVar1, myVar2, ...)
```

Alternatively, if you would prefer to use format strings with a compacted inline
printing style, use the convenience wrappers Printf, Fprintf, etc with %v (most
compact), %+v (adds pointer addresses), %#v (adds types), or %#+v (adds types
and pointer addresses): 

```Go
spew.Printf("myVar1: %v -- myVar2: %+v", myVar1, myVar2)
spew.Printf("myVar3: %#v -- myVar4: %#+v", myVar3, myVar4)
spew.Fprintf(someWriter, "myVar1: %v -- myVar2: %+v", myVar1, myVar2)
spew.Fprintf(someWriter, "myVar3: %#v -- myVar4: %#+v", myVar3, myVar4)
```

## Debugging a Web Application Example

Here is an example of how you can use `spew.Sdump()` to help debug a web application. Please be sure to wrap your output using the `html.EscapeString()` function for safety reasons. You should also only use this debugging technique in a development environment, never in production.

```Go
package main

import (
    "fmt"
    "html"
    "net/http"

    "github.com/use-go/spew"
)

func handler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html")
    fmt.Fprintf(w, "Hi there, %s!", r.URL.Path[1:])
    fmt.Fprintf(w, "<!--\n" + html.EscapeString(spew.Sdump(w)) + "\n-->")
}

func main() {
    http.HandleFunc("/", handler)
    http.ListenAndServe(":8080", nil)
}
```

## Sample Dump Output

```
(main.Foo) {
 unexportedField: (*main.Bar)(0xf84002e210)({
  flag: (main.Flag) flagTwo,
  data: (uintptr) <nil>
 }),
 ExportedField: (map[interface {}]interface {}) {
  (string) "one": (bool) true
 }
}
([]uint8) {
 00000000  11 12 13 14 15 16 17 18  19 1a 1b 1c 1d 1e 1f 20  |............... |
 00000010  21 22 23 24 25 26 27 28  29 2a 2b 2c 2d 2e 2f 30  |!"#$%&'()*+,-./0|
 00000020  31 32                                             |12|
}
```

## Sample Formatter Output

Double pointer to a uint8:
```
	  %v: <**>5
	 %+v: <**>(0xf8400420d0->0xf8400420c8)5
	 %#v: (**uint8)5
	%#+v: (**uint8)(0xf8400420d0->0xf8400420c8)5
```

Pointer to circular struct with a uint8 field and a pointer to itself:
```
	  %v: <*>{1 <*><shown>}
	 %+v: <*>(0xf84003e260){ui8:1 c:<*>(0xf84003e260)<shown>}
	 %#v: (*main.circular){ui8:(uint8)1 c:(*main.circular)<shown>}
	%#+v: (*main.circular)(0xf84003e260){ui8:(uint8)1 c:(*main.circular)(0xf84003e260)<shown>}
```

## Configuration Options

Configuration of spew is handled by fields in the ConfigState type. For
convenience, all of the top-level functions use a global state available via the
spew.Config global.

It is also possible to create a ConfigState instance that provides methods
equivalent to the top-level functions. This allows concurrent configuration
options. See the ConfigState documentation for more details.

```
* Indent
	String to use for each indentation level for Dump functions.
	It is a single space by default.  A popular alternative is "\t".

* MaxDepth
	Maximum number of levels to descend into nested data structures.
	There is no limit by default.

* DisableMethods
	Disables invocation of error and Stringer interface methods.
	Method invocation is enabled by default.

* DisablePointerMethods
	Disables invocation of error and Stringer interface methods on types
	which only accept pointer receivers from non-pointer variables.  This option
	relies on access to the unsafe package, so it will not have any effect when
	running in environments without access to the unsafe package such as Google
	App Engine or with the "safe" build tag specified.
	Pointer method invocation is enabled by default.

* DisablePointerAddresses
	DisablePointerAddresses specifies whether to disable the printing of
	pointer addresses. This is useful when diffing data structures in tests.

* DisableCapacities
	DisableCapacities specifies whether to disable the printing of capacities
	for arrays, slices, maps and channels. This is useful when diffing data
	structures in tests.

* ContinueOnMethod
	Enables recursion into types after invoking error and Stringer interface
	methods. Recursion after method invocation is disabled by default.

* SortKeys
	Specifies map keys should be sorted before being printed. Use
	this to have a more deterministic, diffable output.  Note that
	only native types (bool, int, uint, floats, uintptr and string)
	and types which implement error or Stringer interfaces are supported,
	with other types sorted according to the reflect.Value.String() output
	which guarantees display stability.  Natural map order is used by
	default.

* SpewKeys
	SpewKeys specifies that, as a last resort attempt, map keys should be
	spewed to strings and sorted by those strings.  This is only considered
	if SortKeys is true.

* NoDuplicates
	NoDuplicates specifies that any given pointer should have
	its dereference dumped only once. This is similar to
	circularity detection, but applies to all pointers across a
	given dump action (or SpewState). This can consume much memory.

* UseOrdinals
	UseOrdinals specifies that pointer values are to be
	replaced with monotonically increasing integers. It has no
	effect if DisablePointerAddresses is true; else, it
	provides some degree of stability across runs versus
	printing out raw pointers. This can consume much memory.

* PreserveSpewState
	Preserve state of a spew (dump, format) operation for use
	by the next such operation (including across multiple
	arguments to a single API call as well as across API
	calls). Currently useful only when NoDuplicates is true.

* SpewState
	The state of the last spew (dump, format) operation, if
	PreserveSpewState was true when that operation was started.
	Can be copied to a different ConfigState object.

```

## Unsafe Package Dependency

This package relies on the unsafe package to perform some of the more advanced
features, however it also supports a "limited" mode which allows it to work in
environments where the unsafe package is not available.  By default, it will
operate in this mode on Google App Engine and when compiled with GopherJS.  The
"safe" build tag may also be specified to force the package to build without
using the unsafe package.

## License

Go-spew is licensed under the [copyfree](http://copyfree.org) ISC License.
