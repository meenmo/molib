package market

// ReferenceRate enumerates supported floating benchmarks.
type ReferenceRate string

const (
	ESTR      ReferenceRate = "ESTR"
	EURIBOR3M ReferenceRate = "EURIBOR3M"
	EURIBOR6M ReferenceRate = "EURIBOR6M"
	TONAR     ReferenceRate = "TONAR"
	TIBOR3M   ReferenceRate = "TIBOR3M"
	TIBOR6M   ReferenceRate = "TIBOR6M"
	SOFR      ReferenceRate = "SOFR"
	CD91      ReferenceRate = "CD91"
)

// IsOvernight reports whether the reference rate is an overnight index used in OIS discounting/projection.
func IsOvernight(r ReferenceRate) bool {
	switch r {
	case ESTR, TONAR, SOFR:
		return true
	default:
		return false
	}
}
