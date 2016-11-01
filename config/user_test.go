package config

import (
	"testing"

	yaml "gopkg.in/yaml.v2"
)

func TestUnmarshal(t *testing.T) {
	yml := []byte(`
can_view_num_media: true
can_view_messages: false
`)
	us := new(UserSettings)
	if err := yaml.Unmarshal(yml, us); err != nil {
		t.Fatal(err)
	}
	if us.CanViewNumMedia == false {
		t.Errorf("expected CanViewNumMedia to be true, got false")
	}
	if us.CanViewMessages == true {
		t.Errorf("expected CanViewMessages to be false, got true")
	}
	// unspecified should default to true
	if us.CanViewMessageFrom == false {
		t.Errorf("expected CanViewMessageFrom to be true, got false")
	}
}
