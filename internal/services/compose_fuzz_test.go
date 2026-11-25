package services

import "testing"

func FuzzParseComposeUnits(f *testing.F) {
	f.Add("services:\n  web:\n    image: nginx:alpine\n")
	f.Add(":")
	f.Add("services: []")
	f.Add("services:\n  a:\n    image: ['nope']\n")
	f.Fuzz(func(t *testing.T, doc string) {
		if len(doc) > 1<<16 {
			t.Skip()
		}
		units := ParseComposeUnits(doc)
		seen := map[string]struct{}{}
		for _, u := range units {
			if u.Name == "" {
				t.Fatal("empty service name")
			}
			if _, ok := seen[u.Name]; ok {
				t.Fatalf("duplicate service %q", u.Name)
			}
			seen[u.Name] = struct{}{}
		}
		_ = ParseComposeVolumes(doc)
	})
}
