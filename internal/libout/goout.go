package libout

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/GuyARoss/orbit/internal/assets"
)

type GOLibFile struct {
	PackageName string
	Body        string
}

func (l *GOLibFile) Write(path string) error {
	out := strings.Builder{}
	out.WriteString(fmt.Sprintf("package %s\n\n", l.PackageName))
	out.WriteString(l.Body)

	if _, err := os.Stat(path); err != nil {
		_, err := os.Create(path)
		if err != nil {
			return err
		}
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)

	if err != nil {
		return err
	}
	defer f.Close()

	err = f.Truncate(0)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "%s", out.String())
	if err != nil {
		return err
	}

	return nil
}

type GOLibout struct {
	testFile fs.DirEntry
	httpFile fs.DirEntry
}

func parseFile(entry fs.DirEntry) (string, error) {
	file, err := assets.OpenFile(entry)

	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	out := strings.Builder{}

	for scanner.Scan() {
		line := scanner.Text()

		// we don't want the package to be contained within this
		if strings.Contains(line, "package") {
			continue
		}
		out.WriteString(fmt.Sprintf("%s\n", line))
	}

	return out.String(), nil
}

func (l *GOLibout) TestFile(packageName string) (LiboutFile, error) {
	body, err := parseFile(l.testFile)
	if err != nil {
		return nil, err
	}

	return &GOLibFile{
		PackageName: packageName,
		Body:        body,
	}, nil
}

func (l *GOLibout) HTTPFile(packageName string) (LiboutFile, error) {
	body, err := parseFile(l.httpFile)
	if err != nil {
		return nil, err
	}

	return &GOLibFile{
		PackageName: packageName,
		Body:        body,
	}, nil
}

func (l *GOLibout) EnvFile(bg *BundleGroup) (LiboutFile, error) {
	out := strings.Builder{}

	for rd, v := range bg.componentBodyMap {
		out.WriteString("\n")
		out.WriteString(fmt.Sprintf(`var %s = []string{`, rd))
		out.WriteString("\n")

		for _, b := range v {
			out.WriteString(fmt.Sprintf("`%s`,", b))
			out.WriteString("\n")
		}

		out.WriteString("}")
		out.WriteString("\n")
	}

	if len(bg.BaseBundleOut) > 0 {
		out.WriteString("\n")
		out.WriteString(fmt.Sprintf(`var bundleDir string = "%s"`, bg.BaseBundleOut))
		out.WriteString("\n")
	}

	if len(bg.PublicDir) > 0 {
		out.WriteString("\n")
		out.WriteString(fmt.Sprintf(`var publicDir string = "%s"`, bg.PublicDir))
		out.WriteString("\n")
	}

	out.WriteString("type PageRender string\n\n")

	for idx, p := range bg.pages {
		if idx == 0 {
			out.WriteString("const ( \n")
		}

		if !strings.Contains(p.name, "Page") {
			p.name = fmt.Sprintf("%sPage", p.name)
		}

		out.WriteString(fmt.Sprintf(`	%s PageRender = "%s"`, p.name, p.bundleKey))
		out.WriteString("\n")

		if idx == len(bg.pages)-1 {
			out.WriteString(")\n")
		}
	}

	out.WriteString("\n")
	out.WriteString(`var wrapBody = map[PageRender][]string{`)
	out.WriteString("\n")

	for _, p := range bg.pages {
		out.WriteString(fmt.Sprintf(`	%s: %s,`, p.name, p.wrapVersion))
		out.WriteString("\n")
	}

	out.WriteString("}")

	out.WriteString("\n")

	out.WriteString(`
type BundleMode int32

const (
	DevBundleMode  BundleMode = 0
	ProdBundleMode BundleMode = 1
)

`)

	if bg.BundleMode == "production" {
		out.WriteString("var CurrentDevMode BundleMode = ProdBundleMode")
	} else {
		out.WriteString("var CurrentDevMode BundleMode = DevBundleMode")
	}

	return &GOLibFile{
		PackageName: bg.PackageName,
		Body:        out.String(),
	}, nil
}

func NewGOLibout(testFile fs.DirEntry, httpFile fs.DirEntry) Libout {
	return &GOLibout{
		testFile: testFile,
		httpFile: httpFile,
	}
}