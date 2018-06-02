/*
 * Copyright (c) 2013-2016 Dave Collins <dave@davec.name>
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package spew

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

var (
	// uint8Type is a reflect.Type representing a uint8.  It is used to
	// convert cgo types to uint8 slices for hexdumping.
	uint8Type = reflect.TypeOf(uint8(0))

	// cCharRE is a regular expression that matches a cgo char.
	// It is used to detect character arrays to hexdump them.
	cCharRE = regexp.MustCompile(`^.*\._Ctype_char$`)

	// cUnsignedCharRE is a regular expression that matches a cgo unsigned
	// char.  It is used to detect unsigned character arrays to hexdump
	// them.
	cUnsignedCharRE = regexp.MustCompile(`^.*\._Ctype_unsignedchar$`)

	// cUint8tCharRE is a regular expression that matches a cgo uint8_t.
	// It is used to detect uint8_t arrays to hexdump them.
	cUint8tCharRE = regexp.MustCompile(`^.*\._Ctype_uint8_t$`)
)

// dumpState contains information about the state of a dump operation.
type dumpState struct {
	w                io.Writer
	depth            int
	pointers         map[uintptr]int
	ignoreNextType   bool
	ignoreNextIndent bool
	imports          map[string]string
	cs               *ConfigState
}

// indent performs indentation according to the depth level and cs.Indent
// option.
func (d *dumpState) indent() {
	if d.ignoreNextIndent {
		d.ignoreNextIndent = false
		return
	}
	d.w.Write(bytes.Repeat([]byte(d.cs.Indent), d.depth))
}

// unpackValue returns values inside of non-nil interfaces when possible.
// This is useful for data types like structs, arrays, slices, and maps which
// can contain varying types packed inside an interface.
func (d *dumpState) unpackValue(v reflect.Value) reflect.Value {
	if v.Kind() == reflect.Interface && !v.IsNil() {
		v = v.Elem()
	}
	return v
}

func (d *dumpState) addImport(pkgPath, alias string) {
	var re = regexp.MustCompile(`(?m)^.*\/vendor\/`)
	pkgPath = re.ReplaceAllString(pkgPath, "")
	if _, ok := d.imports[pkgPath]; !ok {
		d.imports[pkgPath] = alias
	}
}

// dumpPtr handles formatting of pointers by indirecting them as necessary.
func (d *dumpState) dumpPtr(v reflect.Value) {
	// Remove pointers at or below the current depth from map used to detect
	// circular refs.
	for k, depth := range d.pointers {
		if depth >= d.depth {
			delete(d.pointers, k)
		}
	}

	// Keep list of all dereferenced pointers to show later.
	pointerChain := make([]uintptr, 0)

	// Figure out how many levels of indirection there are by dereferencing
	// pointers and unpacking interfaces down the chain while detecting circular
	// references.
	nilFound := false
	cycleFound := false
	indirects := 0
	ve := v
	for ve.Kind() == reflect.Ptr {
		if ve.IsNil() {
			nilFound = true
			break
		}
		indirects++
		addr := ve.Pointer()
		pointerChain = append(pointerChain, addr)
		if pd, ok := d.pointers[addr]; ok && pd < d.depth {
			cycleFound = true
			indirects--
			break
		}
		d.pointers[addr] = d.depth

		ve = ve.Elem()
		if ve.Kind() == reflect.Interface {
			if ve.IsNil() {
				nilFound = true
				break
			}
			ve = ve.Elem()
		}
	}
	if nilFound {
		d.w.Write([]byte("nil"))
		return
	}

	addressable := Addressable(ve)
	// Display type information.
	if !addressable {
		fn := fmt.Sprintf("func(x %s) *%s {return &x }(", ve.Type().String(), ve.Type().String())
		d.w.Write([]byte(fn))
	} else {
		if d.depth == 0 {
			d.w.Write([]byte("var _ = "))
		}
		d.w.Write(bytes.Repeat(amperBytes, indirects))
		d.w.Write([]byte(k8sType(d, ve)))
	}

	// Display dereferenced value.
	switch {
	case cycleFound:
		d.w.Write(circularBytes)

	default:
		d.ignoreNextType = true
		d.dump(ve)
	}
	if !addressable {
		d.w.Write(closeParenBytes)
	}
}

// dumpSlice handles formatting of arrays and slices.  Byte (uint8 under
// reflection) arrays and slices are dumped in hexdump -C fashion.
func (d *dumpState) dumpSlice(v reflect.Value) {
	// Determine whether this type should be hex dumped or not.  Also,
	// for types which should be hexdumped, try to use the underlying data
	// first, then fall back to trying to convert them to a uint8 slice.
	var buf []uint8
	doConvert := false
	doHexDump := false
	numEntries := v.Len()
	if numEntries > 0 {
		vt := v.Index(0).Type()
		vts := vt.String()
		switch {
		// C types that need to be converted.
		case cCharRE.MatchString(vts):
			fallthrough
		case cUnsignedCharRE.MatchString(vts):
			fallthrough
		case cUint8tCharRE.MatchString(vts):
			doConvert = true

		// Try to use existing uint8 slices and fall back to converting
		// and copying if that fails.
		case vt.Kind() == reflect.Uint8:
			// We need an addressable interface to convert the type
			// to a byte slice.  However, the reflect package won't
			// give us an interface on certain things like
			// unexported struct fields in order to enforce
			// visibility rules.  We use unsafe, when available, to
			// bypass these restrictions since this package does not
			// mutate the values.
			vs := v
			if !vs.CanInterface() || !vs.CanAddr() {
				vs = unsafeReflectValue(vs)
			}
			if !UnsafeDisabled {
				vs = vs.Slice(0, numEntries)

				// Use the existing uint8 slice if it can be
				// type asserted.
				iface := vs.Interface()
				if slice, ok := iface.([]uint8); ok {
					buf = slice
					doHexDump = true
					break
				}
			}

			// The underlying data needs to be converted if it can't
			// be type asserted to a uint8 slice.
			doConvert = true
		}

		// Copy and convert the underlying type if needed.
		if doConvert && vt.ConvertibleTo(uint8Type) {
			// Convert and copy each element into a uint8 byte
			// slice.
			buf = make([]uint8, numEntries)
			for i := 0; i < numEntries; i++ {
				vv := v.Index(i)
				buf[i] = uint8(vv.Convert(uint8Type).Uint())
			}
			doHexDump = true
		}
	}

	// Hexdump the entire slice as needed.
	if doHexDump {
		//indent := strings.Repeat(d.cs.Indent, d.depth)
		//str := indent + hex.Dump(buf)
		str := "`" + string(buf) + "`"
		str = strings.TrimRight(str, d.cs.Indent)
		d.w.Write([]byte(str))
		return
	}

	// Recursively call dump for each item.
	for i := 0; i < numEntries; i++ {
		d.dump(d.unpackValue(v.Index(i)))
		d.w.Write(commaNewlineBytes)
	}
}

// dump is the main workhorse for dumping a value.  It uses the passed reflect
// value to figure out what kind of object we are dealing with and formats it
// appropriately.  It is a recursive function, however circular data structures
// are detected and handled properly.
func (d *dumpState) dump(v reflect.Value) {
	// Handle invalid reflect values immediately.
	kind := v.Kind()
	if kind == reflect.Invalid {
		d.w.Write(invalidAngleBytes)
		return
	}

	// Handle pointers specially.
	if kind == reflect.Ptr {
		d.indent()
		d.dumpPtr(v)
		return
	}

	byteString := false
	// Print type information unless already handled elsewhere.
	if !d.ignoreNextType {
		d.indent()
		switch kind {
		case reflect.Slice, reflect.Array:
			vt := v.Index(0).Type().Kind()
			if vt == reflect.Uint8 {
				byteString = true
				d.w.Write([]byte("[]byte"))
			} else {
				d.w.Write([]byte(k8sType(d, v)))
			}
		case reflect.Map:
			d.w.Write([]byte(k8sType(d, v)))
		case reflect.Interface, reflect.Struct:
			if v.Type().String() == "resource.Quantity" && !d.isZero(v) && v.IsValid() {
				d.addImport("k8s.io/apimachinery/pkg/api/resource", "")
				strfunc := v.MethodByName("MarshalJSON")
				if !d.isZero(strfunc) {
					marshaled := strfunc.Call(nil)
					octets := marshaled[0].Bytes()
					quantity := fmt.Sprintf(`resource.MustParse(%s)`, string(octets))
					d.w.Write([]byte(quantity))
					byteString = true
				}
			} else {
				d.w.Write([]byte(k8sType(d, v)))
			}
		}
		if !byteString {
			d.w.Write(spaceBytes)
		}
	}
	d.ignoreNextType = false

	// Call Stringer/error interfaces if they exist and the handle methods flag
	// is enabled
	if !d.cs.DisableMethods {
		if (kind != reflect.Invalid) && (kind != reflect.Interface) {
			if handled := handleMethods(d.cs, d.w, v); handled {
				return
			}
		}
	}

	switch kind {
	case reflect.Invalid:
		// Do nothing.  We should never get here since invalid has already
		// been handled above.

	case reflect.Bool:
		printBool(d.w, v.Bool())

	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		printInt(d.w, v.Int(), 10)

	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		printUint(d.w, v.Uint(), 10)

	case reflect.Float32:
		printFloat(d.w, v.Float(), 32)

	case reflect.Float64:
		printFloat(d.w, v.Float(), 64)

	case reflect.Complex64:
		printComplex(d.w, v.Complex(), 32)

	case reflect.Complex128:
		printComplex(d.w, v.Complex(), 64)

	case reflect.Slice:
		if v.IsNil() {
			d.w.Write([]byte("nil"))
			break
		}
		fallthrough

	case reflect.Array:
		openBytes := openBraceNewlineBytes
		closeBytes := closeBraceBytes
		if byteString {
			openBytes = openParenBytes
			closeBytes = closeParenBytes
		}
		d.w.Write(openBytes)
		d.depth++
		if (d.cs.MaxDepth != 0) && (d.depth > d.cs.MaxDepth) {
			d.indent()
			d.w.Write(maxNewlineBytes)
		} else {
			d.dumpSlice(v)
		}
		d.depth--
		if !byteString {
			d.indent()
		}
		d.w.Write(closeBytes)

	case reflect.String:
		str := v.String()
		if strings.Contains(str, "\n") {
			var re = regexp.MustCompile(`(?m)(\x60.*?\x60)`)
			str = re.ReplaceAllString(str, "`+\"${1}\"+`")
			d.w.Write([]byte("`" + str + "`"))
		} else {
			d.w.Write([]byte(strconv.Quote(str)))
		}
	case reflect.Interface:
		// The only time we should get here is for nil interfaces due to
		// unpackValue calls.
		if v.IsNil() {
			d.w.Write([]byte("nil"))
		}

	case reflect.Ptr:
		// Do nothing.  We should never get here since pointers have already
		// been handled above.

	case reflect.Map:
		// nil maps should be indicated as different than empty maps
		if v.IsNil() {
			d.w.Write([]byte("nil"))
			break
		}

		d.w.Write(openBraceNewlineBytes)
		d.depth++
		if (d.cs.MaxDepth != 0) && (d.depth > d.cs.MaxDepth) {
			d.indent()
			d.w.Write(maxNewlineBytes)
		} else {
			keys := v.MapKeys()
			if d.cs.SortKeys {
				sortValues(keys, d.cs)
			}
			for _, key := range keys {
				d.dump(d.unpackValue(key))
				d.w.Write(colonSpaceBytes)
				d.ignoreNextIndent = true
				d.dump(d.unpackValue(v.MapIndex(key)))
				d.w.Write(commaNewlineBytes)
			}
		}
		d.depth--
		d.indent()
		d.w.Write(closeBraceBytes)

	case reflect.Struct:
		if v.Type().String() != "resource.Quantity" {
			d.w.Write(openBraceNewlineBytes)
			d.depth++
			if (d.cs.MaxDepth != 0) && (d.depth > d.cs.MaxDepth) {
				d.indent()
				d.w.Write(maxNewlineBytes)
			} else {
				vt := v.Type()
				numFields := v.NumField()
				for i := 0; i < numFields; i++ {
					if d.isZero(v.Field(i)) {
						continue
					}

					d.indent()
					vtf := vt.Field(i)

					d.w.Write([]byte(vtf.Name))
					d.w.Write(colonSpaceBytes)
					d.ignoreNextIndent = true
					d.dump(d.unpackValue(v.Field(i)))
					d.w.Write(commaNewlineBytes)
				}
			}
			d.depth--
			d.indent()
			d.w.Write(closeBraceBytes)
		}
	case reflect.Uintptr:
		printHexPtr(d.w, uintptr(v.Uint()))

	case reflect.UnsafePointer, reflect.Chan, reflect.Func:
		printHexPtr(d.w, v.Pointer())

	// There were not any other types at the time this code was written, but
	// fall back to letting the default fmt package handle it in case any new
	// types are added.
	default:
		if v.CanInterface() {
			fmt.Fprintf(d.w, "%v", v.Interface())
		} else {
			fmt.Fprintf(d.w, "%v", v.String())
		}
	}
}

// fdump is a helper function to consolidate the logic from the various public
// methods which take varying writers and config states.
func fdump(cs *ConfigState, w io.Writer, a ...interface{}) {
	imports := map[string]string{}
	structs := []string{}

	for _, arg := range a {
		var b strings.Builder
		if arg == nil {
			continue
		}
		d := dumpState{w: &b, cs: cs, imports: make(map[string]string)}

		d.pointers = make(map[uintptr]int)

		d.dump(reflect.ValueOf(arg))
		d.w.Write(newlineBytes)
		for k, v := range d.imports {
			imports[k] = v
		}
		structs = append(structs, b.String())
	}

	if cs.Pkg != "" {
		w.Write([]byte(fmt.Sprintf("package %s\n\n", cs.Pkg)))
	}

	if len(imports) > 0 {
		w.Write([]byte("import ("))
		for pkg, alias := range imports {
			if pkg == "" {
				continue
			}
			if alias == "" {
				w.Write([]byte("\n\t"))
				w.Write([]byte(fmt.Sprintf(`"%s"`, pkg)))
			} else {
				w.Write([]byte("\n\t"))
				w.Write([]byte(fmt.Sprintf(`%s "%s"`, alias, pkg)))
			}
		}
		w.Write([]byte("\n)\n\n"))
	}

	for _, s := range structs {
		w.Write([]byte(s))
	}
}

// Fdump formats and displays the passed arguments to io.Writer w.  It formats
// exactly the same as Dump.
func Fdump(w io.Writer, a ...interface{}) {
	fdump(&Config, w, a...)
}

// Sdump returns a string with the passed arguments formatted exactly the same
// as Dump.
func Sdump(a ...interface{}) string {
	var buf bytes.Buffer
	fdump(&Config, &buf, a...)
	return buf.String()
}

/*
Dump displays the passed parameters to standard out with newlines, customizable
indentation, and additional debug information such as complete types and all
pointer addresses used to indirect to the final value.  It provides the
following features over the built-in printing facilities provided by the fmt
package:

	* Pointers are dereferenced and followed
	* Circular data structures are detected and handled properly
	* Custom Stringer/error interfaces are optionally invoked, including
	  on unexported types
	* Custom types which only implement the Stringer/error interfaces via
	  a pointer receiver are optionally invoked when passing non-pointer
	  variables
	* Byte arrays and slices are dumped like the hexdump -C command which
	  includes offsets, byte values in hex, and ASCII output

The configuration options are controlled by an exported package global,
spew.Config.  See ConfigState for options documentation.

See Fdump if you would prefer dumping to an arbitrary io.Writer or Sdump to
get the formatted result as a string.
*/
func Dump(a ...interface{}) {
	fdump(&Config, os.Stdout, a...)
}
