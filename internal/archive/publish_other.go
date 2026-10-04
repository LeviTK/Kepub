//go:build !linux && !darwin

package archive

import "github.com/LeviTK/Kepub/internal/fault"

func publish(from, to string) error {
	return fault.New(3, "UNSUPPORTED_PLATFORM", "atomic no-replace unpack supports Linux and macOS only")
}
