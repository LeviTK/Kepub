package validation

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/LeviTK/Kepub/internal/fault"
)

type Readiness struct {
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Version string `json:"version,omitempty"`
}

// Probe checks only local readiness. It shares integrity and process ownership
// with formal validation, but never installs tools or checks a publication.
func Probe(ctx context.Context, o Options) (java, checker Readiness, err error) {
	java = Readiness{Status: "unavailable", Reason: "Java is not on PATH"}
	checker = Readiness{Status: "unavailable", Reason: "Java 17+ and the pinned EPUBCheck release are required", Version: Version}
	if ctx.Err() != nil {
		return java, checker, fault.New(130, "CANCELLED", "request cancelled")
	}
	if o.Java == "" {
		o.Java = "java"
	}
	path, e := exec.LookPath(o.Java)
	if e != nil {
		return java, checker, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	probe := func(args []string) (string, error) {
		out, logs, exit, e := runProcessLimit(ctx, path, args, 64<<10)
		if e != nil || exit != 0 {
			if ctx.Err() == context.Canceled {
				return "", fault.New(130, "CANCELLED", "request cancelled")
			}
			return "", fault.New(5, "DEPENDENCY_PROBE_FAILED", "bounded dependency version probe failed or timed out")
		}
		return string(out) + string(logs), nil
	}
	text, e := probe([]string{"-version"})
	if e != nil {
		java.Status = "failed"
		java.Reason = "version probe failed"
		return java, checker, e
	}
	match := regexp.MustCompile(`(?:openjdk|java) version "([0-9]+)(?:\.[^"]*)?"`).FindStringSubmatch(text)
	if len(match) != 2 {
		java.Reason = "Java version could not be established"
		return java, checker, nil
	}
	major, _ := strconv.Atoi(match[1])
	java.Version = match[1]
	if major < 17 {
		java.Reason = "Java 17 or later is required"
		return java, checker, nil
	}
	java.Status, java.Reason = "ready", "Java 17+ version established"
	if o.JAR == "" {
		o.JAR = os.Getenv("KEPUB_EPUBCHECK_JAR")
	}
	if o.JAR == "" {
		checker.Reason = "KEPUB_EPUBCHECK_JAR is not configured"
		return java, checker, nil
	}
	hash, e := fingerprint(o.JAR)
	if e != nil || hash != ToolSHA256 {
		checker.Reason = "checker executable JAR set does not match the pinned release"
		return java, checker, nil
	}
	text, e = probe([]string{"-jar", o.JAR, "--version"})
	if e != nil {
		checker.Status, checker.Reason = "failed", "version probe failed"
		return java, checker, e
	}
	after, e := fingerprint(o.JAR)
	if e != nil || after != hash {
		return java, checker, fault.New(4, "INPUT_DRIFT", "checker bytes changed during readiness probe")
	}
	if strings.TrimSpace(text) != "EPUBCheck v"+Version {
		checker.Reason = "checker version does not match pinned release"
		return java, checker, nil
	}
	checker.Status, checker.Reason = "ready", "pinned executable JAR set and version verified; no publication checked"
	return java, checker, nil
}
