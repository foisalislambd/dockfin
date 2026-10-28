package backup

import "testing"

func TestRedactS3Message(t *testing.T) {
	got := RedactS3Message("s3 upload: exit https://AKIAexample:secret-key@s3.example.com/bucket denied")
	if got != "s3 upload: exit https://s3.example.com/bucket denied" {
		t.Fatalf("%q", got)
	}
}
