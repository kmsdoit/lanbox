package units

import "testing"

func TestParseGPUs(t *testing.T) {
	ok := []struct {
		in   string
		want int
	}{
		{in: "", want: 0},
		{in: "  ", want: 0},
		{in: "all", want: -1},
		{in: " all ", want: -1},
		{in: "1", want: 1},
		{in: "2", want: 2},
		{in: " 8 ", want: 8},
	}
	for _, tc := range ok {
		got, err := ParseGPUs(tc.in)
		if err != nil {
			t.Fatalf("ParseGPUs(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("ParseGPUs(%q)=%d, want %d", tc.in, got, tc.want)
		}
	}

	for _, in := range []string{"0", "-1", "-3", "foo", "ALL", "1.5", "device=0"} {
		if _, err := ParseGPUs(in); err == nil {
			t.Fatalf("ParseGPUs(%q) should fail", in)
		}
	}
}
