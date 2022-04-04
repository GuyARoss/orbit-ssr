package webwrap

import "fmt"

func javascriptWebpack(bundleKey string, data []byte, doc htmlDoc) htmlDoc {
	doc.Head = append(doc.Head, fmt.Sprintf(`<script id="orbit_manifest" type="application/json">%s</script>`, data))
	doc.Head = append(doc.Head, `<script> const onLoadTasks = []; window.onload = (e) => { onLoadTasks.forEach(t => t(e))} </script>`)

	doc.Body = append(doc.Body, fmt.Sprintf(`<script id="orbit_bk" src="/p/%s.js"></script>`, bundleKey))

	return doc
}