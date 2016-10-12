package services

import "testing"

var npurl = "/2010-04-01/Accounts/AC58f1e8f2b1c6b88ca90a012a4be0c279/Messages.json?PageSize=50&Page=1&PageToken=PASM1f753eba6c2942858fd0be4608ead788"

// This test doesn't really do anything
func TestShorter(t *testing.T) {
	out, err := Shorter(npurl)
	if err != nil {
		t.Fatal(err)
	}
	exp := Unshorter(out)
	if exp != npurl {
		t.Fatalf("expected Unshorter(Shorter(%v)) to be the same, got %v", npurl, exp)
	}
}
