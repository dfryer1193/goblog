package stats

import (
	"strings"

	useragent "github.com/medama-io/go-useragent"
	"github.com/medama-io/go-useragent/agents"
)

// The families a User-Agent has when it isn't recognised.
const (
	// familyOther is the browser or operating system of an unrecognised
	// User-Agent.
	familyOther = "other"
	// deviceUnknown is the device type of an unrecognised User-Agent.
	deviceUnknown = "unknown"
)

// maxMajorVersion is the most digits a browser's major version can have.
// Anything longer isn't a real major version, so it isn't emitted.
const maxMajorVersion = 4

// families is what a User-Agent says about a Visitor's software, coarse
// enough that it can't single anyone out: never a raw User-Agent, and no
// version finer than the browser's major version.
type families struct {
	// browser is the browser's family, such as "Firefox", or familyOther.
	browser string
	// browserMajor is the browser's major version, such as "145", or empty
	// if it isn't known.
	browserMajor string
	// os is the operating system's family, such as "macOS", or familyOther.
	os string
	// device is the device type: "desktop", "mobile", "tablet", "tv" or
	// deviceUnknown.
	device string
}

// familiesOf parses userAgent into its families with parser.
func familiesOf(parser *useragent.Parser, userAgent string) families {
	ua := parser.Parse(userAgent)
	f := families{
		browser:      familyOther,
		browserMajor: majorVersion(ua.BrowserVersionMajor()),
		device:       deviceOf(ua.Device()),
	}
	if ua.Browser() != "" {
		f.browser = string(ua.Browser())
	} else {
		f.browserMajor = ""
	}
	f.os = osOf(ua.OS())
	// Chrome on an Android tablet says "Mobile" only on phones, but the
	// parser calls both mobile.
	if ua.OS() == agents.OSAndroid && f.device == "mobile" && !strings.Contains(userAgent, "Mobile") {
		f.device = "tablet"
	}
	return f
}

// osOf is the operating system family for the parser's os.
func osOf(os agents.OS) string {
	switch os {
	case agents.OSMacOS:
		return "macOS"
	case agents.OSAndroid, agents.OSChromeOS, agents.OSIOS, agents.OSLinux,
		agents.OSFreeBSD, agents.OSOpenBSD, agents.OSWindows:
		return string(os)
	default:
		return familyOther
	}
}

// deviceOf is the device type for the parser's device.
func deviceOf(device agents.Device) string {
	switch device {
	case agents.DeviceDesktop, agents.DeviceMobile, agents.DeviceTablet, agents.DeviceTV:
		return strings.ToLower(string(device))
	case agents.DeviceBot:
		return deviceUnknown
	default:
		return deviceUnknown
	}
}

// majorVersion is version if it's a plausible major version (only digits,
// and not too many), or empty.
func majorVersion(version string) string {
	if version == "" || len(version) > maxMajorVersion {
		return ""
	}
	for _, c := range version {
		if c < '0' || c > '9' {
			return ""
		}
	}
	return version
}
