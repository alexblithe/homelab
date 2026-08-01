package tests

import (
	"testing"
)

func TestSanity(t *testing.T) {
	t.Run("assert_true", func(t *testing.T) {
		if true != true {
			t.Fatal("sanity check failed")
		}
	})
}
