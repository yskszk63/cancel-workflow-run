package main

import (
	"testing"
)

func TestIntoJson(t *testing.T) {
	cases := []struct {
		name string
		in   string
		out  string
	}{
		{
			name: "empty",
			in:   "",
			out:  `""`,
		},
		{
			name: "obj",
			in:   "{}",
			out:  "{}",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := intoJson([]byte(c.in))
			if string(r) != c.out {
				t.Fatalf("%s != %s", r, c.out)
			}
		})
	}
}
