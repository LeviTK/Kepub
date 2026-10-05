package app

import (
	"context"
	"os/exec"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/LeviTK/Kepub/internal/validation"
)

func BuildVersion() map[string]any {
	v := map[string]any{"version": "development", "development": true, "goVersion": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH}
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			v["version"] = info.Main.Version
			// Go 1.27 can embed a VCS pseudo-version for an untagged local
			// build. That is build metadata, not a software release.
			v["development"] = regexp.MustCompile(`-[0-9]{14}-[0-9a-f]{12}(?:\+dirty)?$`).MatchString(info.Main.Version) || strings.HasSuffix(info.Main.Version, "+dirty")
		}
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				v["revision"] = s.Value
			case "vcs.time":
				v["revisionTime"] = s.Value
			case "vcs.modified":
				v["modified"] = s.Value == "true"
			}
		}
	}
	return v
}

func Doctor(ctx context.Context) (any, error) {
	java, checker, err := validation.Probe(ctx, validation.Options{})
	amp := validation.Readiness{Status: "unavailable", Reason: "optional Amp not discovered; core commands do not require Amp"}
	if _, e := exec.LookPath("amp"); e == nil {
		amp.Status, amp.Reason = "discovered", "PATH executable found only; version, authentication and model access not tested"
	}
	return map[string]any{"core": map[string]any{"status": "ready", "build": BuildVersion(), "requiresGUI": false, "requiresNode": false, "requiresModel": false}, "java": java, "epubcheck": checker, "amp": amp, "formalValidationAvailable": checker.Status == "ready"}, err
}
