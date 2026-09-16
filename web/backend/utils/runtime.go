package utils

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/xibodev/facet-studio/pkg/config"
	"github.com/xibodev/facet-studio/pkg/logger"
)

// GetFacetStudioHome returns the facet-studio home directory.
// Priority: $FACET_STUDIO_HOME > ~/.facet-studio
func GetFacetStudioHome() string {
	return config.GetHome()
}

// GetDefaultConfigPath returns the default path to the facet-studio config file.
func GetDefaultConfigPath() string {
	if configPath := os.Getenv(config.EnvConfig); configPath != "" {
		return configPath
	}
	return filepath.Join(GetFacetStudioHome(), "config.json")
}

// FindFacetStudioBinary locates the facet-studio executable.
// Search order:
//  1. FACET_STUDIO_BINARY environment variable (explicit override)
//  2. Same directory as the current executable
//  3. Falls back to "facet-studio" and relies on $PATH
func FindFacetStudioBinary() string {
	// CANDIDATE NAMES, HYPHENATED FIRST.
	//
	// The Makefile builds BINARY_NAME=facet-studio, so `facet-studio.exe` is
	// what actually lands on disk. This function looked ONLY for
	// `facetstudio.exe` on Windows -- no hyphen -- so the stat missed, the
	// lookup fell through to the bare name, and exec failed with the useless
	// "exit status 1" that the launcher logged five times without ever saying
	// which path it tried.
	//
	// KERNEL NAME FIRST. The harness ships under two names depending on how it
	// was obtained: `facet-studio-kernel` when installed beside the shell as
	// part of the full product, and `facet-studio` when someone built or
	// downloaded the harness on its own. Both are the same binary; only the
	// packaging differs. Preferring the kernel name means a full-product
	// install resolves to ITS OWN harness rather than to an unrelated one that
	// happens to be on $PATH -- which would run the user's chat against a
	// different build than the shell was shipped with, and nothing would say so.
	//
	// Both spellings are accepted rather than one corrected, because the
	// hyphenless form is real elsewhere (the pid file is facetstudio.pid.json,
	// and gateway.go matches "facetstudio.exe" in tasklist output). Renaming
	// the built binary to satisfy this would break those; renaming this to
	// match the build would break an installed hyphenless one.
	var names []string
	if runtime.GOOS == "windows" {
		names = []string{"facet-studio-kernel.exe", "facet-studio.exe", "facetstudio.exe"}
	} else {
		names = []string{"facet-studio-kernel", "facet-studio", "facetstudio"}
	}

	if p := os.Getenv(config.EnvBinary); p != "" {
		if info, _ := os.Stat(p); info != nil && !info.IsDir() {
			return p
		}
	}

	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, name := range names {
			candidate := filepath.Join(dir, name)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
		// Say WHICH names were tried and where. The previous version logged a
		// Debugf naming only the launcher's own path, so a failed lookup was
		// indistinguishable from a spawn that failed for any other reason.
		logger.Warnf("no facet-studio binary found in %s (tried %v); "+
			"falling back to PATH lookup, which will fail if it is not installed",
			dir, names)
	}

	return names[0]
}

func appendUniqueIP(addrs []string, seen map[string]struct{}, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return addrs
	}
	if _, ok := seen[value]; ok {
		return addrs
	}
	seen[value] = struct{}{}
	return append(addrs, value)
}

// GetLocalIPv4s returns all non-loopback local IPv4 addresses.
func GetLocalIPv4s() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	results := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP == nil || ipnet.IP.IsLoopback() {
			continue
		}
		if ip4 := ipnet.IP.To4(); ip4 != nil {
			results = appendUniqueIP(results, seen, ip4.String())
		}
	}
	return results
}

func isDisplayGlobalIPv6(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.To4() != nil {
		return false
	}
	ip = ip.To16()
	if ip == nil {
		return false
	}
	// Only show IPv6 global unicast addresses in 2000::/3.
	return ip[0]&0xe0 == 0x20
}

// GetGlobalIPv6s returns all IPv6 global unicast addresses.
func GetGlobalIPv6s() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	results := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP == nil {
			continue
		}
		ip := ipnet.IP
		if !isDisplayGlobalIPv6(ip) {
			continue
		}
		results = appendUniqueIP(results, seen, ip.String())
	}
	return results
}

// GetLocalIPv4 returns the first non-loopback local IPv4 address.
func GetLocalIPv4() string {
	addrs := GetLocalIPv4s()
	if len(addrs) == 0 {
		return ""
	}
	return addrs[0]
}

// GetLocalIPv6 returns the first IPv6 global unicast address.
func GetLocalIPv6() string {
	addrs := GetGlobalIPv6s()
	if len(addrs) == 0 {
		return ""
	}
	return addrs[0]
}

// GetLocalIP returns a non-loopback local IPv4 address for backward compatibility.
func GetLocalIP() string {
	return GetLocalIPv4()
}

// OpenBrowser automatically opens the given URL in the default browser.
func OpenBrowser(url string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return LauncherExecCommand("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return fmt.Errorf("unsupported platform")
	}
}
