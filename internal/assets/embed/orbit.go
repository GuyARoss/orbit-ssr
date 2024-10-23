package orbit

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
)

// Request is the standard request payload for the orbit page handler
// this is just a fancy wrapper around the http request & response that will also assist
// the rendering of bundled pages & incoming path slugs
type Request struct {
	RenderPage  func(page PageRender, data interface{})
	RenderPages func(data interface{}, pages ...PageRender)
	Request     *http.Request
	Response    http.ResponseWriter
	Slugs       map[string]string
}

// DefaultPage defines the standard behavior for a orbit page handler
type DefaultPage interface {
	Handle(*Request)
}

// htmlDoc represents a basic document model that will be rendered upon build request
type htmlDoc struct {
	Head []string
	Body []string
}

// render renders the document out to a single string
func (s *htmlDoc) render() string {
	return fmt.Sprintf(`
	<!doctype html>
	<html lang="en">
	<head>%s</head>
	<body>%s</body>
	</html>`, strings.Join(s.Head, ""), strings.Join(s.Body, ""))
}

func (s *htmlDoc) merge(doc *htmlDoc) *htmlDoc {
	s.Body = append(doc.Body, s.Body...)
	s.Head = append(doc.Head, s.Head...)

	return s
}

// parseStaticDocument attempts to find the specified document and return it as a string
func parseStaticDocument(path string) (string, error) {
	_, err := os.Stat(path)
	if !os.IsNotExist(err) {
		f, _ := ioutil.ReadFile(path)

		return string(f), nil
	}

	return "", fmt.Errorf("static document does not exist for '%s'", publicDir)
}

// build buildHTMLPages creates the htmldocument given data for orbits manifest and the page's
func buildHTMLPages(data []byte, pages ...PageRender) *htmlDoc {
	body := make([]string, 0)
	head := make([]string, 0)
	isWrapped := make(map[string]bool)

	for _, p := range pages {
		// if the page is of static origin, we first check to see if it exists on the file system
		// if it does, it will be applied to the current html document, rather than returned directly
		// this is to support the usage of static html within micro-frontends
		if staticResourceMap[p] {
			staticDocument, err := parseStaticDocument(fmt.Sprintf("%s%c%s", http.Dir(bundleDir), os.PathSeparator, p))
			if err == nil {
				body = append(body, innerHTML(string(staticDocument), "<body>", "</body>"))
				head = append(head, innerHTML(string(staticDocument), "<head>", "</head>"))
				continue
			}
		}

		// wrapping page content should only happen once as it just creates
		// the requirements for the specific web wrapper to work correctly
		pv := wrapDocRender[p]
		if pv != nil && !isWrapped[pv.version] {
			isWrapped[pv.version] = true

			for _, b := range pageDependencies[p] {
				head = append(head, b)
			}
		}
	}

	html := &htmlDoc{
		Head: head,
		Body: body,
	}

	ctx := context.Background()
	for _, p := range pages {
		if op := wrapDocRender[p]; op != nil {
			if useManifestConfig {
				html.Head = append(html.Head, fmt.Sprintf(`<link rel="stylesheet" href="/p/%s.css">`, string(p)))
			}

			html, ctx = op.fn(ctx, string(p), data, html)
		}
	}

	return html
}

// innerHTML is a utility function that assists with the parsing the content of html tags
// it does this by returning the subset of the two provided strings "subStart" & "subEnd"
func innerHTML(str string, subStart string, subEnd string) string {
	return strings.Split(strings.Join(strings.Split(str, subStart)[1:], ""), subEnd)[0]
}

// defaultHTMLDoc builds a standard html doc for orbit that also verifies the public directory
// if override data exits, then it will use that as a base for the HTML document
func defaultHTMLDoc(override string) *htmlDoc {
	base := &htmlDoc{Head: []string{`<meta charset="utf-8" />`}, Body: []string{}}

	// we allow some special operations on the dom for debugging, currently supporting:
	// - getting the contents of orbit manifest with the function "getManifest"
	if CurrentDevMode == DevBundleMode {
		base.Body = append(base.Body, `<script class="debug"> const getManifest = () => JSON.parse(document.getElementById("orbit_manifest").textContent) </script>`)
		base.Body = append(base.Body, `<script class="debug" src="/p/hotreload.js"> </script>`)
		base.Body = append(base.Body, fmt.Sprintf(`<script class="debug" id="debug_data" type="application/json">{ "hotReloadPort": %d }</script>`, hotReloadPort))
	}

	// the html override that will provide a basis for the default html doc
	if override != "" {
		base.Body = append(base.Body, innerHTML(string(override), "<body>", "</body>"))
		base.Head = append(base.Head, innerHTML(string(override), "<head>", "</head>"))
	}

	return base
}

// parseSlug will parse slugs from the incoming path provided initial slugKeys and return a map of the slugs
// in map[string]string form where the key is the slug name & value is the value as it appears in the path
func parseSlug(slugKeys map[int]string, path string) map[string]string {
	slugs := make(map[string]string)
	if len(slugKeys) > 0 {
		paths := strings.Split(path, "/")

		for idx, p := range paths {
			key := slugKeys[idx]
			if key != "" {
				slugs[key] = p
			}
		}
	}

	return slugs
}

// parsePathSlugs will parse initial slugs found in a path, these slugs can be identified with
// the "{" char prepended & "}" appended to the path/subpath e.g "/path/{thing}" will represent "thing" as a slug key.
func parsePathSlugs(path string) (map[int]string, string) {
	slugKeys := make(map[int]string)

	validInitial := make([]string, 0)
	slide := true
	if strings.Contains(path, "{") {
		paths := strings.Split(path, "/")
		for idx, p := range paths {
			if strings.Contains(p, "{") {
				slide = false
				slugKeys[idx] = p[1 : len(p)-1]
			}

			if slide {
				validInitial = append(validInitial, p)
			}
		}
	}

	finalPath := path
	if len(slugKeys) > 0 {
		finalPath = fmt.Sprintf("%s/", strings.Join(validInitial, "/"))
	}

	return slugKeys, finalPath
}

// muxHandle is used to inject the base mux handler behavior
type MuxHandler interface {
	HandleFunc(string, func(http.ResponseWriter, *http.Request))
	Handle(string, http.Handler)
	ServeHTTP(http.ResponseWriter, *http.Request)
}

type Serve struct {
	mux MuxHandler
	doc *htmlDoc
}

// HandleFunc attaches a handler to the specified pattern, this handler will be
// ran upon a match of the request path during an incoming http request.
func (s *Serve) HandleFunc(path string, handler func(c *Request)) {
	slugs, path := parsePathSlugs(path)

	s.mux.HandleFunc(path, func(rw http.ResponseWriter, r *http.Request) {
		requestSlugs := make(map[string]string)

		if len(slugs) > 0 {
			requestSlugs = parseSlug(slugs, r.URL.Path)

			if len(requestSlugs) != len(slugs) {
				rw.WriteHeader(http.StatusBadRequest)
				return
			}
		}

		renderPage := func(page PageRender, data interface{}) {
			if staticResourceMap[page] {
				if staticDocument, err := parseStaticDocument(fmt.Sprintf("%s%c%s", http.Dir(bundleDir), os.PathSeparator, page)); err == nil {
					rw.Write([]byte(staticDocument))
					return
				}
			}

			d, err := json.Marshal(data)
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}

			doc := buildHTMLPages(d, page)
			doc.merge(s.doc)

			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(doc.render()))
		}

		renderPages := func(data interface{}, pages ...PageRender) {
			// renderPage (single) has some optimizations for micro-frontends
			// that should be preferred over the generalized method
			if len(pages) == 1 {
				renderPage(pages[0], data)
				return
			}

			d, err := json.Marshal(data)
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}

			doc := buildHTMLPages(d, pages...)
			doc.merge(s.doc)

			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(doc.render()))
		}

		ctx := &Request{
			RenderPage:  renderPage,
			RenderPages: renderPages,
			Request:     r,
			Response:    rw,
			Slugs:       requestSlugs,
		}

		handler(ctx)
	})
}

// HandlePage attaches an orbit page to the specified pattern, this handler will be
// ran upon a match of the request path during an incoming http request
func (s *Serve) HandlePage(path string, dp DefaultPage) {
	s.HandleFunc(path, dp.Handle)
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(status)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// setupMuxRequirements creates the required mux handlers for orbit, these include
// - fileserver for the bundle directory bound to the "/p/" directory
func (s *Serve) setupMuxRequirements() *Serve {
	pool := sync.Pool{
		New: func() interface{} {
			w := gzip.NewWriter(ioutil.Discard)
			return w
		},
	}

	s.mux.HandleFunc("/p/", func(w http.ResponseWriter, r *http.Request) {
		// TODO(guy): allow these cache policies to be overwritten
		switch CurrentDevMode {
		case DevBundleMode:
			w.Header().Set("Cache-Control", "no-cache, no-store, max-age=0, must-revalidate")
		case ProdBundleMode:
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}

		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			w.Header().Set("Content-Encoding", "gzip")

			gz := pool.Get().(*gzip.Writer)
			defer pool.Put(gz)

			gz.Reset(w)
			defer gz.Close()

			http.StripPrefix("/p/", http.FileServer(http.Dir(bundleDir))).ServeHTTP(&gzipResponseWriter{ResponseWriter: w, Writer: gz}, r)
			return
		}

		http.StripPrefix("/p/", http.FileServer(http.Dir(bundleDir))).ServeHTTP(w, r)
	})

	return s
}

// Serve returns the mux server
func (s *Serve) Serve() MuxHandler {
	return s.mux
}

func setupDoc() *htmlDoc {
	html := ""

	_, err := os.Stat(publicDir)
	if !os.IsNotExist(err) {
		// invalid files should already be skipped, besides that, an empty []byte should suffice.
		data, _ := ioutil.ReadFile(publicDir)
		html = string(data)
	}

	return defaultHTMLDoc(html)
}

func New() (*Serve, error) {
	for _, sfn := range serverStartupTasks {
		sfn()
	}

	return (&Serve{
		mux: http.NewServeMux(),
		doc: setupDoc(),
	}).setupMuxRequirements(), nil
}
