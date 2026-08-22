package decoder

import "testing"

func TestActivityChangeInfoDecodeCrewBit(t *testing.T) {
	tests := []struct {
		name string
		raw  ActivityChangeInfo
		team bool
	}{
		{name: "single", raw: ActivityChangeInfo{0x00, 0x00}, team: false},
		{name: "team", raw: ActivityChangeInfo{0x40, 0x00}, team: true},
		{name: "team with other fields", raw: ActivityChangeInfo{0x7b, 0xff}, team: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.raw.Decode().Team; got != tt.team {
				t.Fatalf("Team = %v, want %v", got, tt.team)
			}
		})
	}
}

// The c bit is dual-meaning per Annex 1B/1C Appendix 1 §2.1: with p=1 (card not
// inserted) it encodes activity status (0=UNKNOWN '?', 1=KNOWN manual entry),
// not crew. The decoder intentionally exposes it as Team; these cases pin the
// four p/c combinations so the raw bits survive decoding unambiguously.
func TestActivityChangeInfoDecodePCCombinations(t *testing.T) {
	tests := []struct {
		name        string
		raw         ActivityChangeInfo
		team        bool
		cardPresent bool
	}{
		// p=0,c=0: recorded, single crew
		{name: "p0 c0 recorded single", raw: ActivityChangeInfo{0x00, 0x00}, team: false, cardPresent: true},
		// p=0,c=1: recorded, crew
		{name: "p0 c1 recorded crew", raw: ActivityChangeInfo{0x40, 0x00}, team: true, cardPresent: true},
		// p=1,c=0: card records -> UNKNOWN ('?')
		{name: "p1 c0 unknown marker", raw: ActivityChangeInfo{0x20, 0x00}, team: false, cardPresent: false},
		// p=1,c=1: card records -> KNOWN (manual entry)
		{name: "p1 c1 manual entry", raw: ActivityChangeInfo{0x60, 0x00}, team: true, cardPresent: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := tt.raw.Decode()
			if d.Team != tt.team {
				t.Fatalf("Team = %v, want %v", d.Team, tt.team)
			}
			if d.CardPresent != tt.cardPresent {
				t.Fatalf("CardPresent = %v, want %v", d.CardPresent, tt.cardPresent)
			}
		})
	}
}
