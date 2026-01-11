package market

// ReferenceIndex enumerates supported floating benchmarks.
type ReferenceIndex string

const (
	ESTR      ReferenceIndex = "ESTR"
	EURIBOR3M ReferenceIndex = "EURIBOR3M"
	EURIBOR6M ReferenceIndex = "EURIBOR6M"
	TONAR     ReferenceIndex = "TONAR"
	TIBOR3M   ReferenceIndex = "TIBOR3M"
	TIBOR6M   ReferenceIndex = "TIBOR6M"
	SOFR      ReferenceIndex = "SOFR"
	CD91D     ReferenceIndex = "CD91D"
)

// IsOvernight reports whether the reference rate is an overnight index used in OIS discounting/projection.
func IsOvernight(r ReferenceIndex) bool {
	switch r {
	case ESTR, TONAR, SOFR:
		return true
	default:
		return false
	}
}
