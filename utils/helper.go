package utils

import (
	"bytes"

	"golang.org/x/sys/unix"
)

const (
	X86Architecture = "x86_64"
	ArmArchitecture = "arm64"
	AmdArchitecture = "amd64"
)

func GetRuntimeArchitecture() string {
	var uname unix.Utsname
	if err := unix.Uname(&uname); err != nil {
		return AmdArchitecture
	}

	switch string(uname.Machine[:bytes.IndexByte(uname.Machine[:], 0)]) {
	case "aarch64":
		return ArmArchitecture
	default:
		return X86Architecture
	}
}
