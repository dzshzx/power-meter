package fonts

import "testing"

func TestRegister(t *testing.T) {
	if err := Register(); err != nil {
		t.Fatal(err)
	}
	if err := Register(); err != nil {
		t.Fatal("registering twice keeps the first result:", err)
	}
}
