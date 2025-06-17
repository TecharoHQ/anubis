package internal

import (
	"fmt"
	"strings"
	"testing"
)

// XXHash64sum is a test alias for FastHash to benchmark against SHA256
func XXHash64sum(text string) string {
	return FastHash(text)
}

// Test data that matches real usage patterns in the codebase
var (
	// Typical policy checker inputs
	policyInputs = []string{
		"User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"User-Agent: bot/1.0",
		"User-Agent: GoogleBot/2.1",
		"/robots.txt",
		"/api/.*",
		"10.0.0.0/8",
		"192.168.1.0/24",
		"172.16.0.0/12",
	}

	// Challenge data from challengeFor function
	challengeInputs = []string{
		"Accept-Language=en-US,X-Real-IP=192.168.1.100,User-Agent=Mozilla/5.0,WeekTime=2025-06-16T00:00:00Z,Fingerprint=abc123,Difficulty=5",
		"Accept-Language=fr-FR,X-Real-IP=10.0.0.50,User-Agent=Chrome/91.0,WeekTime=2025-06-16T00:00:00Z,Fingerprint=def456,Difficulty=3",
		"Accept-Language=es-ES,X-Real-IP=172.16.1.1,User-Agent=Safari/14.0,WeekTime=2025-06-16T00:00:00Z,Fingerprint=ghi789,Difficulty=7",
	}

	// Bot rule patterns
	botRuleInputs = []string{
		"GoogleBot::path:/robots.txt",
		"BingBot::useragent:Mozilla/5.0 (compatible; bingbot/2.0)",
		"FacebookBot::headers:Accept-Language,User-Agent",
		"TwitterBot::cidr:192.168.1.0/24",
	}

	// CEL expressions from policy rules
	celInputs = []string{
		`request.headers["User-Agent"].contains("bot")`,
		`request.path.startsWith("/api/") && request.method == "POST"`,
		`request.remoteAddress in ["192.168.1.0/24", "10.0.0.0/8"]`,
		`request.userAgent.matches(".*[Bb]ot.*") || request.userAgent.matches(".*[Cc]rawler.*")`,
	}

	// Thoth ASN checker inputs
	asnInputs = []string{
		"ASNChecker\nAS 15169\nAS 8075\nAS 32934",
		"ASNChecker\nAS 13335\nAS 16509\nAS 14061",
		"ASNChecker\nAS 36351\nAS 20940\nAS 8100",
	}
)

func BenchmarkSHA256_PolicyInputs(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input := policyInputs[i%len(policyInputs)]
		_ = SHA256sum(input)
	}
}

func BenchmarkXXHash_PolicyInputs(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input := policyInputs[i%len(policyInputs)]
		_ = XXHash64sum(input)
	}
}

func BenchmarkSHA256_ChallengeInputs(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input := challengeInputs[i%len(challengeInputs)]
		_ = SHA256sum(input)
	}
}

func BenchmarkXXHash_ChallengeInputs(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input := challengeInputs[i%len(challengeInputs)]
		_ = XXHash64sum(input)
	}
}

func BenchmarkSHA256_BotRuleInputs(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input := botRuleInputs[i%len(botRuleInputs)]
		_ = SHA256sum(input)
	}
}

func BenchmarkXXHash_BotRuleInputs(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input := botRuleInputs[i%len(botRuleInputs)]
		_ = XXHash64sum(input)
	}
}

func BenchmarkSHA256_CELInputs(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input := celInputs[i%len(celInputs)]
		_ = SHA256sum(input)
	}
}

func BenchmarkXXHash_CELInputs(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input := celInputs[i%len(celInputs)]
		_ = XXHash64sum(input)
	}
}

func BenchmarkSHA256_ASNInputs(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input := asnInputs[i%len(asnInputs)]
		_ = SHA256sum(input)
	}
}

func BenchmarkXXHash_ASNInputs(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input := asnInputs[i%len(asnInputs)]
		_ = XXHash64sum(input)
	}
}

// Benchmark the policy list hashing used in checker.go
func BenchmarkSHA256_PolicyList(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var sb strings.Builder
		for _, input := range policyInputs {
			fmt.Fprintln(&sb, SHA256sum(input))
		}
		_ = SHA256sum(sb.String())
	}
}

func BenchmarkXXHash_PolicyList(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var sb strings.Builder
		for _, input := range policyInputs {
			fmt.Fprintln(&sb, XXHash64sum(input))
		}
		_ = XXHash64sum(sb.String())
	}
}

// Tests that xxhash doesn't have collisions in realistic scenarios
func TestHashCollisions(t *testing.T) {
	allInputs := append(append(append(append(policyInputs, challengeInputs...), botRuleInputs...), celInputs...), asnInputs...)

	// Start with realistic inputs from actual usage
	xxhashHashes := make(map[string]string)
	for _, input := range allInputs {
		hash := XXHash64sum(input)
		if existing, exists := xxhashHashes[hash]; exists {
			t.Errorf("XXHash collision detected: %q and %q both hash to %s", input, existing, hash)
		}
		xxhashHashes[hash] = input
	}

	t.Logf("Basic test: %d realistic inputs, no collisions", len(allInputs))

	// Test similar strings that might cause hash collisions
	prefixes := []string{"User-Agent: ", "X-Real-IP: ", "Accept-Language: ", "Host: "}
	suffixes := []string{"bot", "crawler", "spider", "scraper", "Mozilla", "Chrome", "Safari", "Firefox"}
	variations := []string{"", "/1.0", "/2.0", " (compatible)", " (Windows)", " (Linux)", " (Mac)"}

	stressCount := 0
	for _, prefix := range prefixes {
		for _, suffix := range suffixes {
			for _, variation := range variations {
				for i := 0; i < 100; i++ {
					input := fmt.Sprintf("%s%s%s-%d", prefix, suffix, variation, i)
					hash := XXHash64sum(input)
					if existing, exists := xxhashHashes[hash]; exists {
						t.Errorf("XXHash collision in stress test: %q and %q both hash to %s", input, existing, hash)
					}
					xxhashHashes[hash] = input
					stressCount++
				}
			}
		}
	}
	t.Logf("Stress test 1: %d similar string variations, no collisions", stressCount)

	// Test sequential patterns that might be problematic
	patterns := []string{
		"192.168.1.%d",
		"10.0.0.%d",
		"172.16.%d.1",
		"challenge-%d",
		"bot-rule-%d",
		"policy-%016x",
		"session-%016x",
	}

	seqCount := 0
	for _, pattern := range patterns {
		for i := 0; i < 10000; i++ {
			input := fmt.Sprintf(pattern, i)
			hash := XXHash64sum(input)
			if existing, exists := xxhashHashes[hash]; exists {
				t.Errorf("XXHash collision in sequential test: %q and %q both hash to %s", input, existing, hash)
			}
			xxhashHashes[hash] = input
			seqCount++
		}
	}
	t.Logf("Stress test 2: %d sequential patterns, no collisions", seqCount)

	totalInputs := len(allInputs) + stressCount + seqCount
	t.Logf("TOTAL: Tested %d inputs across realistic scenarios - NO COLLISIONS", totalInputs)
}

// Verify xxhash output works as cache keys
func TestXXHashFormat(t *testing.T) {
	testCases := []string{
		"short",
		"",
		"very long string with lots of content that might be used in policy checking and other internal hashing scenarios",
		"User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	}

	for _, input := range testCases {
		hash := XXHash64sum(input)

		// Check it's valid hex
		if len(hash) == 0 {
			t.Errorf("Empty hash for input %q", input)
		}

		// xxhash is 64-bit so max 16 hex chars
		if len(hash) > 16 {
			t.Errorf("Hash too long for input %q: %s (length %d)", input, hash, len(hash))
		}

		// Make sure it's all hex characters
		for _, char := range hash {
			if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
				t.Errorf("Non-hex character %c in hash %s for input %q", char, hash, input)
			}
		}

		t.Logf("Input: %q -> Hash: %s", input, hash)
	}
}
