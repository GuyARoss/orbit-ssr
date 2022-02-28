package internal

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/GuyARoss/orbit/internal/assets"
	"github.com/GuyARoss/orbit/internal/srcpack"
	"github.com/GuyARoss/orbit/pkg/bundler"
	"github.com/GuyARoss/orbit/pkg/fs"
	"github.com/GuyARoss/orbit/pkg/jsparse"
	"github.com/GuyARoss/orbit/pkg/libgen"
	"github.com/GuyARoss/orbit/pkg/log"
	"github.com/GuyARoss/orbit/pkg/runtimeanalytics"
	webwrapper "github.com/GuyARoss/orbit/pkg/web_wrapper"
)

type AutoGenPages struct {
	BundleData *libgen.GoFile
	Master     *libgen.GoFile
	Test       *libgen.GoFile

	Pages       []*srcpack.Component
	OutDir      string
	PackageName string
}

type GenPagesSettings struct {
	PackageName    string
	OutDir         string
	WebDir         string
	BundlerMode    string
	NodeModulePath string
	PublicDir      string
	UseDebug       bool
}

func (s *GenPagesSettings) DefaultPacker(ctx context.Context, logger log.Logger) (context.Context, *srcpack.Packer) {
	return ctx, &srcpack.Packer{
		Bundler: &bundler.WebPackBundler{
			BaseBundler: &bundler.BaseBundler{
				Mode:           bundler.BundlerMode(s.BundlerMode),
				WebDir:         s.WebDir,
				PageOutputDir:  ".orbit/base/pages",
				NodeModulesDir: s.NodeModulePath,
				Logger:         logger,
			},
		},
		WebDir:           s.WebDir,
		JsParser:         &jsparse.JSFileParser{},
		ValidWebWrappers: webwrapper.NewActiveMap(),
		Logger:           logger,
	}
}

func (s *GenPagesSettings) PackWebDir(ctx context.Context, logger log.Logger) (*AutoGenPages, error) {
	_, settings := s.DefaultPacker(ctx, logger)

	ats, err := assets.AssetKeys()
	if err != nil {
		return nil, err
	}

	pageFiles := fs.DirFiles(fmt.Sprintf("%s/pages", s.WebDir))

	err = assets.WriteFile(".orbit/assets", ats.AssetKey(assets.WebPackConfig))
	if err != nil {
		return nil, err
	}

	comps, err := settings.PackMany(pageFiles)
	if err != nil {
		return nil, err
	}

	lg := libgen.New(&libgen.BundleGroupOpts{
		PackageName:   s.PackageName,
		BaseBundleOut: ".orbit/dist",
		BundleMode:    string(s.BundlerMode),
		PublicDir:     s.PublicDir,
	})

	// initialize bundle fields for each of the root pages
	lg.AcceptComponents(ctx, comps, &webwrapper.CacheDOMOpts{
		CacheDir:  ".orbit/dist",
		WebPrefix: "/p/",
	})

	libStaticContent, err := libgen.ParseFile(ats.AssetKey(assets.PrimaryPackage))
	if err != nil {
		return nil, err
	}

	testsStaticContent, err := libgen.ParseFile(ats.AssetKey(assets.Tests))
	if err != nil {
		return nil, err
	}

	return &AutoGenPages{
		OutDir:      s.OutDir,
		BundleData:  lg.CreateBundleLib(),
		Master:      libgen.NewGoFile(s.PackageName, libStaticContent),
		Test:        libgen.NewGoFile(s.PackageName, testsStaticContent),
		Pages:       comps,
		PackageName: s.PackageName,
	}, nil
}

func (s *GenPagesSettings) Repack(p *srcpack.Component, hooks srcpack.PackHook) error {
	ra := &runtimeanalytics.RuntimeAnalytics{}
	ra.StartCapture()

	hooks.Pre(p.OriginalFilePath())

	r := p.Repack()

	hooks.Post(p.OriginalFilePath(), ra.StopCapture())

	return r
}

func (s *AutoGenPages) WriteOut() error {
	err := s.BundleData.WriteFile(fmt.Sprintf("%s/%s/orb_bundle.go", s.OutDir, s.PackageName))
	if err != nil {
		return err
	}
	err = s.Master.WriteFile(fmt.Sprintf("%s/%s/orb_master.go", s.OutDir, s.PackageName))
	if err != nil {
		return err
	}

	err = s.Test.WriteFile(fmt.Sprintf("%s/%s/orb_test.go", s.OutDir, s.PackageName))
	if err != nil {
		return err
	}

	return nil
}

func (s *GenPagesSettings) CleanPathing() error {
	err := os.RemoveAll(".orbit/")
	if err != nil {
		return err
	}

	if !fs.DoesDirExist(fmt.Sprintf("%s/%s", s.OutDir, s.PackageName)) {
		err := os.Mkdir(fmt.Sprintf("%s/%s", s.OutDir, s.PackageName), os.ModePerm)
		if err != nil {
			return err
		}
	}

	dirs := []string{".orbit", ".orbit/base", ".orbit/base/pages", ".orbit/dist", ".orbit/assets"}
	for _, dir := range dirs {
		_, err := os.Stat(dir)
		if errors.Is(err, os.ErrNotExist) {
			err := os.Mkdir(dir, 0755)
			if err != nil {
				return err
			}
		}
	}

	return nil
}