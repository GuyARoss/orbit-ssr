// Copyright (c) 2021 Guy A. Ross
// This source code is licensed under the GNU GPLv3 found in the
// license file in the root directory of this source tree.

package internal

import (
	"fmt"
	"strings"
	"time"

	"github.com/GuyARoss/orbit/pkg/hotreload"
	"github.com/GuyARoss/orbit/pkg/log"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type DevServer struct {
	hr             *hotreload.HotReload
	logger         log.Logger
	session        *devSession
	fileChangeOpts *ChangeRequestOpts
}

// RedirectionBundler waits for a redirection event from the client and performs a re-bundle if needed.
func (s *DevServer) RedirectionBundler() {
	for {
		// during dev mode when the browser redirects, we want to process
		// the file only if the bundle has not already been processed
		event := <-s.hr.Redirected

		for _, bundleKey := range event.BundleKeys.Diff(event.PreviousBundleKeys) {
			// the change request maintains a cache of recently bundled pages
			// if it exists on the cache, then we don't care to process it
			if !s.session.ChangeRequest.ExistsInCache(bundleKey) {
				go func(change string) {
					err := s.session.DoBundleKeyChangeRequest(change, s.fileChangeOpts)

					if err != nil {
						s.hr.EmitLog(hotreload.Warning, err.Error())
					}
				}(bundleKey)
			}
		}
	}
}

var blacklistedDirectories = []string{
	".orbit/",
}

func isBlacklistedDirectory(dir string) bool {
	for _, b := range blacklistedDirectories {
		if strings.Contains(dir, b) {
			return true
		}
	}
	return false
}

type DevServerEvent struct {
	fsnotify.Event
	Processed bool
}

// FileWatcherBundler watches for events given the file watcher and processes change requests as found
func (s *DevServer) FileWatcherBundler(timeout time.Duration, watcher *fsnotify.Watcher) {
	var recentEvent *DevServerEvent

	for {
		time.Sleep(timeout)

		select {
		case e := <-watcher.Events:
			if isBlacklistedDirectory(e.Name) {
				continue
			}
			recentEvent = &DevServerEvent{Event: e, Processed: false}
		default:
			if recentEvent == nil || recentEvent.Processed {
				continue
			}
			recentEvent.Processed = true
			err := s.session.DoFileChangeRequest(recentEvent.Name, s.fileChangeOpts)

			switch err {
			case nil, ErrFileTooRecentlyProcessed:
				//
			default:
				s.hr.EmitLog(hotreload.Error, err.Error())
				s.logger.Error(err.Error())
			}

			if err == nil && len(viper.GetString("dep_map_out_dir")) > 0 {
				s.session.SourceMap.Write(viper.GetString("dep_map_out_dir"))
			}
		case err := <-watcher.Errors:
			panic(fmt.Sprintf("watcher failed %s", err.Error()))
		}
	}
}

func NewDevServer(hotReload *hotreload.HotReload, logger log.Logger, session *devSession, changeOpts *ChangeRequestOpts) *DevServer {
	return &DevServer{
		hr:             hotReload,
		logger:         logger,
		session:        session,
		fileChangeOpts: changeOpts,
	}
}
