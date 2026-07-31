package services

import "testing"

func TestParseComposeVolumes(t *testing.T) {
	raw := `
services:
  app:
    image: nginx
    volumes:
      - data:/var/lib/data
      - ./cfg:/etc/cfg:ro
  db:
    image: postgres
    volumes:
      - type: volume
        source: pg
        target: /var/lib/postgresql/data
`
	vols := ParseComposeVolumes(raw)
	if len(vols) != 3 {
		t.Fatalf("want 3 got %d %#v", len(vols), vols)
	}
}
