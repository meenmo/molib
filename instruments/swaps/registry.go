package swaps

import (
	"strings"

	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/swap/market"
)

// BootstrapKind identifies the curve construction path for a reference index.
type BootstrapKind int

const (
	BootstrapOIS BootstrapKind = iota
	BootstrapIBOR
	BootstrapKRX
)

// BootstrapConfig captures the market metadata needed to bootstrap an index curve.
type BootstrapConfig struct {
	Calendar calendar.CalendarID
	SpotLag  int
	Kind     BootstrapKind
}

// FloatingLegByIndex returns a copy of the floating-leg convention for index.
func FloatingLegByIndex(index string) (market.LegConvention, bool) {
	switch normalizeIndex(index) {
	case "ESTR":
		return ESTRFloating, true
	case "SOFR":
		return SOFRFloating, true
	case "TONAR":
		return TONARFloating, true
	case "SONIA":
		return SONIAFloating, true
	case "EURIBOR3M":
		return EURIBOR3MFloating, true
	case "EURIBOR6M":
		return EURIBOR6MFloating, true
	case "HIBOR3M":
		return HIBOR3MFloating, true
	case "TIBOR3M":
		return TIBOR3MFloating, true
	case "TIBOR6M":
		return TIBOR6MFloating, true
	case "CD91", "CD91D":
		return KRXCD91DFloating, true
	default:
		return market.LegConvention{}, false
	}
}

// FixedLegByIndex returns a copy of the standard fixed leg paired with index.
func FixedLegByIndex(index string) (market.LegConvention, bool) {
	switch normalizeIndex(index) {
	case "ESTR":
		return ESTRFixed, true
	case "SOFR":
		return SOFRFixed, true
	case "TONAR":
		return TONARFixed, true
	case "SONIA":
		return SONIAFixed, true
	case "EURIBOR3M", "EURIBOR6M":
		return EURIBORFixed, true
	case "HIBOR3M":
		return HIBOR3MFixed, true
	case "TIBOR3M", "TIBOR6M":
		return TIBORFixed, true
	case "CD91", "CD91D":
		return KRXCD91DFixed, true
	default:
		return market.LegConvention{}, false
	}
}

// FixedFloatLegsByIndex returns copies of the standard fixed and floating legs for index.
func FixedFloatLegsByIndex(index string) (market.LegConvention, market.LegConvention, bool) {
	fixed, ok := FixedLegByIndex(index)
	if !ok {
		return market.LegConvention{}, market.LegConvention{}, false
	}
	floating, ok := FloatingLegByIndex(index)
	if !ok {
		return market.LegConvention{}, market.LegConvention{}, false
	}
	return fixed, floating, true
}

// BootstrapConfigByIndex returns the bootstrap metadata for a supported curve index.
func BootstrapConfigByIndex(index string) (BootstrapConfig, bool) {
	switch normalizeIndex(index) {
	case "ESTR":
		return BootstrapConfig{Calendar: calendar.TARGET, SpotLag: 2, Kind: BootstrapOIS}, true
	case "SOFR":
		return BootstrapConfig{Calendar: calendar.FD, SpotLag: 2, Kind: BootstrapOIS}, true
	case "TONAR":
		return BootstrapConfig{Calendar: calendar.JP, SpotLag: 2, Kind: BootstrapOIS}, true
	case "SONIA":
		return BootstrapConfig{Calendar: calendar.EN, SpotLag: 0, Kind: BootstrapOIS}, true
	case "HIBOR3M":
		return BootstrapConfig{Calendar: calendar.HK, SpotLag: 2, Kind: BootstrapIBOR}, true
	case "EURIBOR3M", "EURIBOR6M":
		return BootstrapConfig{Calendar: calendar.TARGET, SpotLag: 2, Kind: BootstrapIBOR}, true
	case "CD91", "CD91D":
		return BootstrapConfig{Calendar: calendar.KR, SpotLag: 1, Kind: BootstrapKRX}, true
	default:
		return BootstrapConfig{}, false
	}
}

func normalizeIndex(index string) string {
	return strings.ToUpper(strings.TrimSpace(index))
}
