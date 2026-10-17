package systemone

import (
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/config"
)

func noul(v float64) Answer {
	return Answer{Type: Noul, Noul: &v}
}

func TestScoreAnswers(t *testing.T) {
	tests := []struct {
		name      string
		answers   map[string]Answer
		hasGeoIP  bool
		wantScore float64
		wantBlock bool
	}{
		{
			name: "browser passes",
			answers: map[string]Answer{
				"legit":            noul(0.95),
				"header_coherence": noul(0.95),
				"language":         noul(0.95),
				"hosting_network":  noul(0.9),
				"deep_link":        noul(0.9),
				"client_class": {
					Type:          Choice,
					Choice:        "browser",
					Probabilities: map[string]float64{"browser": 1},
				},
			},
			hasGeoIP:  true,
			wantScore: 1.5,
		},
		{
			name: "scripted client blocks",
			answers: map[string]Answer{
				"legit":            noul(0.05),
				"header_coherence": noul(0.05),
				"language":         noul(0.1),
				"hosting_network":  noul(0.1),
				"deep_link":        noul(0.4),
				"client_class": {
					Type:          Choice,
					Choice:        "http_library",
					Probabilities: map[string]float64{"http_library": 1},
				},
			},
			hasGeoIP:  true,
			wantScore: 12.3,
			wantBlock: true,
		},
		{
			name: "headless browser blocks",
			answers: map[string]Answer{
				"legit":            noul(0.4),
				"header_coherence": noul(0.8),
				"language":         noul(0.9),
				"hosting_network":  noul(0.8),
				"deep_link":        noul(0.6),
				"client_class": {
					Type:          Choice,
					Choice:        "headless_browser",
					Probabilities: map[string]float64{"headless_browser": 1},
				},
			},
			hasGeoIP:  true,
			wantScore: 6.6,
			wantBlock: true,
		},
		{
			name: "geoip questions skipped without database",
			answers: map[string]Answer{
				"legit":            noul(0.5),
				"header_coherence": noul(0.9),
				"language":         noul(0),
				"hosting_network":  noul(0),
				"deep_link":        noul(0.9),
			},
			hasGeoIP:  false,
			wantScore: 3.6,
		},
		{
			name: "missing answers are skipped",
			answers: map[string]Answer{
				"legit": noul(0.95),
			},
			hasGeoIP:  true,
			wantScore: 0.25,
		},
		{
			name:      "nil response",
			answers:   nil,
			wantScore: 0,
		},
		{
			name: "noul answers are clamped",
			answers: map[string]Answer{
				"legit":            noul(1.5),
				"header_coherence": noul(-1),
				"language":         noul(0.5),
				"hosting_network":  noul(0.5),
				"deep_link":        noul(2),
				"client_class": {
					Type:   Choice,
					Choice: "probe",
				},
			},
			hasGeoIP:  true,
			wantScore: 7.5,
			wantBlock: true,
		},
		{
			name: "score at threshold blocks",
			answers: map[string]Answer{
				"legit": noul(0),
			},
			wantScore: blockScore,
			wantBlock: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotScore, scores := scoreAnswers(&Response{Answers: tt.answers}, tt.hasGeoIP)
			if math.Abs(gotScore-tt.wantScore) > 1e-9 {
				t.Fatalf("score = %v, want %v (components: %v)", gotScore, tt.wantScore, scores)
			}
			if gotBlock := gotScore >= blockScore; gotBlock != tt.wantBlock {
				t.Fatalf("block = %v, want %v (score %v, threshold %v)", gotBlock, tt.wantBlock, gotScore, blockScore)
			}
		})
	}
}

func TestClientClassRiskFor(t *testing.T) {
	tests := []struct {
		name   string
		answer Answer
		want   float64
		ok     bool
	}{
		{
			name:   "choice browser",
			answer: Answer{Choice: "browser"},
			want:   0,
			ok:     true,
		},
		{
			name:   "choice probe",
			answer: Answer{Choice: "probe"},
			want:   1,
			ok:     true,
		},
		{
			name:   "unknown choice falls back to unknown class",
			answer: Answer{Choice: "not-a-class"},
			want:   0.5,
			ok:     true,
		},
		{
			name:   "probabilities are averaged",
			answer: Answer{Probabilities: map[string]float64{"browser": 0.5, "http_library": 0.5}},
			want:   0.425,
			ok:     true,
		},
		{
			name:   "probabilities are normalized",
			answer: Answer{Probabilities: map[string]float64{"browser": 1, "probe": 3}},
			want:   0.75,
			ok:     true,
		},
		{
			name:   "empty answer",
			answer: Answer{},
			ok:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := clientClassRiskFor(tt.answer)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("risk = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateRiskThreshold(t *testing.T) {
	tests := []struct {
		name    string
		answers string
		wantErr bool
	}{
		{
			name: "browser accepted",
			answers: `{
				"legit": {"type": "noul", "noul": 0.99},
				"header_coherence": {"type": "noul", "noul": 0.99},
				"language": {"type": "noul", "noul": 0.99},
				"hosting_network": {"type": "noul", "noul": 0.99},
				"deep_link": {"type": "noul", "noul": 0.9},
				"client_class": {"type": "choice", "choice": "browser", "probabilities": {"browser": 1}}
			}`,
		},
		{
			name: "scripted client refused",
			answers: `{
				"legit": {"type": "noul", "noul": 0.05},
				"header_coherence": {"type": "noul", "noul": 0.1},
				"deep_link": {"type": "noul", "noul": 0.5},
				"client_class": {"type": "choice", "choice": "http_library", "probabilities": {"http_library": 1}}
			}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if _, err := io.WriteString(w, `{"model":"jev-test","answers":`+tt.answers+`,"usage":{"input_tokens":1,"output_tokens":1}}`); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			t.Cleanup(srv.Close)

			impl := &Impl{
				cli:    NewClient(srv.URL),
				model:  "jev-test",
				sysOne: &config.SystemOne{},
			}
			r := httptest.NewRequest(http.MethodGet, "http://example.com/some/page", nil)
			r.Header.Set("User-Agent", "test")
			in := &challenge.ValidateInput{
				Challenge: &challenge.Challenge{TargetURL: "http://example.com/some/page"},
			}

			err := impl.Validate(r, slog.New(slog.NewTextHandler(io.Discard, nil)), in)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			var cerr *challenge.Error
			if !errors.As(err, &cerr) {
				t.Fatalf("error = %v, want *challenge.Error", err)
			}
			if cerr.StatusCode != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", cerr.StatusCode, http.StatusForbidden)
			}
		})
	}
}
