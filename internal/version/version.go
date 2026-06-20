package version

import "runtime/debug"

var Version = "0.1.1"

func init() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		Version = info.Main.Version
	}
}