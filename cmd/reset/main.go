// Command reset generates Reset() methods for structs marked with "// generate:reset" comment.
package main

import (
	"bytes"
	"fmt"
	"go/types"
	"log"

	"golang.org/x/tools/go/packages"
)

func main() {
	root, err := findModuleRoot()
	if err != nil {
		log.Fatal(err)
	}

	processPackages(root)
}

func emitResetMethod(w *codeWriter, im *importManager, pkg *packages.Package, t targetStruct) error {
	st, ok := t.Named.Underlying().(*types.Struct)
	if !ok {
		return fmt.Errorf("%s is not a struct type", t.Name)
	}

	recv := "x"
	w.line(0, fmt.Sprintf("func (%s *%s) Reset() {", recv, t.Name))
	w.line(1, fmt.Sprintf("if %s == nil {", recv))
	w.line(2, "return")
	w.line(1, "}")
	w.line(0, "")

	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		fieldExpr := recv + "." + f.Name()
		emitResetForExpr(w, im, pkg.PkgPath, fieldExpr, f.Type())
	}

	w.line(0, "}")
	return nil
}

type codeWriter struct {
	buf *bytes.Buffer
}

func (w *codeWriter) ensure() {
	if w.buf == nil {
		w.buf = &bytes.Buffer{}
	}
}

func (w *codeWriter) line(indent int, s string) {
	w.ensure()
	for i := 0; i < indent; i++ {
		w.buf.WriteByte('\t')
	}
	w.buf.WriteString(s)
	w.buf.WriteByte('\n')
}

func emitResetForExpr(w *codeWriter, im *importManager, localPkgPath, expr string, t types.Type) {
	u := t.Underlying()

	switch tt := u.(type) {
	case *types.Basic:
		w.line(1, fmt.Sprintf("%s = %s", expr, zeroBasic(tt)))
		return

	case *types.Slice:
		w.line(1, fmt.Sprintf("%s = %s[:0]", expr, expr))
		return

	case *types.Map:
		w.line(1, fmt.Sprintf("clear(%s)", expr))
		return

	case *types.Pointer:
		ptrExpr := expr
		w.line(1, fmt.Sprintf("if %s != nil {", ptrExpr))
		emitResetThroughPointer(w, im, localPkgPath, ptrExpr, tt.Elem())
		w.line(1, "}")
		return

	case *types.Struct:
		if hasResetMethod(types.NewPointer(t)) {
			w.line(1, fmt.Sprintf("(&%s).Reset()", expr))
			return
		}
		if hasResetMethod(t) {
			w.line(1, fmt.Sprintf("%s.Reset()", expr))
			return
		}

		if st, ok := accessibleStruct(t, localPkgPath); ok {
			for i := 0; i < st.NumFields(); i++ {
				f := st.Field(i)
				emitResetForExpr(w, im, localPkgPath, expr+"."+f.Name(), f.Type())
			}
			return
		}

		w.line(1, fmt.Sprintf("%s = %s{}", expr, im.typeString(t)))
		return

	case *types.Array:
		w.line(1, fmt.Sprintf("%s = %s{}", expr, im.typeString(t)))
		return

	default:
		w.line(1, fmt.Sprintf("%s = %s", expr, zeroExpr(im, t)))
		return
	}
}

func emitResetThroughPointer(w *codeWriter, im *importManager, localPkgPath, ptrExpr string, elem types.Type) {
	u := elem.Underlying()

	switch tt := u.(type) {
	case *types.Basic:
		w.line(2, fmt.Sprintf("*%s = %s", ptrExpr, zeroBasic(tt)))
		return

	case *types.Slice:
		w.line(2, fmt.Sprintf("*%s = (*%s)[:0]", ptrExpr, ptrExpr))
		return

	case *types.Map:
		w.line(2, fmt.Sprintf("clear(*%s)", ptrExpr))
		return

	case *types.Pointer:
		w.line(2, fmt.Sprintf("if *%s != nil {", ptrExpr))
		emitResetThroughPointer(w, im, localPkgPath, "*"+ptrExpr, tt.Elem())
		w.line(2, "}")
		return

	case *types.Struct:
		if hasResetMethod(types.NewPointer(elem)) {
			w.line(2, fmt.Sprintf("%s.Reset()", ptrExpr))
			return
		}

		if st, ok := accessibleStruct(elem, localPkgPath); ok {
			for i := 0; i < st.NumFields(); i++ {
				f := st.Field(i)
				emitResetForExpr(w, im, localPkgPath, ptrExpr+"."+f.Name(), f.Type())
			}
			return
		}

		w.line(2, fmt.Sprintf("*%s = %s{}", ptrExpr, im.typeString(elem)))
		return

	case *types.Array:
		w.line(2, fmt.Sprintf("*%s = %s{}", ptrExpr, im.typeString(elem)))
		return

	default:
		w.line(2, fmt.Sprintf("*%s = %s", ptrExpr, zeroExpr(im, elem)))
		return
	}
}

func accessibleStruct(t types.Type, localPkgPath string) (*types.Struct, bool) {
	switch tt := t.(type) {
	case *types.Struct:
		return tt, true
	case *types.Named:
		st, ok := tt.Underlying().(*types.Struct)
		if !ok {
			return nil, false
		}
		if tt.Obj() != nil && tt.Obj().Pkg() != nil && tt.Obj().Pkg().Path() == localPkgPath {
			return st, true
		}
		return nil, false
	default:
		return nil, false
	}
}
