package benchmark

// ReferenceRate enumerates supported floating benchmarks.
type ReferenceRate string

const (
	ESTR      ReferenceRate = "ESTR"
	EURIBOR3M ReferenceRate = "EURIBOR3M"
	EURIBOR6M ReferenceRate = "EURIBOR6M"
	TONAR     ReferenceRate = "TONAR"
	TIBOR3M   ReferenceRate = "TIBOR3M"
	TIBOR6M   ReferenceRate = "TIBOR6M"
	CD91      ReferenceRate = "CD91"
)
