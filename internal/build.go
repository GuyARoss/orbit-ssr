// Copyright (c) 2021 Guy A. Ross
// This source code is licensed under the GNU GPLv3 found in the
// license file in the root directory of this source tree.

package internal

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/GuyARoss/orbit/internal/assets"
	"github.com/GuyARoss/orbit/internal/libout"
	"github.com/GuyARoss/orbit/internal/srcpack"
	"github.com/GuyARoss/orbit/pkg/experiments"
	"github.com/GuyARoss/orbit/pkg/fsutils"
	"github.com/GuyARoss/orbit/pkg/log"
	"github.com/GuyARoss/orbit/pkg/webwrap"
)

type BuildOpts struct {
	Packname       string
	OutDir         string
	WebDir         string
	Mode           string
	NodeModulePath string
	PublicDir      string
	Dirs           []string
	NoWrite        bool
}

func Build(opts *BuildOpts) (srcpack.PackedComponentList, error) {
	ats, err := assets.AssetKeys()
	if err != nil {
		return nil, err
	}

	fileAssets := make([]fs.DirEntry, 0)
	switch {
	case experiments.GlobalExperimentalFeatures.PreferViteCompiler:
		fileAssets = append(fileAssets, ats.AssetEntry(assets.VitePackConfig))
	default:
		fileAssets = append(fileAssets, ats.AssetEntry(assets.WebPackConfig),
			ats.AssetEntry(assets.SSRProtoFile),
			ats.AssetEntry(assets.JsWebPackConfig),
			ats.AssetEntry(assets.WebPackSWCConfig))
	}

	s := &FileStructure{
		PackageName: opts.Packname,
		OutDir:      opts.OutDir,
		Assets:      fileAssets,
		Mkdirs:      opts.Dirs,
	}

	if err = s.Make(); err != nil {
		return nil, err
	}

	// TODO(pages): remove hardcode pages path
	pageFiles := fsutils.DirFiles(fmt.Sprintf("%s/pages", opts.WebDir))

	c, err := CachedEnvFromFile(fmt.Sprintf("%s/%s/orb_env.go", opts.OutDir, opts.Packname))
	if err != nil && !errors.Is(err, os.ErrNotExist) && !opts.NoWrite {
		return nil, err
	}

	packer := srcpack.NewDefaultPacker(log.NewDefaultLogger(), &srcpack.DefaultPackerOpts{
		WebDir:           opts.WebDir,
		BundlerMode:      opts.Mode,
		NodeModuleDir:    opts.NodeModulePath,
		CachedBundleKeys: c,
	})

	components, err := packer.PackMany(pageFiles)
	if err != nil {
		return nil, err
	}

	bg := libout.New(&libout.BundleGroupOpts{
		PackageName:   opts.Packname,
		BaseBundleOut: ".orbit/dist",
		BundleMode:    opts.Mode,
		PublicDir:     opts.PublicDir,
	})

	ctx := context.Background()
	ctx = context.WithValue(ctx, webwrap.BundlerID, opts.Mode)

	if err = bg.AcceptComponents(ctx, components, &webwrap.CacheDOMOpts{
		CacheDir:  ".orbit/dist",
		WebPrefix: "/p/",
	}); err != nil {
		return nil, err
	}

	if !opts.NoWrite {
		err = bg.WriteLibout(libout.NewGOLibout(
			ats.AssetKey(assets.Tests),
			ats.AssetKey(assets.PrimaryPackage),
		), &libout.FilePathOpts{
			TestFile: fmt.Sprintf("%s/%s/orb_test.go", opts.OutDir, opts.Packname),
			EnvFile:  fmt.Sprintf("%s/%s/orb_env.go", opts.OutDir, opts.Packname),
			HTTPFile: fmt.Sprintf("%s/%s/orb_http.go", opts.OutDir, opts.Packname),
		})

		if err != nil {
			return nil, err
		}
	}

	return components, nil
}
