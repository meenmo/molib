package config

// Config holds solver and curve construction parameters.
// These were previously hardcoded magic numbers throughout the codebase.
type Config struct {
	// ConvergenceTolerance is the NPV tolerance for Newton-Raphson convergence.
	// Used in bootstrap and par spread solving.
	ConvergenceTolerance float64

	// MaxBootstrapIterations is the maximum iterations for curve bootstrap.
	MaxBootstrapIterations int

	// MaxSpreadIterations is the maximum iterations for par spread solving.
	MaxSpreadIterations int

	// DampingFactor limits Newton step size to prevent overshooting.
	// Delta is clamped to DampingFactor * currentGuess.
	DampingFactor float64

	// MaxPaymentDates is the maximum number of payment dates to generate.
	// 600 supports up to 50Y with monthly frequency.
	MaxPaymentDates int

	// MinDiscountFactor is the floor for discount factors to prevent
	// numerical instability (division by near-zero).
	MinDiscountFactor float64

	// DerivativeThreshold is the minimum derivative magnitude.
	// Below this, Newton iteration stops to avoid division by near-zero.
	DerivativeThreshold float64

	// PVToleranceMultiplier scales the notional to compute PV tolerance.
	// PV tolerance = PVToleranceMultiplier * max(1.0, abs(notional))
	PVToleranceMultiplier float64
}

// DefaultConfig provides production-ready default values.
var DefaultConfig = Config{
	ConvergenceTolerance:   1e-12,
	MaxBootstrapIterations: 100,
	MaxSpreadIterations:    10,
	DampingFactor:          0.5,
	MaxPaymentDates:        600,
	MinDiscountFactor:      1e-9,
	DerivativeThreshold:    1e-15,
	PVToleranceMultiplier:  1e-10,
}

// cfg is the active configuration. Defaults to DefaultConfig.
var cfg = DefaultConfig

// SetConfig replaces the active configuration.
func SetConfig(c Config) {
	cfg = c
}

// GetConfig returns the active configuration.
func GetConfig() Config {
	return cfg
}
