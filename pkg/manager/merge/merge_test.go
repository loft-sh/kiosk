package merge

import (
	"testing"
)

func TestBuildTemp(t *testing.T) {
	t.Skipf("This tests against a real cluster, uncomment to test")
	c := New(nil)

	before := `apiVersion: v1
kind: Service
metadata:
  name: nginx
spec:
  ports:
  - port: 80
    protocol: TCP
  selector:
    app: nginx`
	after := `apiVersion: v1
kind: Service
metadata:
  name: nginx
spec:
  ports:
  - port: 80
    protocol: TCP
  selector:
    app: nginx2`

	// Create
	err := c.Merge("", before, false)
	if err != nil {
		t.Fatal(err)
	}

	// Update
	err = c.Merge(before, after, false)
	if err != nil {
		t.Fatal(err)
	}

	// Delete
	err = c.Merge(after, "", false)
	if err != nil {
		t.Fatal(err)
	}
}
