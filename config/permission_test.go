package config

import (
	"testing"

	yaml "gopkg.in/yaml.v2"
)

var policy = []byte(`
- name: support
  permissions:
    can_view_num_media: false
    can_view_calls: false
  users:
    - test@example.com
    - test@example.net

- name: eng
  permissions:
    can_view_num_media: True
    can_view_calls: True
  default: true
  users:
    - eng@example.com
    - eng@example.net

- name: empty
  permissions:
  users:
    - empty@example.com
`)

func TestLoadPermission(t *testing.T) {
	t.Parallel()
	var p Policy
	if err := yaml.Unmarshal(policy, &p); err != nil {
		t.Fatal(err)
	}
	if len(p) != 3 {
		t.Errorf("expected 3 groups, got %d", len(p))
	}
	if p[0].Name != "support" {
		t.Errorf("expected name to equal 'support', got %s", p[0].Name)
	}
	if p[0].Users[0] != "test@example.com" {
		t.Errorf("expected first user to be test@example.com, got %s", p[0].Users[0])
	}
	if p[0].Permissions.CanViewNumMedia == true {
		t.Errorf("expected CanViewNumMedia to be false, got true")
	}
	if p[0].Permissions.CanViewCalls == true {
		t.Errorf("expected CanViewCalls to be false, got true")
	}
	if p[0].Default == true {
		t.Error("expected Default to be false, got true")
	}
	if p[1].Default == false {
		t.Error("expected Default to be true, got false")
	}
	if p[2].Permissions.CanViewCalls == false {
		t.Errorf("expected CanViewCalls to be true, got false")
	}
}

var policyTests = []struct {
	p   *Policy
	err string
}{
	{p: &Policy{
		&Group{Name: "1", Users: []string{"foo"}},
		&Group{Name: "2", Users: []string{"foo"}},
	},
		err: "User foo appears twice in the list"},
	{p: &Policy{
		&Group{Name: "1", Users: []string{"foo"}},
		&Group{Name: "1", Users: []string{"two"}},
	},
		err: "Group name 1 appears twice in the list"},
	{p: &Policy{
		&Group{Name: "1", Default: true, Users: []string{"foo"}},
		&Group{Name: "2", Default: true, Users: []string{"two"}},
	},
		err: "More than one group marked as default"},
	{p: &Policy{
		&Group{Name: "", Default: true, Users: []string{"foo"}},
		&Group{Name: "2", Default: false, Users: []string{"two"}},
	},
		err: "Group has no name, define a group name"},
	{p: &Policy{
		&Group{Name: "1", Default: true, Users: []string{"foo"}},
		&Group{Name: "2", Default: false, Users: []string{"two"}},
	}, err: ""},
}

func TestValidatePolicy(t *testing.T) {
	t.Parallel()
	for _, tt := range policyTests {
		err := validatePolicy(tt.p)
		if err == nil && tt.err != "" {
			t.Errorf("validatePolicy(%#v): got nil error, expected %s", tt.p, tt.err)

		}
		if err != nil {
			if tt.err == "" {
				t.Errorf("got non-nil error %v but expected nil", err)
			}
			if err.Error() != tt.err {
				t.Errorf("got wrong error: %v, want %s", err, tt.err)
			}
		}
	}
}
