package auth

import "testing"

// A deployment behind a proxy that hides every client's address raises the
// budget; anything else keeps the core's constants.
func TestLoginBudgetFollowsTheDeployment(t *testing.T) {
	t.Setenv("LOGIN_RATE_PER_MINUTE", "")
	t.Setenv("LOGIN_BURST", "")
	if perMinute, burst := LoginBudget(); perMinute != LoginRatePerMinute || burst != LoginBurst {
		t.Fatalf("unset budget = %d/%d, want the constants %d/%d", perMinute, burst, LoginRatePerMinute, LoginBurst)
	}

	t.Setenv("LOGIN_RATE_PER_MINUTE", " 300 ")
	t.Setenv("LOGIN_BURST", "100")
	if perMinute, burst := LoginBudget(); perMinute != 300 || burst != 100 {
		t.Fatalf("configured budget = %d/%d, want 300/100", perMinute, burst)
	}

	// A value that is not a positive whole number must not switch the brake off.
	for _, bad := range []string{"0", "-5", "lots", "1.5"} {
		t.Setenv("LOGIN_RATE_PER_MINUTE", bad)
		t.Setenv("LOGIN_BURST", bad)
		if perMinute, burst := LoginBudget(); perMinute != LoginRatePerMinute || burst != LoginBurst {
			t.Errorf("budget %q = %d/%d, want the constants", bad, perMinute, burst)
		}
	}
}
