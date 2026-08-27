package access

import (
	"testing"
)

func TestRoleCodeValidation(t *testing.T) {
	valid := []string{"admin", "sales_manager", "inventory.read"}
	invalid := []string{"A", "Admin", " has-space", "x/owner", "-admin"}
	for _, v := range valid {
		if !roleCodePattern.MatchString(v) {
			t.Errorf("expected %q valid", v)
		}
	}
	for _, v := range invalid {
		if roleCodePattern.MatchString(v) {
			t.Errorf("expected %q invalid", v)
		}
	}
}
