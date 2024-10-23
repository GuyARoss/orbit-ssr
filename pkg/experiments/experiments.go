// Copyright (c) 2021 Guy A. Ross
// This source code is licensed under the GNU GPLv3 found in the
// license file in the root directory of this source tree.

package experiments

import "github.com/GuyARoss/orbit/pkg/log"

type Features struct {
	PreferSSR          bool
	PreferSWCCompiler  bool
	PreferViteCompiler bool
}

var GlobalExperimentalFeatures *Features = &Features{}

func Load(logger log.Logger, features []string) error {
	for _, e := range features {
		switch e {
		case "ssr":
			GlobalExperimentalFeatures.PreferSSR = true
			logger.Warn("experimental feature 'prefer ssr' enabled\n")
		case "swc":
			GlobalExperimentalFeatures.PreferSWCCompiler = true
			logger.Warn("experimental feature 'prefer swc compiler' enabled\n")
		case "vite":
			GlobalExperimentalFeatures.PreferViteCompiler = true
			logger.Warn("experimental feature 'prefer vite compiler' enabled\n")
		}
	}

	return nil
}
