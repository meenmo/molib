package swaps

import (
	"testing"

	"github.com/meenmo/molib/calendar"
	"github.com/meenmo/molib/swap/market"
)

func TestRegistryReturnsCopies(t *testing.T) {
	leg, ok := FloatingLegByIndex("ESTR")
	if !ok {
		t.Fatal("FloatingLegByIndex(ESTR) not found")
	}
	leg.IncludeFinalPrincipal = false

	again, ok := FloatingLegByIndex("ESTR")
	if !ok {
		t.Fatal("FloatingLegByIndex(ESTR) second lookup not found")
	}
	if !again.IncludeFinalPrincipal {
		t.Fatal("FloatingLegByIndex returned shared mutable state")
	}
}

func TestBootstrapConfigByIndex(t *testing.T) {
	cfg, ok := BootstrapConfigByIndex("sonia")
	if !ok {
		t.Fatal("BootstrapConfigByIndex(sonia) not found")
	}
	if cfg.Calendar != calendar.EN || cfg.SpotLag != 0 || cfg.Kind != BootstrapOIS {
		t.Fatalf("SONIA config = %+v", cfg)
	}

	cfg, ok = BootstrapConfigByIndex("CD91")
	if !ok {
		t.Fatal("BootstrapConfigByIndex(CD91) not found")
	}
	if cfg.Calendar != calendar.KR || cfg.SpotLag != 1 || cfg.Kind != BootstrapKRX {
		t.Fatalf("CD91 config = %+v", cfg)
	}
}

func TestFixedFloatLegsByIndex(t *testing.T) {
	fixed, floating, ok := FixedFloatLegsByIndex("EURIBOR6M")
	if !ok {
		t.Fatal("FixedFloatLegsByIndex(EURIBOR6M) not found")
	}
	if fixed.LegType != market.LegFixed || floating.ReferenceIndex != market.EURIBOR6M {
		t.Fatalf("unexpected EURIBOR6M legs: fixed=%+v floating=%+v", fixed, floating)
	}
}
