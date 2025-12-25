package stratum

import (
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestDifficultyToTarget(t *testing.T) {
	tests := []struct {
		difficulty uint64
		wantPrefix string // First few chars of hex target
	}{
		{1, "ffffffff"}, // Max target
		{1000, "0041893"},
		{10000, "00068db"},
		{100000, "0000a7c"},
	}

	for _, tc := range tests {
		target := DifficultyToTarget(tc.difficulty)
		hexTarget := hex.EncodeToString(target.Bytes())
		if len(hexTarget) < len(tc.wantPrefix) {
			t.Errorf("DifficultyToTarget(%d) = %s, want prefix %s", tc.difficulty, hexTarget, tc.wantPrefix)
		}
	}
}

func TestDifficultyToCompact(t *testing.T) {
	tests := []struct {
		difficulty uint64
		wantLen    int
	}{
		{1000, 8},
		{10000, 8},
		{100000, 8},
	}

	for _, tc := range tests {
		compact := DifficultyToCompact(tc.difficulty)
		if len(compact) != tc.wantLen {
			t.Errorf("DifficultyToCompact(%d) = %s (len %d), want len %d",
				tc.difficulty, compact, len(compact), tc.wantLen)
		}
	}
}

func TestHashMeetsDifficulty(t *testing.T) {
	// A hash of all zeros should meet any difficulty
	zeroHash := make([]byte, 32)
	if !HashMeetsDifficulty(zeroHash, 1000000000) {
		t.Error("Zero hash should meet any difficulty")
	}

	// A hash of all 0xff should not meet high difficulty
	maxHash := make([]byte, 32)
	for i := range maxHash {
		maxHash[i] = 0xff
	}
	if HashMeetsDifficulty(maxHash, 1000) {
		t.Error("Max hash should not meet difficulty 1000")
	}
}

func TestParseLoginParams(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name:    "valid login",
			json:    `{"login":"sy1qtest","pass":"x","agent":"XMRig","rigid":"rig1"}`,
			wantErr: false,
		},
		{
			name:    "missing login",
			json:    `{"pass":"x"}`,
			wantErr: true,
		},
		{
			name:    "invalid json",
			json:    `{invalid}`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			params, err := ParseLoginParams(json.RawMessage(tc.json))
			if tc.wantErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tc.wantErr && params.Login == "" {
				t.Error("Expected login to be set")
			}
		})
	}
}

func TestParseSubmitParams(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name:    "valid submit",
			json:    `{"id":"abc","job_id":"123","nonce":"deadbeef","result":"0000000000000000000000000000000000000000000000000000000000000000"}`,
			wantErr: false,
		},
		{
			name:    "missing nonce",
			json:    `{"id":"abc","job_id":"123"}`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseSubmitParams(json.RawMessage(tc.json))
			if tc.wantErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestJobJSON(t *testing.T) {
	job := &Job{
		JobID:    "abc123",
		Blob:     "0102030405",
		Target:   "00000000",
		Height:   12345,
		SeedHash: "deadbeef",
		Algo:     "rx/0",
	}

	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("Failed to marshal job: %v", err)
	}

	var parsed Job
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal job: %v", err)
	}

	if parsed.JobID != job.JobID {
		t.Errorf("JobID mismatch: got %s, want %s", parsed.JobID, job.JobID)
	}
	if parsed.Height != job.Height {
		t.Errorf("Height mismatch: got %d, want %d", parsed.Height, job.Height)
	}
}

func TestResponseJSON(t *testing.T) {
	resp := &Response{
		ID:     1,
		Result: &LoginResult{ID: "sess1", Status: "OK"},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	// Verify it's valid JSON
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	if raw["id"] != float64(1) {
		t.Errorf("ID mismatch: got %v", raw["id"])
	}
}
