package thr1

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func Fingerprint(r *http.Request) string {
	result := strings.Join([]string{
		thr1Head(r),
		thr1Lang(r),
		thr1Sec(r),
		thr1UA(r),
		thr1Encoding(r),
	}, "_")

	slog.Info("THR1 got", "method", r.Method, "path", r.URL.Path, "thr1", result)

	return result
}

func thr1Head(r *http.Request) string {
	method := strings.ToLower(r.Method)
	if len(method) > 3 {
		method = method[:3]
	}

	version := "00"
	if override := r.Header.Get("X-Http-Version"); override != "" {
		switch strings.TrimSpace(strings.ToUpper(override)) {
		case "HTTP/1.0":
			version = "10"
		case "HTTP/1.1":
			version = "11"
		case "HTTP/2.0":
			version = "20"
		case "HTTP/3.0":
			version = "30"
		}
	} else {
		switch {
		case r.ProtoMajor == 1 && r.ProtoMinor == 0:
			version = "10"
		case r.ProtoMajor == 1 && r.ProtoMinor == 1:
			version = "11"
		case r.ProtoMajor == 2:
			version = "20"
		case r.ProtoMajor == 3:
			version = "30"
		}
	}

	hasSec := false
	for k := range r.Header {
		if strings.HasPrefix(strings.ToLower(k), "sec-") {
			hasSec = true
			break
		}
	}

	return method + version + strconv.FormatBool(hasSec)[:2]
}

func thr1Encoding(r *http.Request) string {
	raw := r.Header.Get("Accept-Encoding")
	if raw == "" {
		return "none-00"
	}

	encodings := strings.Split(raw, ",")
	count := len(encodings)
	if count > 99 {
		count = 99
	}

	seen := make(map[string]struct{})
	var available []string
	for _, e := range encodings {
		enc := strings.ToLower(strings.TrimSpace(strings.Split(e, ";")[0]))
		if enc != "" {
			if _, exists := seen[enc]; !exists {
				available = append(available, enc)
				seen[enc] = struct{}{}
			}
		}
	}

	priorities := map[string]int{
		"zstd":    1,
		"br":      2,
		"deflate": 3,
		"gzip":    4,
		"*":       5,
	}

	best := "none"
	bestRank := 999 // arbitrarily high
	for _, enc := range available {
		if rank, ok := priorities[enc]; ok {
			if rank < bestRank {
				best = enc
				bestRank = rank
			}
		}
	}

	if best == "*" {
		best = "wild"
	}

	return best + "-" + pad2(count)
}

func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	if n > 99 {
		return "99"
	}
	return strconv.Itoa(n)
}

func thr1Lang(r *http.Request) string {
	raw := r.Header.Get("Accept-Language")
	if raw == "" {
		return "-000000000"
	}
	trimmed := first4AlphaNum(strings.ToLower(raw)) + "-"
	sum := sha256.Sum256([]byte(raw))
	return trimmed + hex.EncodeToString(sum[:])[:9]
}

func first4AlphaNum(s string) string {
	out := make([]rune, 0, 4)
	for _, ch := range s {
		if len(out) == 4 {
			break
		}
		if ('a' <= ch && ch <= 'z') || ('0' <= ch && ch <= '9') {
			out = append(out, ch)
		}
	}
	for len(out) < 4 {
		out = append(out, '0')
	}
	return string(out)
}

func thr1Sec(r *http.Request) string {
	var lines []string
	for k, vs := range r.Header {
		lkey := strings.ToLower(k)
		if !strings.HasPrefix(lkey, "sec-") || lkey == "sec-fetch-user" {
			continue
		}
		switch lkey {
		case "sec-ch-ua":
			lines = append(lines, parseSecChUA(vs))
		case "sec-ch-ua-mobile":
			lines = append(lines, parseSecCHSimple("mobile", vs))
		case "sec-ch-ua-platform":
			lines = append(lines, parseSecCHSimple("platform", vs))
		case "sec-ch-ua-platform-version":
			lines = append(lines, parseSecCHSimple("platform_version", vs))
		case "sec-ch-ua-model":
			lines = append(lines, parseSecCHSimple("model", vs))
		case "sec-ch-ua-full-version":
			lines = append(lines, parseSecCHSimple("full_version", vs))
		default:
			for _, v := range vs {
				v = strings.Trim(v, `" `)
				lines = append(lines, lkey+":"+v)
			}
		}
	}
	sort.Strings(lines)
	canonical := strings.Join(lines, "\n")
	sum := sha256.Sum256([]byte(canonical))
	return "sec-" + hex.EncodeToString(sum[:])[:9]
}

var brandVersionRe = regexp.MustCompile(`\s*"([^"]+)";v="([^"]+)"`)

func parseSecChUA(vs []string) string {
	type pair struct{ Brand, Version string }
	var pairs []pair

	for _, v := range vs {
		for _, match := range brandVersionRe.FindAllStringSubmatch(v, -1) {
			if len(match) != 3 {
				continue
			}
			brand := match[1]
			version := match[2]
			if brand == "Not=A?Brand" {
				continue
			}
			pairs = append(pairs, pair{brand, version})
		}
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].Brand < pairs[j].Brand
	})

	var sb strings.Builder
	sb.WriteString("ua:")
	for i, p := range pairs {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(p.Brand + "/" + p.Version)
	}
	return sb.String()
}

func parseSecCHSimple(key string, vs []string) string {
	for _, v := range vs {
		v = strings.Trim(v, `" `)
		if key == "mobile" {
			switch v {
			case "?1":
				return "mobile:true"
			case "?0":
				return "mobile:false"
			default:
				continue
			}
		}
		return key + ":" + v
	}
	return key + ":"
}

func thr1UA(r *http.Request) string {
	ua := r.Header.Get("User-Agent")
	sum := sha256.Sum256([]byte(ua))
	return hex.EncodeToString(sum[:])[:9]
}
