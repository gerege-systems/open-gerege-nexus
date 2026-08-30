/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The exported surface of this package, written down.
 *
 * `docs/MODULES.md` promises that `pkg/nexus` does not break inside a major
 * version. A promise a build cannot check is a promise somebody keeps until the
 * afternoon they are in a hurry, and the way this one would be broken is not by
 * anybody deciding to break it: it is by renaming a parameter during a
 * refactor, or by adding a method to an interface that a distribution's own
 * type implements, neither of which fails a single test in this repository.
 * Every distribution's build is where it would fail instead, days later.
 *
 * So the surface is a golden file. Changing it is allowed and often right —
 * this is not a freeze — but it cannot happen by accident, and the diff in a
 * review says exactly what the ecosystem's contract gained or lost.
 *
 *	go test ./pkg/nexus -update    # after a deliberate API change
 */

package nexus_test

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/api.txt from the current package")

func TestTheExportedAPIIsTheOneOnRecord(t *testing.T) {
	got := renderAPI(t)
	golden := filepath.Join("testdata", "api.txt")

	if *update {
		if err := os.MkdirAll("testdata", 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Log("wrote", golden)
		return
	}

	want, err := os.ReadFile(golden) // #nosec G304 -- a fixed path beside the test
	if err != nil {
		t.Fatalf("read the recorded API: %v", err)
	}
	if got != string(want) {
		t.Errorf(`the exported API of pkg/nexus is not what is on record.

Every distribution repository compiles against this package, so a change here
is a change to the ecosystem's contract. If it is deliberate, re-record it with

    go test ./pkg/nexus -update

and say in the commit message what a caller has to do about it. If it is not
deliberate, this is the accident the file exists to catch.

%s`, diff(string(want), got))
	}
}

// renderAPI prints every exported declaration, one per line, sorted.
//
// Signatures are printed from the AST rather than described by hand, so a
// parameter type that changes shows up even when the name and arity do not.
func renderAPI(t *testing.T) string {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse the package: %v", err)
	}

	var lines []string
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				lines = append(lines, declarationLines(fset, decl)...)
			}
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n") + "\n"
}

func declarationLines(fset *token.FileSet, decl ast.Decl) []string {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if !d.Name.IsExported() {
			return nil
		}
		receiver := ""
		if d.Recv != nil && len(d.Recv.List) > 0 {
			// A method on an unexported type is not reachable from outside.
			name := print(fset, d.Recv.List[0].Type)
			if !ast.IsExported(strings.TrimPrefix(name, "*")) {
				return nil
			}
			receiver = "(" + name + ") "
		}
		return []string{"func " + receiver + d.Name.Name + print(fset, d.Type)[len("func"):]}

	case *ast.GenDecl:
		var out []string
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				if !s.Name.IsExported() {
					continue
				}
				out = append(out, typeLines(fset, s)...)
			case *ast.ValueSpec:
				for _, name := range s.Names {
					if name.IsExported() {
						out = append(out, fmt.Sprintf("%s %s", d.Tok, name.Name))
					}
				}
			}
		}
		return out
	}
	return nil
}

// typeLines prints a type, and for structs and interfaces every exported field
// or method with it: adding a method to an exported interface breaks every
// outside implementation of it, and that is precisely the change this file has
// to make visible.
func typeLines(fset *token.FileSet, s *ast.TypeSpec) []string {
	alias := ""
	if s.Assign.IsValid() {
		alias = "= "
	}
	head := fmt.Sprintf("type %s %s%s", s.Name.Name, alias, kindOf(s.Type))
	lines := []string{head}

	switch t := s.Type.(type) {
	case *ast.StructType:
		for _, field := range t.Fields.List {
			for _, name := range field.Names {
				if name.IsExported() {
					lines = append(lines, fmt.Sprintf("  %s.%s %s", s.Name.Name, name.Name, print(fset, field.Type)))
				}
			}
		}
	case *ast.InterfaceType:
		for _, method := range t.Methods.List {
			for _, name := range method.Names {
				if name.IsExported() {
					lines = append(lines, fmt.Sprintf("  %s.%s%s", s.Name.Name, name.Name,
						strings.TrimPrefix(print(fset, method.Type), "func")))
				}
			}
		}
	default:
		lines[0] = fmt.Sprintf("type %s %s%s", s.Name.Name, alias, print(fset, s.Type))
	}
	return lines
}

func kindOf(expr ast.Expr) string {
	switch expr.(type) {
	case *ast.StructType:
		return "struct"
	case *ast.InterfaceType:
		return "interface"
	default:
		return ""
	}
}

func print(fset *token.FileSet, node ast.Node) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, node); err != nil {
		return "<unprintable>"
	}
	return strings.Join(strings.Fields(buf.String()), " ")
}

// diff reports the lines that moved, which is all a reviewer needs from a file
// that is one declaration per line.
func diff(want, got string) string {
	inWant := map[string]bool{}
	for _, line := range strings.Split(want, "\n") {
		inWant[line] = true
	}
	inGot := map[string]bool{}
	for _, line := range strings.Split(got, "\n") {
		inGot[line] = true
	}

	var out strings.Builder
	for _, line := range strings.Split(want, "\n") {
		if line != "" && !inGot[line] {
			fmt.Fprintf(&out, "removed: %s\n", line)
		}
	}
	for _, line := range strings.Split(got, "\n") {
		if line != "" && !inWant[line] {
			fmt.Fprintf(&out, "added:   %s\n", line)
		}
	}
	return out.String()
}
