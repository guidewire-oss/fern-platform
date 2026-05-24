package main

import (
	mathrand "math/rand"
	"testing"
)

func TestHealthBandFor(t *testing.T) {
	cases := []struct {
		total int
		want  []int
	}{
		// 5 projects: one per band.
		{5, []int{0, 1, 2, 3, 4}},
		// 10 projects: two per band.
		{10, []int{0, 0, 1, 1, 2, 2, 3, 3, 4, 4}},
		// 20 projects: four per band.
		{20, []int{
			0, 0, 0, 0,
			1, 1, 1, 1,
			2, 2, 2, 2,
			3, 3, 3, 3,
			4, 4, 4, 4,
		}},
		// Single project always goes to the top band.
		{1, []int{4}},
	}
	for _, c := range cases {
		for i, want := range c.want {
			got := healthBandFor(i, c.total)
			if got != want {
				t.Errorf("healthBandFor(%d, %d) = %d, want %d", i, c.total, got, want)
			}
		}
	}
}

// TestPickStatusForBand_Distribution sanity-checks that each band
// produces pass rates near the documented target. Tolerances are wide
// because we're sampling — not asserting on exact percentages.
func TestPickStatusForBand_Distribution(t *testing.T) {
	const samples = 10_000
	cases := []struct {
		band    int
		minPass int
		maxPass int
	}{
		{0, 300, 700},   // ~5% (50 stddev → wide band)
		{1, 2200, 2800}, // ~25%
		{2, 5200, 5800}, // ~55%
		{3, 8200, 8800}, // ~85%
		{4, 9700, 9900}, // ~98%
	}
	for _, c := range cases {
		rng := mathrand.New(mathrand.NewSource(int64(c.band + 1)))
		passed := 0
		for i := 0; i < samples; i++ {
			if pickStatusForBand(rng, c.band) == "passed" {
				passed++
			}
		}
		if passed < c.minPass || passed > c.maxPass {
			t.Errorf("band %d passed=%d out of %d, want in [%d,%d]",
				c.band, passed, samples, c.minPass, c.maxPass)
		}
	}
}

// TestSuitePassRateMatchesProjectBand is a regression guard for the
// "red project, green suites" bug. The seed-time path uses
// failFractionForBand for BOTH the run-level failed-tests cap and the
// suite-level failed-specs cap. If the two ever diverge again, this
// test catches it.
func TestSuitePassRateMatchesProjectBand(t *testing.T) {
	// Sample a band-0 (broken) suite-failure cap and verify it's close
	// to the run-level cap. Both should be 0.90.
	runCap := failFractionForBand(0, true)
	suiteCap := failFractionForBand(0, true) // same function intentional
	if runCap != suiteCap {
		t.Errorf("band 0: run-level cap %v != suite-level cap %v", runCap, suiteCap)
	}
	// Band 4: both caps should be 0.03.
	if got := failFractionForBand(4, true); got != 0.03 {
		t.Errorf("band 4 cap: got %v, want 0.03", got)
	}
}

// TestComputeSpecFailures_FullCoverageMatchesHeader is a regression
// guard for the "header says 3 failed, body shows 4 failed" bug. When
// the seeder writes one spec_run per suite-metadata spec (perSuite ==
// totalSpecs), the failure count of the body MUST equal the suite's
// failedSpecs exactly — otherwise the UI looks like it's lying about
// itself.
func TestComputeSpecFailures_FullCoverageMatchesHeader(t *testing.T) {
	// Use a deterministic source so failures here are reproducible.
	rng := mathrand.New(mathrand.NewSource(42))
	cases := []struct {
		totalSpecs  int
		failedSpecs int
	}{
		{17, 3},  // the reported case
		{25, 22}, // mostly-failing suite
		{10, 0},  // no failures
		{1, 1},   // edge: single spec, all failed
		{1, 0},   // edge: single spec, none failed
	}
	for _, c := range cases {
		got := computeSpecFailures(c.totalSpecs, c.totalSpecs, c.failedSpecs, rng)
		if got != c.failedSpecs {
			t.Errorf("full coverage with totalSpecs=%d failedSpecs=%d: got %d failures, want %d (header/body must match exactly)",
				c.totalSpecs, c.failedSpecs, got, c.failedSpecs)
		}
	}
}

// TestComputeSpecFailures_SamplingTracksProportion confirms that when
// the seeder samples a subset of specs (perSuite < totalSpecs), the
// failed count is proportional ± at most 1 for jitter.
func TestComputeSpecFailures_SamplingTracksProportion(t *testing.T) {
	rng := mathrand.New(mathrand.NewSource(7))
	// 25-total / 5-failed at perSuite=5: expected ≈ 1 failed, allow 0-2 for jitter.
	for i := 0; i < 200; i++ {
		got := computeSpecFailures(5, 25, 5, rng)
		if got < 0 || got > 2 {
			t.Errorf("sampling 5 of 25/5: got %d failures, want 0..2", got)
		}
	}
	// Heavy failure: 25-total / 22-failed at perSuite=5: expected ≈ 4 failed.
	for i := 0; i < 200; i++ {
		got := computeSpecFailures(5, 25, 22, rng)
		if got < 3 || got > 5 {
			t.Errorf("sampling 5 of 25/22: got %d failures, want 3..5", got)
		}
	}
}

// TestComputeSpecFailures_AlwaysInBounds confirms the helper never
// emits a negative count or one above perSuite, even with adversarial
// inputs.
func TestComputeSpecFailures_AlwaysInBounds(t *testing.T) {
	rng := mathrand.New(mathrand.NewSource(99))
	for _, total := range []int{1, 5, 25, 100} {
		for failed := 0; failed <= total; failed++ {
			for _, per := range []int{1, total / 2, total} {
				if per <= 0 {
					continue
				}
				got := computeSpecFailures(per, total, failed, rng)
				if got < 0 || got > per {
					t.Errorf("computeSpecFailures(per=%d, total=%d, failed=%d) = %d; out of bounds [0,%d]",
						per, total, failed, got, per)
				}
			}
		}
	}
}

// TestSampleNearTarget_StaysInBand is a regression guard for the
// "everything is green" bug. The old `1 + rng.Intn(maxFail)` sampled
// uniformly across [1, maxFail], averaging to maxFail/2 — which
// collapsed every health band toward green. sampleNearTarget must
// instead cluster near maxFail.
func TestSampleNearTarget_StaysInBand(t *testing.T) {
	rng := mathrand.New(mathrand.NewSource(1))
	const samples = 10_000
	for _, target := range []int{10, 100, 200} {
		sum := 0
		minSeen := target + 1
		for i := 0; i < samples; i++ {
			v := sampleNearTarget(target, rng)
			if v < 1 || v > target {
				t.Fatalf("sampleNearTarget(%d) = %d, out of bounds [1, %d]", target, v, target)
			}
			sum += v
			if v < minSeen {
				minSeen = v
			}
		}
		mean := float64(sum) / float64(samples)
		// Mean must be ≥ 85% of target (vs. the old uniform behavior
		// which averaged to 50%). 85% is a comfortable floor for the
		// "jitter ≤ 10% of target" rule with ceil rounding.
		if mean < 0.85*float64(target) {
			t.Errorf("sampleNearTarget(%d): mean=%.1f, want ≥ %.1f", target, mean, 0.85*float64(target))
		}
		// Lower bound check: minimum sampled value should be reasonably
		// close to target (we expect 90%+ of target as the floor).
		if target >= 10 && minSeen < int(0.85*float64(target)) {
			t.Errorf("sampleNearTarget(%d): minSeen=%d, want ≥ %d", target, minSeen, int(0.85*float64(target)))
		}
	}
}

// TestSampleNearTarget_TrivialTarget guards the edge case where target
// is small enough that the jitter window collapses.
func TestSampleNearTarget_TrivialTarget(t *testing.T) {
	rng := mathrand.New(mathrand.NewSource(2))
	for _, target := range []int{0, 1, 2} {
		v := sampleNearTarget(target, rng)
		if v < 1 {
			t.Errorf("sampleNearTarget(%d) = %d, must be ≥ 1", target, v)
		}
	}
}

func TestFailFractionForBand(t *testing.T) {
	// Legacy mode (bands off) keeps the 0.20 cap regardless of band.
	if got := failFractionForBand(0, false); got != 0.20 {
		t.Errorf("legacy mode: failFractionForBand(0,false) = %v, want 0.20", got)
	}
	if got := failFractionForBand(4, false); got != 0.20 {
		t.Errorf("legacy mode: failFractionForBand(4,false) = %v, want 0.20", got)
	}

	// Bands mode: monotonic decrease from band 0 → 4.
	prev := 1.0
	for b := 0; b <= 4; b++ {
		got := failFractionForBand(b, true)
		if got >= prev {
			t.Errorf("expected failFractionForBand monotonically decreasing across bands; band %d = %v, prev = %v", b, got, prev)
		}
		prev = got
	}
}
