package orbit

import (
	"context"
	"fmt"
)

var bundleDir string = ".orbit/dist"

func deleteMeThing(bundleKey string, data []byte, doc htmlDoc) htmlDoc {
	doc.Head = append(doc.Head, fmt.Sprintf(`<script id="orbit_manifest" type="application/json">%s</script>`, data))
	doc.Body = append(doc.Body, fmt.Sprintf(`<script class="orbit_bk" src="/p/%s.js">`, bundleKey))

	return doc
}

var staticResourceMap = map[PageRender]bool{}

var pageDependencies = map[PageRender][]string{}

type PageRender string

var publicDir string = "./public/index.html"

type BundleMode int32

const (
	DevBundleMode  BundleMode = 0
	ProdBundleMode BundleMode = 1
)

var CurrentDevMode BundleMode

var hotReloadPort = 1000

var serverStartupTasks = []func(){}

type DocumentRenderer struct {
	fn      func(context.Context, string, []byte, *htmlDoc) (*htmlDoc, context.Context)
	version string
}

var wrapDocRender = map[PageRender]*DocumentRenderer{}

var useManifestConfig bool = false
