package app

import "testing"

func TestPseudoVersionClassification(t *testing.T) {
	for _, version := range []string{
		"v0.0.0-20261005105343-1648a8931c6c",
		"v0.1.1-0.20261005105343-1648a8931c6c",
		"v1.2.3-rc.1.0.20261005105343-1648a8931c6c",
		"v0.1.1-0.20261005105343-1648a8931c6c+dirty",
	} {
		if !pseudoVersion.MatchString(version) {
			t.Fatal("pseudo-version misclassified as release", version)
		}
	}
	for _, version := range []string{"v0.1.0", "v1.2.3-rc.1", "v1.2.3+build.4"} {
		if pseudoVersion.MatchString(version) {
			t.Fatal("release misclassified as pseudo-version", version)
		}
	}
}
