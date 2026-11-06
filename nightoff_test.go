package main

import "testing"

func TestNormalizeHHMM(t *testing.T) {
	valid := map[string]string{
		"22:00":   "22:00",
		"06:05":   "06:05",
		" 22:00 ": "22:00",
		"00:00":   "00:00",
		"23:59":   "23:59",
	}
	for in, want := range valid {
		got, ok := normalizeHHMM(in)
		if !ok || got != want {
			t.Errorf("normalizeHHMM(%q) = (%q, %v), want (%q, true)", in, got, ok, want)
		}
	}

	invalid := []string{
		"",
		"24:00",
		"22:60",
		"9:5",
		"10:30abc",
		"10:30'; Start-Process calc; '",
		"22-00",
	}
	for _, in := range invalid {
		if got, ok := normalizeHHMM(in); ok {
			t.Errorf("normalizeHHMM(%q) = %q, want rejected", in, got)
		}
	}
}

func TestParseNightOffResult(t *testing.T) {
	// Windows PowerShell 5.1 writes a UTF-8 BOM; success must still be
	// recognised.
	if err := parseNightOffResult([]byte("\ufeffOK\r\n")); err != nil {
		t.Fatalf("BOM + OK: %v", err)
	}
	if err := parseNightOffResult([]byte("OK:Up\r\n")); err != nil {
		t.Fatalf("OK:Up: %v", err)
	}
	err := parseNightOffResult([]byte("\ufeffERROR: access denied\r\n"))
	if err == nil || err.Error() != "access denied" {
		t.Fatalf("ERROR result = %v, want %q", err, "access denied")
	}
	if err := parseNightOffResult(nil); err == nil {
		t.Fatal("expected an error for an empty result")
	}
	if err := parseNightOffResult([]byte("garbage")); err == nil || err.Error() != "garbage" {
		t.Fatalf("garbage result = %v", err)
	}
}
