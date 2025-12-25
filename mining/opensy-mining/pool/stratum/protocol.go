// Package stratum implements the Stratum mining protocol for OpenSY.
// This implements Stratum v1 which is compatible with XMRig and other
// RandomX mining software.
package stratum

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
)

// JSON-RPC message types

// Request represents an incoming JSON-RPC request from a miner
type Request struct {
	ID     interface{}     `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// Response represents an outgoing JSON-RPC response to a miner
type Response struct {
	ID     interface{} `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *Error      `json:"error,omitempty"`
}

// Notification represents a server-to-client notification (no id expected back)
type Notification struct {
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

// Error represents a JSON-RPC error
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Standard error codes
var (
	ErrUnknown        = &Error{Code: -1, Message: "Unknown error"}
	ErrInvalidRequest = &Error{Code: -2, Message: "Invalid request"}
	ErrJobNotFound    = &Error{Code: -3, Message: "Job not found"}
	ErrDuplicateShare = &Error{Code: -4, Message: "Duplicate share"}
	ErrLowDifficulty  = &Error{Code: -5, Message: "Low difficulty share"}
	ErrUnauthorized   = &Error{Code: -6, Message: "Unauthorized worker"}
	ErrNotSubscribed  = &Error{Code: -7, Message: "Not subscribed"}
)

// Stratum method names
const (
	MethodLogin     = "login"
	MethodSubmit    = "submit"
	MethodKeepAlive = "keepalived"

	// Server-to-client methods
	MethodJob = "job"
)

// LoginParams represents login request parameters
type LoginParams struct {
	Login string `json:"login"` // Wallet address or username
	Pass  string `json:"pass"`  // Password (often "x" or worker name)
	Agent string `json:"agent"` // Mining software identifier
	RigID string `json:"rigid"` // Optional rig identifier
}

// LoginResult represents a successful login response
type LoginResult struct {
	ID     string `json:"id"`     // Session ID
	Job    *Job   `json:"job"`    // First job to work on
	Status string `json:"status"` // "OK"
}

// Job represents a mining job sent to miners
type Job struct {
	JobID    string `json:"job_id"`    // Unique job identifier
	Blob     string `json:"blob"`      // Block header template (hex)
	Target   string `json:"target"`    // Mining target (hex, compact)
	Height   int64  `json:"height"`    // Block height
	SeedHash string `json:"seed_hash"` // RandomX seed hash
	Algo     string `json:"algo"`      // Algorithm (rx/0 for RandomX)
}

// SubmitParams represents share submission parameters
type SubmitParams struct {
	ID     string `json:"id"`     // Session ID
	JobID  string `json:"job_id"` // Job this share is for
	Nonce  string `json:"nonce"`  // Nonce found (hex, 4 bytes)
	Result string `json:"result"` // Hash result (hex, 32 bytes)
}

// SubmitResult represents a share submission response
type SubmitResult struct {
	Status string `json:"status"` // "OK" or error
}

// Difficulty utilities

// DifficultyToTarget converts a difficulty value to a 256-bit target.
// Target = MaxTarget / Difficulty
// Where MaxTarget = 2^256 - 1
func DifficultyToTarget(difficulty uint64) *big.Int {
	if difficulty == 0 {
		difficulty = 1
	}

	// MaxTarget for RandomX is 2^256 - 1
	maxTarget := new(big.Int)
	maxTarget.SetString("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", 16)

	diff := new(big.Int).SetUint64(difficulty)
	target := new(big.Int).Div(maxTarget, diff)

	return target
}

// TargetToCompact converts a 256-bit target to a compact hex string (8 chars).
// This is the format miners expect for the "target" field.
func TargetToCompact(target *big.Int) string {
	// Take the first 4 bytes of the target (big endian)
	bytes := target.Bytes()

	// Pad to 32 bytes if necessary
	if len(bytes) < 32 {
		padded := make([]byte, 32)
		copy(padded[32-len(bytes):], bytes)
		bytes = padded
	}

	// First 4 bytes as little-endian hex
	compact := make([]byte, 4)
	compact[0] = bytes[31]
	compact[1] = bytes[30]
	compact[2] = bytes[29]
	compact[3] = bytes[28]

	return hex.EncodeToString(compact)
}

// DifficultyToCompact converts difficulty to compact target string
func DifficultyToCompact(difficulty uint64) string {
	target := DifficultyToTarget(difficulty)
	return TargetToCompact(target)
}

// HashMeetsDifficulty checks if a hash meets the required difficulty
func HashMeetsDifficulty(hash []byte, difficulty uint64) bool {
	if len(hash) != 32 {
		return false
	}

	target := DifficultyToTarget(difficulty)

	// Convert hash to big.Int (little-endian)
	hashReversed := make([]byte, 32)
	for i := 0; i < 32; i++ {
		hashReversed[i] = hash[31-i]
	}
	hashInt := new(big.Int).SetBytes(hashReversed)

	// Hash must be <= target
	return hashInt.Cmp(target) <= 0
}

// ParseLoginParams parses login parameters from JSON
func ParseLoginParams(params json.RawMessage) (*LoginParams, error) {
	var p LoginParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid login params: %w", err)
	}
	if p.Login == "" {
		return nil, fmt.Errorf("login required")
	}
	return &p, nil
}

// ParseSubmitParams parses submit parameters from JSON
func ParseSubmitParams(params json.RawMessage) (*SubmitParams, error) {
	var p SubmitParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid submit params: %w", err)
	}
	if p.ID == "" || p.JobID == "" || p.Nonce == "" || p.Result == "" {
		return nil, fmt.Errorf("incomplete submit params")
	}
	return &p, nil
}
