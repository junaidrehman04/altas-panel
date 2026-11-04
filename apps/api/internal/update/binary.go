package update

// IsInstallableBinary reports whether data has an executable container
// header. Shell scripts and other text payloads are refused so an update
// cannot be executed as a script.
func IsInstallableBinary(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	if data[0] == 0x7f && data[1] == 'E' && data[2] == 'L' && data[3] == 'F' {
		return true
	}
	if data[0] == 'M' && data[1] == 'Z' {
		return true
	}
	// Mach-O thin and fat headers, both endiannesses.
	magic := uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
	switch magic {
	case 0xFEEDFACE, 0xFEEDFACF, 0xCAFEBABE, 0xCAFEBABF, 0xCEFAEDFE, 0xCFFAEDFE:
		return true
	default:
		return false
	}
}

// BinaryAssetName is the official release asset for this OS and architecture.
func BinaryAssetName(goos, goarch string) (string, error) {
	if !knownOS(goos) || !knownArch(goarch) {
		return "", ErrAsset
	}
	name := "altas-panel-" + goos + "-" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name, nil
}

func knownOS(goos string) bool {
	switch goos {
	case "linux", "darwin", "windows", "freebsd":
		return true
	default:
		return false
	}
}

func knownArch(goarch string) bool {
	switch goarch {
	case "amd64", "arm64", "386", "arm":
		return true
	default:
		return false
	}
}
