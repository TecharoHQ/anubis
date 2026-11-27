package challenge

import "testing"

func TestParseElapsedTime(t *testing.T) {
	for _, tt := range []struct {
		in      string
		want    float64
		wantErr bool
	}{
		{in: "0", want: 0},
		{in: "69", want: 69},
		{in: "12.5", want: 12.5},
		{in: "taco", wantErr: true},
		{in: "NaN", wantErr: true},
		{in: "nan", wantErr: true},
		{in: "Inf", wantErr: true},
		{in: "+Inf", wantErr: true},
		{in: "-Inf", wantErr: true},
		{in: "-1", wantErr: true},
		{in: "1e999", wantErr: true},
	} {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseElapsedTime(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseElapsedTime(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("ParseElapsedTime(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
