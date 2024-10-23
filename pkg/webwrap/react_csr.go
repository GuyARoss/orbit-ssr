// Copyright (c) 2021 Guy A. Ross
// This source code is licensed under the GNU GPLv3 found in the
// license file in the root directory of this source tree.

package webwrap

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/GuyARoss/orbit/pkg/embedutils"
	"github.com/GuyARoss/orbit/pkg/experiments"
	"github.com/GuyARoss/orbit/pkg/jsparse"
	parseerror "github.com/GuyARoss/orbit/pkg/parse_error"
)

type ReactCSR struct {
	*BaseWebWrapper
	*BaseBundler
}

var ErrComponentExport = errors.New("prefer capitalization for jsx components")
var ErrInvalidComponent = errors.New("invalid jsx component")

func (s *ReactCSR) Apply(page jsparse.JSDocument) (map[string]jsparse.JSDocument, error) {
	if len(string(page.Name())) == 0 {
		return nil, ErrInvalidComponent
	}

	// react components should always be capitalized.
	if string(page.Name()[0]) != strings.ToUpper(string(page.Name()[0])) {
		return nil, ErrComponentExport
	}

	page.AddImport(&jsparse.ImportDependency{
		FinalStatement: "import ReactDOM from 'react-dom'",
		Type:           jsparse.ModuleImportType,
	})

	page.AddOther(fmt.Sprintf(
		"ReactDOM.render(<%s {...JSON.parse(document.getElementById('orbit_manifest').textContent)}/>, document.getElementById('%s_react_frame'))",
		page.Name(), page.Key()),
	)

	return map[string]jsparse.JSDocument{"normal": page}, nil
}

func (r *ReactCSR) VerifyRequirements() error {
	if experiments.GlobalExperimentalFeatures.PreferViteCompiler {
		// TODO: check for vite install
		return nil
	}

	webpackPath := fmt.Sprintf("%s%c%s%c%s", r.NodeModulesDir, os.PathSeparator, ".bin", os.PathSeparator, "webpack")

	// due to a "bug" with windows, it has an issue with shebang cmds, so we prefer the webpack.js file instead.
	if runtime.GOOS == "windows" {
		webpackPath = r.NodeModulesDir + "/webpack/bin/webpack.js"
	}

	_, err := os.Stat(webpackPath)
	if err != nil {
		return fmt.Errorf("node module not found: webpack. It is possible that you need to run `npm i` in your workspace directory to remedy this issue")
	}

	return nil
}

func (s *ReactCSR) Version() string {
	return "reactCSR"
}

func (s *ReactCSR) Stats() *WrapStats {
	switch {
	case experiments.GlobalExperimentalFeatures.PreferSWCCompiler:
		return &WrapStats{
			WebVersion: "React CSR",
			Bundler:    "swc",
		}

	case experiments.GlobalExperimentalFeatures.PreferViteCompiler:
		return &WrapStats{
			WebVersion: "React CSR",
			Bundler:    "vite",
		}
	default:
		return &WrapStats{
			WebVersion: "React CSR",
			Bundler:    "webpack",
		}
	}
}

func (s *ReactCSR) RequiredBodyDOMElements(ctx context.Context, cache *CacheDOMOpts) []string {
	if experiments.GlobalExperimentalFeatures.PreferViteCompiler {
		return []string{}
	}

	mode := ctx.Value(BundlerID).(string)

	uris := make([]string, 0)
	switch BundlerMode(mode) {
	case DevelopmentBundle:
		uris = append(uris, "https://unpkg.com/react/umd/react.development.js")
		uris = append(uris, "https://unpkg.com/react-dom/umd/react-dom.development.js")
	case ProductionBundle:
		uris = append(uris, "https://unpkg.com/react/umd/react.production.min.js")
		uris = append(uris, "https://unpkg.com/react-dom/umd/react-dom.production.min.js")
	}

	files, _ := cache.CacheWebRequest(uris)

	// currently these files are just paths to a directory to refer
	// to them on the dom, we need to convert them to <script> tags.
	for i, f := range files {
		files[i] = fmt.Sprintf(`<script src="%s"></script>`, f)
	}

	return files
}

func (b *ReactCSR) Setup(ctx context.Context, settings *BundleOpts) (*BundledResource, error) {
	page := jsparse.NewEmptyDocument()

	if experiments.GlobalExperimentalFeatures.PreferViteCompiler {
		bundleFilePath := fmt.Sprintf("%s/%s.jsx", b.PageOutputDir, settings.BundleKey)

		page.AddOther(fmt.Sprintf(`
import { defineConfig } from 'vite';
import { resolve } from 'path';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: '../../dist',
    manifest: true,
    rollupOptions: {      
      input: resolve(__dirname, "./%s.jsx"),
      output: {
        entryFileNames: '[name].js',
        chunkFileNames: '[name].js',
        assetFileNames: '[name].[ext]'
      }
    }    
  },
  optimizeDeps: {
    include: ['@emotion/styled'],
  },
});
`, settings.BundleKey))

		return &BundledResource{
			BundleOpFileDescriptor: map[string]string{"normal": bundleFilePath},
			Configurators: []BundleConfigurator{
				{
					FilePath: fmt.Sprintf("%s/%s.config.js", b.PageOutputDir, settings.BundleKey),
					Page:     page,
				},
			},
		}, nil
	}

	page.AddImport(&jsparse.ImportDependency{
		FinalStatement: "const {merge} = require('webpack-merge')",
		Type:           jsparse.ModuleImportType,
	})

	switch {
	case experiments.GlobalExperimentalFeatures.PreferSWCCompiler:
		page.AddImport(&jsparse.ImportDependency{
			FinalStatement: "const baseConfig = require('../../assets/swc-base.config.js')",
			Type:           jsparse.ModuleImportType,
		})
	default:
		page.AddImport(&jsparse.ImportDependency{
			FinalStatement: "const baseConfig = require('../../assets/base.config.js')",
			Type:           jsparse.ModuleImportType,
		})
	}

	outputFileName := fmt.Sprintf("%s.js", settings.BundleKey)

	bundleFilePath := fmt.Sprintf("%s/%s.js", b.PageOutputDir, settings.BundleKey)

	page.AddOther(fmt.Sprintf(`module.exports = merge(baseConfig, {
		entry: ['./%s'],
		mode: '%s',
		output: {
			filename: '%s'
		},
	})`, bundleFilePath, string(b.Mode), outputFileName))

	return &BundledResource{
		BundleOpFileDescriptor: map[string]string{"normal": bundleFilePath},
		Configurators: []BundleConfigurator{
			{
				FilePath: fmt.Sprintf("%s/%s.config.js", b.PageOutputDir, settings.BundleKey),
				Page:     page,
			},
		},
	}, nil
}

func (b *ReactCSR) Bundle(configuratorFilePath string, filePath string) error {
	if experiments.GlobalExperimentalFeatures.PreferViteCompiler {
		cmd := exec.Command("vite", "build", ".orbit/base/pages", "--config", configuratorFilePath)
		_, err := cmd.Output()

		if err != nil {
			b.Logger.Warn(fmt.Sprintf(`invalid pack: vite '%s'`, err))
			return parseerror.New("failed to bundle, this could denote a syntax error", filePath)
		}
		return nil
	}

	webpackPath := fmt.Sprintf("%s%c%s%c%s", b.NodeModulesDir, os.PathSeparator, ".bin", os.PathSeparator, "webpack")

	// due to a "bug" with windows, it has an issue with shebang cmds, so we prefer the webpack.js file instead.
	if runtime.GOOS == "windows" {
		webpackPath = b.NodeModulesDir + "/webpack/bin/webpack.js"
	}

	cmd := exec.Command("node", webpackPath, "--config", configuratorFilePath)
	output, err := cmd.Output()

	if err != nil {
		b.Logger.Warn(fmt.Sprintf(`invalid pack: "node %s --config %s"\n "%s"`, webpackPath, configuratorFilePath, string(output)))
		return parseerror.New("failed to bundle, this could denote a syntax error", filePath)
	}

	return nil
}

func (b *ReactCSR) HydrationFile() []embedutils.FileReader {
	files, err := embedFiles.ReadDir("embed")
	if err != nil {
		return nil
	}

	for _, file := range files {
		if strings.Contains(file.Name(), "react_csr.go") {
			return []embedutils.FileReader{&embedFileReader{fileName: file.Name()}}
		}
	}
	return nil
}

func (b *ReactCSR) DoesSatisfyConstraints(page jsparse.JSDocument) bool {
	return page.Extension() == "jsx" && page.DefaultExport() != nil
}

func NewReactCSR(bundler *BaseBundler) *ReactCSR {
	return &ReactCSR{
		BaseBundler: bundler,
	}
}
