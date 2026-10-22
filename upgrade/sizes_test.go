package upgrade

import "testing"

func TestParseTabSizes(t *testing.T) {
	got := parseTabSizes("libddcutil.x86_64\t289000\t1200000\nbash.x86_64\t8873537\nbroken\n")
	if got["libddcutil.x86_64"] != (upgradeSize{289000, 1200000}) || got["bash.x86_64"].download != 8873537 {
		t.Fatalf("got %+v", got)
	}
	if _, ok := got["broken"]; ok {
		t.Fatal("line without a size should be skipped")
	}
}
