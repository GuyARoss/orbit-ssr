// Copyright (c) 2021 Guy A. Ross
// This source code is licensed under the GNU GPLv3 found in the
// license file in the root directory of this source tree.

package jsparse

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestFormatImportLine(t *testing.T) {
	tt := []struct {
		i string
		o string
	}{
		{"import React from 'react'", "import React from 'react'"},
		{"import Thing2 from './thing2'", "import Thing2 from '../../../thing/thing2.jsx'"},
		{"import { withMemo } from 'react-thing';", "import { withMemo } from 'react-thing';"},
		{"import React from '../react'", "import React from '../../../test/react.jsx'"},
		{`import React from "../react"`, "import React from '../../../test/react.jsx'"},
		{"import { tool } from '../tools/test'", "import { tool } from '../../../test/tools/test.jsx'"},
		{"import { tool } from '../tools/test.js'", "import { tool } from '../../../test/tools/test.js'"},
		{"import 'thing.css'", "import 'thing.css'"},
	}

	p := DefaultJSDocument{webDir: "test", pageDir: "./thing/apple.js"}

	for i, c := range tt {
		got := p.formatImportLine(c.i)

		if c.o != got.FinalStatement {
			t.Errorf("(%d) expected %s got %s \n", i, c.o, got.FinalStatement)
		}
	}
}

func TestFormatImportLine_Index(t *testing.T) {
	dir := t.TempDir() + "/thing"
	err := os.Mkdir(dir+"/", 0777)
	if err != nil {
		t.Error("mkdir failed", err)
		return
	}

	f, err := os.Create(dir + "/index.js")
	if err != nil {
		t.Error("create file failed", err)
		return
	}
	f.Close()

	p := DefaultJSDocument{webDir: "", pageDir: "./thing/apple.js"}
	got := p.formatImportLine(fmt.Sprintf("import Thing from '%s'", dir))
	expect := fmt.Sprintf("import Thing from '../../..//%s/index.jsx'", dir)

	if got.FinalStatement != expect {
		t.Errorf("expected '%s' got '%s'", expect, got.FinalStatement)
	}
}

func TestWriteFile(t *testing.T) {
	path := t.TempDir() + "/thing"

	d := NewEmptyDocument()
	d.imports = append(d.imports, &ImportDependency{
		FinalStatement: "import { useMemo } from 'react'",
	}, &ImportDependency{
		FinalStatement: "import Thing from 'thing'",
	})

	d.AddOther("function working() {}")

	jsSwitch := NewSwitch("thing")
	jsSwitch.Add(JSString, "apple", "break;")
	jsSwitch.Add(JSString, "banana", "break;")

	d.AddSerializable(jsSwitch)

	err := d.WriteFile(path)
	if err != nil {
		t.Errorf("should successfully create a file without errors")
	}

	_, err = ioutil.ReadFile(path)
	if err != nil {
		t.Errorf("file should read successfully")
	}
}

func TestKey(t *testing.T) {
	d := NewEmptyDocument()

	if k := d.Key(); len(k) == 0 || strings.Contains(k, "-") {
		t.Errorf("invalid key created ")
	}
}

func TestVerifyPath(t *testing.T) {
	var tt = []struct {
		i string
		o string
	}{
		{"./thing.jsx", "./thing.jsx"},
		{".thing.jsx", "./thing.jsx"},
		{"/cake.jsx", "./cake.jsx"},
		{"cake.jsx", "./cake.jsx"},
		{"../../cake.jsx", "../../cake.jsx"},
	}

	for i, d := range tt {
		got := verifyPath(d.i)
		if got != d.o {
			t.Errorf("(%d) expected '%s' got '%s'", i, d.o, got)
		}

	}
}

func TestParseInformalExportDefault(t *testing.T) {
	p := &DefaultJSDocument{
		other: []string{},
		scope: map[string]*JsDocumentScope{},
	}
	p.parseInformalExportDefault("file_path", "export default Thing(Thing2)")
}

func TestTokenizeCommentString(t *testing.T) {
	var tt = []struct {
		i        string
		o        DefaultJSDocument
		lineData string
	}{
		{`"http://site.com"`, DefaultJSDocument{
			extension: "jsx",
			other:     []string{""},
		}, `"http://site.com"`},
	}

	for _, d := range tt {
		cdoc := NewEmptyDocument()
		_, err := cdoc.tokenizeLine(context.TODO(), "", d.i)

		if err != nil {
			t.Errorf("error not expected %s", err)
		}

		if cdoc.other[0] != d.lineData {
			t.Errorf("expected '%s' got '%s'", d.lineData, cdoc.other[0])
		}
	}
}

func TestTokenizeLine(t *testing.T) {
	var tt = []struct {
		i          string
		o          DefaultJSDocument
		path       string
		exportName string
	}{
		{"import Thing from 'thing'", DefaultJSDocument{
			imports: []*ImportDependency{
				{"import Thing from 'thing'", "", ModuleImportType},
			},
		}, "", ""},
		{"// some random text", DefaultJSDocument{
			extension: "jsx",
			other:     []string{},
		}, "", ""},
		{"// import thing from 'thing'", DefaultJSDocument{
			extension: "jsx",
			other:     []string{},
		}, "", ""},
		{"some random text // import thing from 'thing'", DefaultJSDocument{
			extension: "jsx",
			other:     []string{"some random text"},
		}, "", ""},
		{"export default Thing", DefaultJSDocument{
			extension: "jsx",
		}, "", "Thing"},
		{"export default HOCSomething(Component)", DefaultJSDocument{
			extension: "jsx",
			other:     []string{"const ThisThing = HOCSomething(Component)"},
		}, "this-thing", "ThisThing"},
		{"", DefaultJSDocument{
			other:     []string{},
			extension: "jsx",
		}, "", ""},
		{"export default () => (<> </>)", DefaultJSDocument{
			extension: "jsx",
			other:     []string{"const SomethingEasy = () => (<> </>)"},
		}, "something_easy", "SomethingEasy"},
		{"const thing = `//cat", DefaultJSDocument{
			extension: "jsx",
			other:     []string{"const thing = `//cat"},
		}, "", ""},
	}

	for i, d := range tt {
		cdoc := NewEmptyDocument()
		_, got := cdoc.tokenizeLine(context.TODO(), d.path, d.i)

		if got != nil {
			t.Error("did not expect error during line tokenization")
			continue
		}

		if cdoc.defaultExport.Name != d.exportName {
			t.Errorf("(%d) expected name %s got %s", i, d.exportName, cdoc.defaultExport.Name)
			continue
		}

		if len(cdoc.imports) != len(d.o.imports) {
			t.Errorf("(%d) import missmatch expected '%d' got '%d'", i, len(d.o.imports), len(cdoc.imports))
			continue
		}

		if len(cdoc.other) != len(d.o.other) {
			fmt.Println(i, len(cdoc.other), len(d.o.other), cdoc.other, d.o.other)
			t.Errorf("(%d) other missmatch expected '%d' got '%d'", i, len(d.o.other), len(cdoc.other))
			continue
		}

		cdoc = &DefaultJSDocument{}
	}
}

func TestTokenizeLine_DetectExport(t *testing.T) {
	cdoc := NewEmptyDocument()
	_, err := cdoc.tokenizeLine(context.TODO(), "", "function Thing() {}")
	if err != nil {
		t.Errorf("error occurred %s", err)
		return
	}

	_, err = cdoc.tokenizeLine(context.TODO(), "", "export default Thing")
	if err != nil {
		t.Errorf("error occurred %s", err)
		return
	}

	if cdoc.defaultExport == nil {
		t.Error("did not expect default export to be nil")
		return
	}

	if len(cdoc.defaultExport.Args) != 0 {
		t.Error("did not expect args to be present on resource default export")
		return
	}
}

func TestRemoveCenterOfToken(t *testing.T) {
	cases := []struct {
		l string
		t string
		e string
	}{
		{`import {withLayout} from "../components/layout"`, `"`, `import {withLayout} from ""`},
	}

	for _, c := range cases {
		got, _ := removeCenterOfToken(c.l, c.t)
		if got != c.e {
			t.Errorf("got '%s' expected '%s'", got, c.e)
		}
	}
}

func TestDefaultJSDocumentClone(t *testing.T) {
	d := NewDocument("somedir", "thing.jsx")
	newThing := d.Clone().(*DefaultJSDocument)

	if d.pageDir != newThing.pageDir {
		t.Errorf("clone page dir does not match")
	}
	if d.webDir != newThing.webDir {
		t.Errorf("clone webdir does not match")
	}
}

func TestJsDocSwitchSerialize(t *testing.T) {
	d := NewSwitch("thing")
	d.Add(JSString, "apple", "break;")
	d.Add(JSString, "banana", "break;")
	d.Add(JSString, "orange", "break;")
	d.Add(JSNumber, "12", "break;")

	got := d.Serialize()
	expected := "switch (thing) {case 'apple': { break; }case 'banana': { break; }case 'orange': { break; }case 12: { break; }}"

	if got != expected {
		t.Errorf("expected '%s' got '%s'", expected, got)
	}
}

func TestNewImportantDocument(t *testing.T) {
	d := NewImportDocument(&ImportDependency{}, &ImportDependency{})

	if len(d.imports) != 2 {
		t.Errorf("import length mismatch got '%d' expected '%d' ", len(d.imports), 2)
		return
	}
}

func TestJSDocArgListToString(t *testing.T) {
	argList := make(JSDocArgList, 2)
	argList[0] = "arg_sauce"
	argList[1] = "arg_juice"

	if argList.ToString() != "arg_sauce,arg_juice" {
		t.Errorf("expected 'arg_sauce,arg_juice' got '%s'", argList.ToString())
		return
	}
}

func TestParseComment(t *testing.T) {
	line := "// orbit:route /page/:id"

	doc := &DefaultJSDocument{}
	doc.parseComment(line, strings.Split(line, string(CommentToken)))

	if doc.orbitRoute != "/page/:id" {
		t.Errorf("incorrect orbit route '%s'", doc.orbitRoute)
	}
}
