package vtui

import (
	"os"
	"strconv"
	"strings"
)

// ProbeGraphicsProtocols resolves the graphics protocols the terminal
// supports, best first, combining environment detection with an active DA1
// query where the environment alone is not conclusive. Call once at startup
// before the input reader starts, and hand the first entry to SetProtocol.
func ProbeGraphicsProtocols() []GraphicsProtocol {
	return probeGraphicsProtocolsWith(os.Getenv, da1Sixel)
}

// probeGraphicsProtocolsWith is ProbeGraphicsProtocols with the environment
// and the DA1 query injected, so the decision is testable without a terminal.
func probeGraphicsProtocolsWith(env func(string) string, da1 func() bool) []GraphicsProtocol {
	if envs := envGraphicsProtocolsWith(env); len(envs) > 0 {
		return envs
	}
	// The environment says nothing about images. Ask the device itself
	// whether it is a sixel terminal: this is how a bare conhost (no
	// WT_SESSION), or an xterm built with sixel support, is recognised
	// without corrupting old terminals that would print raw escape garbage.
	if da1() {
		// A sixel-positive DA1 means a modern ConPTY bridge that also forwards
		// kitty, so WezTerm behind it can use its best renderer.
		if isWezTermEnv(env) {
			return weztermProtocols()
		}
		return []GraphicsProtocol{GraphicsSixel}
	}
	return nil
}

// da1ResponseComplete reports whether s already holds a whole primary
// device attributes answer: the CSI ? prefix and the terminating 'c'.
func da1ResponseComplete(s string) bool {
	return strings.HasSuffix(s, "c") && strings.Contains(s, "\x1b[?")
}

// cellSizeResponseComplete reports whether s already holds a whole CSI 16 t
// answer: the CSI 6 prefix and the terminating 't'.
func cellSizeResponseComplete(s string) bool {
	return strings.HasSuffix(s, "t") && strings.Contains(s, "\x1b[6;")
}

// parseDA1Sixel reports whether a primary device attributes answer declares
// sixel graphics. The first parameter is the conformance level; a standalone
// '4' among the parameters is the sixel extension (VT330/VT340).
func parseDA1Sixel(s string) bool {
	i := strings.Index(s, "\x1b[?")
	if i < 0 {
		return false
	}
	rest := s[i+3:]
	if j := strings.IndexByte(rest, 'c'); j >= 0 {
		rest = rest[:j]
	}
	for _, p := range strings.Split(rest, ";") {
		if strings.TrimSpace(p) == "4" {
			return true
		}
	}
	return false
}

// parseCellSizeResponse decodes a CSI 16 t answer: "\x1b[6;height;width t".
func parseCellSizeResponse(s string) (cw, ch int, ok bool) {
	i := strings.Index(s, "\x1b[6;")
	if i < 0 {
		return 0, 0, false
	}
	rest := s[i+len("\x1b[6;"):]
	if j := strings.IndexByte(rest, 't'); j >= 0 {
		rest = rest[:j]
	}
	parts := strings.Split(rest, ";")
	if len(parts) < 2 {
		return 0, 0, false
	}
	ch, errH := strconv.Atoi(strings.TrimSpace(parts[0]))
	cw, errW := strconv.Atoi(strings.TrimSpace(parts[1]))
	return cw, ch, errH == nil && errW == nil && cw > 0 && ch > 0
}
