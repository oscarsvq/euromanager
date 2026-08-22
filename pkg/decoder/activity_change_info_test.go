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
