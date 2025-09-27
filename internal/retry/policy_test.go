package retry

import (
    "testing"
    "time"
    "git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestDefaultPolicy verifies the baseline default values.
func TestDefaultPolicy(t *testing.T) {
    p := DefaultPolicy()
    if p.Mode != config.RetryBackoffLinear { t.Fatalf("expected linear default mode got %s", p.Mode) }
    if p.Initial != time.Second { t.Fatalf("expected initial 1s got %v", p.Initial) }
    if p.Max != 30*time.Second { t.Fatalf("expected max 30s got %v", p.Max) }
    if p.MaxRetries != 2 { t.Fatalf("expected max retries 2 got %d", p.MaxRetries) }
}

// TestNewPolicyOverrides checks override precedence and clamping when initial > max.
func TestNewPolicyOverrides(t *testing.T) {
    p := NewPolicy(config.RetryBackoffFixed, 5*time.Second, 2*time.Second, 5)
    // initial > max -> clamped
    if p.Initial != 2*time.Second { t.Fatalf("expected clamped initial 2s got %v", p.Initial) }
    if p.Max != 2*time.Second { t.Fatalf("expected max 2s got %v", p.Max) }
    if p.Mode != config.RetryBackoffFixed { t.Fatalf("expected fixed mode got %s", p.Mode) }
    if p.MaxRetries != 5 { t.Fatalf("expected maxRetries 5 got %d", p.MaxRetries) }
}

// TestDelayModes ensures fixed, linear, exponential behave and respect cap.
func TestDelayModes(t *testing.T) {
    fixed := NewPolicy(config.RetryBackoffFixed, 100*time.Millisecond, 500*time.Millisecond, 3)
    for i := 1; i <= 3; i++ {
        if d := fixed.Delay(i); d != 100*time.Millisecond {
            t.Fatalf("fixed attempt %d expected 100ms got %v", i, d)
        }
    }

    linear := NewPolicy(config.RetryBackoffLinear, 100*time.Millisecond, 250*time.Millisecond, 5)
    // attempts: 1->100ms,2->200ms,3->cap 250ms,4->cap 250ms
    cases := []struct { attempt int; want time.Duration }{{1,100*time.Millisecond},{2,200*time.Millisecond},{3,250*time.Millisecond},{4,250*time.Millisecond}}
    for _, c := range cases {
        if got := linear.Delay(c.attempt); got != c.want {
            t.Fatalf("linear attempt %d expected %v got %v", c.attempt, c.want, got)
        }
    }

    exp := NewPolicy(config.RetryBackoffExponential, 50*time.Millisecond, 160*time.Millisecond, 5)
    // 1->50,2->100,3->160 (cap),4->160
    expCases := []struct { attempt int; want time.Duration }{{1,50*time.Millisecond},{2,100*time.Millisecond},{3,160*time.Millisecond},{4,160*time.Millisecond}}
    for _, c := range expCases {
        if got := exp.Delay(c.attempt); got != c.want {
            t.Fatalf("exp attempt %d expected %v got %v", c.attempt, c.want, got)
        }
    }
}

// TestDelayEdgeCases ensures non-positive attempts yield zero and negative attempts don't panic.
func TestDelayEdgeCases(t *testing.T) {
    p := NewPolicy(config.RetryBackoffLinear, 10*time.Millisecond, 20*time.Millisecond, 1)
    if d := p.Delay(0); d != 0 { t.Fatalf("attempt 0 expected 0 got %v", d) }
    if d := p.Delay(-1); d != 0 { t.Fatalf("attempt -1 expected 0 got %v", d) }
}

// TestValidate covers validation error paths.
func TestValidate(t *testing.T) {
    badInitial := Policy{Mode: config.RetryBackoffLinear, Initial: 0, Max: time.Second, MaxRetries: 1}
    if err := badInitial.Validate(); err == nil { t.Fatalf("expected error for zero initial") }
    badMax := Policy{Mode: config.RetryBackoffLinear, Initial: time.Second, Max: 0, MaxRetries: 1}
    if err := badMax.Validate(); err == nil { t.Fatalf("expected error for zero max") }
    badRetries := Policy{Mode: config.RetryBackoffLinear, Initial: time.Second, Max: 2*time.Second, MaxRetries: -1}
    if err := badRetries.Validate(); err == nil { t.Fatalf("expected error for negative retries") }
    good := Policy{Mode: config.RetryBackoffLinear, Initial: time.Second, Max: 2*time.Second, MaxRetries: 0}
    if err := good.Validate(); err != nil { t.Fatalf("unexpected validation error: %v", err) }
}

// TestUnknownModeFallsBack leaves mode default when unknown string supplied.
func TestUnknownModeFallsBack(t *testing.T) {
    p := NewPolicy("weird", 250*time.Millisecond, 500*time.Millisecond, 1)
    if p.Mode != config.RetryBackoffLinear { t.Fatalf("unknown mode should fall back to linear got %s", p.Mode) }
}
