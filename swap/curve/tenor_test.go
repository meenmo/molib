package curve

import "testing"

func TestParseTenorYears(t *testing.T) {
	cases := map[string]float64{
		"1W":  7.0 / 365.0,
		"3M":  0.25,
		"10Y": 10,
		"91D": 91.0 / 365.0,
		"1.5": 1.5,
	}
	for tenor, want := range cases {
		got, err := ParseTenorYears(tenor)
		if err != nil {
			t.Fatalf("ParseTenorYears(%q) error: %v", tenor, err)
		}
		if got != want {
			t.Fatalf("ParseTenorYears(%q) = %.12f, want %.12f", tenor, got, want)
		}
	}
}

func TestParseTenorYearsRejectsMalformedTenor(t *testing.T) {
	if _, err := ParseTenorYears("BAD"); err == nil {
		t.Fatal("ParseTenorYears(BAD) error = nil, want error")
	}
}
