package validation

import (
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"unicode"
)

// Validation constants
const (
	MaxWorkerLength  = 64
	MaxAgentLength   = 128
	MaxRigIDLength   = 64
	MaxNonceLength   = 16
	MaxResultLength  = 64
	MaxJobIDLength   = 16
	MinAddressLength = 32
	MaxAddressLength = 128
)

// Validator provides input validation
type Validator struct {
	addressPattern *regexp.Regexp
}

// NewValidator creates a new validator
func NewValidator() *Validator {
	return &Validator{
		// OpenSY address pattern (similar to Bitcoin/Monero style)
		addressPattern: regexp.MustCompile(`^[a-zA-Z0-9]{32,128}$`),
	}
}

// Validation errors
var (
	// Address errors
	ErrEmptyAddress         = errors.New("address is required")
	ErrAddressTooShort      = errors.New("address is too short")
	ErrAddressTooLong       = errors.New("address exceeds maximum length")
	ErrInvalidAddressFormat = errors.New("invalid address format")

	// Worker errors
	ErrWorkerTooLong     = errors.New("worker name exceeds maximum length")
	ErrInvalidWorkerName = errors.New("worker name contains invalid characters")

	// Agent errors
	ErrAgentTooLong = errors.New("agent exceeds maximum length")

	// Rig ID errors
	ErrRigIDTooLong = errors.New("rig ID exceeds maximum length")

	// Nonce errors
	ErrEmptyNonce   = errors.New("nonce is required")
	ErrNonceTooLong = errors.New("nonce exceeds maximum length")
	ErrInvalidNonce = errors.New("nonce is not valid hex")

	// Result errors
	ErrEmptyResult         = errors.New("result is required")
	ErrResultTooLong       = errors.New("result exceeds maximum length")
	ErrInvalidResultLength = errors.New("result must be 64 hex characters")
	ErrInvalidResult       = errors.New("result is not valid hex")

	// Job ID errors
	ErrEmptyJobID   = errors.New("job ID is required")
	ErrJobIDTooLong = errors.New("job ID exceeds maximum length")
	ErrInvalidJobID = errors.New("job ID is not valid hex")

	// Difficulty errors
	ErrDifficultyTooLow  = errors.New("difficulty is below minimum")
	ErrDifficultyTooHigh = errors.New("difficulty exceeds maximum")
)

// ValidateLogin validates login parameters
func (v *Validator) ValidateLogin(login, worker, agent, rigID string) error {
	// Login (wallet address)
	if err := v.ValidateAddress(login); err != nil {
		return err
	}

	// Worker name
	if len(worker) > MaxWorkerLength {
		return ErrWorkerTooLong
	}
	if worker != "" && !v.isSafeString(worker) {
		return ErrInvalidWorkerName
	}

	// Agent
	if len(agent) > MaxAgentLength {
		return ErrAgentTooLong
	}

	// Rig ID
	if len(rigID) > MaxRigIDLength {
		return ErrRigIDTooLong
	}

	return nil
}

// ValidateAddress validates a wallet address
func (v *Validator) ValidateAddress(address string) error {
	if address == "" {
		return ErrEmptyAddress
	}

	// Remove worker suffix if present (format: address.worker)
	if idx := strings.Index(address, "."); idx > 0 {
		address = address[:idx]
	}

	if len(address) < MinAddressLength {
		return ErrAddressTooShort
	}
	if len(address) > MaxAddressLength {
		return ErrAddressTooLong
	}
	if !v.addressPattern.MatchString(address) {
		return ErrInvalidAddressFormat
	}

	return nil
}

// ValidateNonce validates a nonce string
func (v *Validator) ValidateNonce(nonce string) error {
	if nonce == "" {
		return ErrEmptyNonce
	}
	if len(nonce) > MaxNonceLength {
		return ErrNonceTooLong
	}
	// Must be valid hex
	if _, err := hex.DecodeString(nonce); err != nil {
		return ErrInvalidNonce
	}
	return nil
}

// ValidateResult validates a hash result
func (v *Validator) ValidateResult(result string) error {
	if result == "" {
		return ErrEmptyResult
	}
	if len(result) > MaxResultLength {
		return ErrResultTooLong
	}
	// Must be 64 hex characters (32 bytes = 256 bits)
	if len(result) != 64 {
		return ErrInvalidResultLength
	}
	if _, err := hex.DecodeString(result); err != nil {
		return ErrInvalidResult
	}
	return nil
}

// ValidateJobID validates a job ID
func (v *Validator) ValidateJobID(jobID string) error {
	if jobID == "" {
		return ErrEmptyJobID
	}
	if len(jobID) > MaxJobIDLength {
		return ErrJobIDTooLong
	}
	// Must be valid hex
	if _, err := hex.DecodeString(jobID); err != nil {
		return ErrInvalidJobID
	}
	return nil
}

// ValidateDifficulty validates a difficulty value
func (v *Validator) ValidateDifficulty(diff uint64, min, max uint64) error {
	if diff < min {
		return ErrDifficultyTooLow
	}
	if diff > max {
		return ErrDifficultyTooHigh
	}
	return nil
}

// isSafeString checks if a string contains only safe characters
func (v *Validator) isSafeString(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' && r != '.' {
			return false
		}
	}
	return true
}

// SanitizeWorkerName sanitizes a worker name
func SanitizeWorkerName(name string) string {
	if name == "" {
		return "default"
	}

	// Remove unsafe characters
	var safe strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			safe.WriteRune(r)
		}
	}

	result := safe.String()
	if result == "" {
		return "default"
	}
	if len(result) > MaxWorkerLength {
		result = result[:MaxWorkerLength]
	}
	return result
}

// SanitizeAgent sanitizes an agent string
func SanitizeAgent(agent string) string {
	if len(agent) > MaxAgentLength {
		agent = agent[:MaxAgentLength]
	}
	// Remove control characters
	var safe strings.Builder
	for _, r := range agent {
		if r >= 32 && r < 127 {
			safe.WriteRune(r)
		}
	}
	return safe.String()
}
