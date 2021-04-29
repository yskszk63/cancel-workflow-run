package main

import (
	"testing"
)

func TestNewEventGridEvent(t *testing.T) {
	e, err := newEventGridEvent("subject", "type", "0", "OK")
	if err != nil {
		t.Fatal(err)
	}

	if string(e.Data) != "\"OK\"" {
		t.Fatal(e.Data)
	}
}
