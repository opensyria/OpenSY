// Package randomx provides Go bindings for the RandomX proof-of-work algorithm.
//
// RandomX is a CPU-friendly, ASIC-resistant proof-of-work algorithm used by
// OpenSY and other cryptocurrencies. This package wraps the C library via CGO.
//
// OpenSY-Specific Configuration:
//   - Key block interval: 32 blocks (not 2048 like Monero)
//   - Dataset regeneration: Every ~64 minutes on mainnet
//
// Thread Safety:
//   - Context initialization is NOT thread-safe
//   - VM instances are NOT thread-safe
//   - Create one VM per goroutine from a shared Context
//   - Multiple VMs can share the same dataset (read-only after init)
package randomx

/*
#cgo CFLAGS: -I${SRCDIR}/include
#cgo LDFLAGS: -L${SRCDIR}/lib -lrandomx -lstdc++ -lm
#cgo linux LDFLAGS: -lpthread
#cgo darwin LDFLAGS: -lpthread

#include <stdlib.h>
#include <randomx.h>
*/
import "C"
import (
	"errors"
	"runtime"
	"sync"
	"unsafe"
)

// HashSize is the size of a RandomX hash output in bytes.
const HashSize = 32

// KeySize is the recommended size for the RandomX key (seed).
const KeySize = 32

// OpenSY-specific constants
const (
	// KeyBlockInterval is how often the RandomX key changes in OpenSY.
	// This is 32 blocks, NOT 2048 like Monero.
	KeyBlockInterval = 32
)

// Flag represents RandomX initialization flags.
type Flag uint32

const (
	// FlagDefault uses the default configuration.
	FlagDefault Flag = 0

	// FlagLargePages uses large memory pages if available.
	FlagLargePages Flag = 1 << 0

	// FlagHardAES uses hardware AES instructions if available.
	FlagHardAES Flag = 1 << 1

	// FlagFullMem allocates the full 2GB dataset for faster hashing.
	// Required for efficient mining, optional for validation.
	FlagFullMem Flag = 1 << 2

	// FlagJIT enables JIT compilation for faster execution.
	FlagJIT Flag = 1 << 3

	// FlagSecure disables JIT for security (slower but safer).
	FlagSecure Flag = 1 << 4

	// FlagArgon2SSSE3 uses SSSE3 for Argon2.
	FlagArgon2SSSE3 Flag = 1 << 5

	// FlagArgon2AVX2 uses AVX2 for Argon2.
	FlagArgon2AVX2 Flag = 1 << 6

	// FlagArgon2 selects Argon2 implementation automatically.
	FlagArgon2 Flag = 1 << 7
)

// GetFlags returns the recommended flags for the current CPU.
func GetFlags() Flag {
	return Flag(C.randomx_get_flags())
}

// Errors returned by the RandomX package.
var (
	ErrCacheAllocation   = errors.New("randomx: failed to allocate cache")
	ErrDatasetAllocation = errors.New("randomx: failed to allocate dataset")
	ErrVMCreation        = errors.New("randomx: failed to create VM")
	ErrNotInitialized    = errors.New("randomx: context not initialized")
	ErrInvalidKey        = errors.New("randomx: invalid key")
)

// Context holds the RandomX cache and optional dataset.
// It is NOT thread-safe for initialization but multiple VMs can
// be created from it for concurrent hashing.
type Context struct {
	flags   Flag
	cache   *C.randomx_cache
	dataset *C.randomx_dataset
	key     []byte
	mu      sync.RWMutex
}

// NewContext creates a new RandomX context with the specified flags.
// Call InitCache() before using.
func NewContext(flags Flag) (*Context, error) {
	// Combine user flags with recommended flags
	recommended := GetFlags()
	combinedFlags := flags | recommended

	return &Context{
		flags: combinedFlags,
	}, nil
}

// InitCache initializes the RandomX cache with the given key.
// The key is typically a block hash that changes periodically.
//
// For OpenSY, the key changes every 32 blocks.
func (c *Context) InitCache(key []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(key) == 0 {
		return ErrInvalidKey
	}

	// Free existing cache if any
	if c.cache != nil {
		C.randomx_release_cache(c.cache)
		c.cache = nil
	}

	// Allocate new cache
	c.cache = C.randomx_alloc_cache(C.randomx_flags(c.flags))
	if c.cache == nil {
		return ErrCacheAllocation
	}

	// Initialize cache with key
	keyPtr := (*C.char)(unsafe.Pointer(&key[0]))
	C.randomx_init_cache(c.cache, unsafe.Pointer(keyPtr), C.size_t(len(key)))

	// Store key copy
	c.key = make([]byte, len(key))
	copy(c.key, key)

	return nil
}

// InitDataset initializes the full RandomX dataset (~2GB).
// This is required for efficient mining but optional for validation.
//
// numThreads specifies how many threads to use for dataset generation.
// If 0, uses runtime.NumCPU().
func (c *Context) InitDataset(numThreads int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cache == nil {
		return ErrNotInitialized
	}

	if numThreads <= 0 {
		numThreads = runtime.NumCPU()
	}

	// Free existing dataset if any
	if c.dataset != nil {
		C.randomx_release_dataset(c.dataset)
		c.dataset = nil
	}

	// Allocate dataset
	c.dataset = C.randomx_alloc_dataset(C.randomx_flags(c.flags))
	if c.dataset == nil {
		return ErrDatasetAllocation
	}

	// Get dataset item count
	itemCount := C.randomx_dataset_item_count()

	// Initialize dataset in parallel
	itemsPerThread := uint64(itemCount) / uint64(numThreads)

	var wg sync.WaitGroup
	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		startItem := uint64(i) * itemsPerThread
		count := itemsPerThread
		if i == numThreads-1 {
			// Last thread handles remaining items
			count = uint64(itemCount) - startItem
		}

		go func(start, cnt uint64) {
			defer wg.Done()
			C.randomx_init_dataset(
				c.dataset,
				c.cache,
				C.ulong(start),
				C.ulong(cnt),
			)
		}(startItem, count)
	}
	wg.Wait()

	return nil
}

// CreateVM creates a new virtual machine for hashing.
// Each goroutine should have its own VM.
func (c *Context) CreateVM() (*VM, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.cache == nil {
		return nil, ErrNotInitialized
	}

	var vm *C.randomx_vm
	if c.dataset != nil {
		// Full dataset mode (faster, for mining)
		vm = C.randomx_create_vm(C.randomx_flags(c.flags), c.cache, c.dataset)
	} else {
		// Light mode (slower, for validation)
		vm = C.randomx_create_vm(C.randomx_flags(c.flags), c.cache, nil)
	}

	if vm == nil {
		return nil, ErrVMCreation
	}

	return &VM{vm: vm, ctx: c}, nil
}

// CalculateHash computes a RandomX hash using light mode (no dataset).
// This is slower than using a VM with full dataset but uses less memory.
// For validation only; use CreateVM() for mining.
func (c *Context) CalculateHash(input []byte) ([HashSize]byte, error) {
	vm, err := c.CreateVM()
	if err != nil {
		return [HashSize]byte{}, err
	}
	defer vm.Close()

	return vm.CalculateHash(input), nil
}

// GetKey returns a copy of the current key.
func (c *Context) GetKey() []byte {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.key == nil {
		return nil
	}
	result := make([]byte, len(c.key))
	copy(result, c.key)
	return result
}

// HasDataset returns true if the full dataset is initialized.
func (c *Context) HasDataset() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dataset != nil
}

// Close releases all resources held by the context.
func (c *Context) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.dataset != nil {
		C.randomx_release_dataset(c.dataset)
		c.dataset = nil
	}
	if c.cache != nil {
		C.randomx_release_cache(c.cache)
		c.cache = nil
	}
	c.key = nil
}

// VM is a RandomX virtual machine for computing hashes.
// It is NOT thread-safe; each goroutine should have its own VM.
type VM struct {
	vm  *C.randomx_vm
	ctx *Context
}

// CalculateHash computes the RandomX hash of the input.
func (v *VM) CalculateHash(input []byte) [HashSize]byte {
	var hash [HashSize]byte

	if len(input) == 0 {
		// Empty input - use a zero byte
		var zero byte
		C.randomx_calculate_hash(
			v.vm,
			unsafe.Pointer(&zero),
			C.size_t(0),
			unsafe.Pointer(&hash[0]),
		)
	} else {
		C.randomx_calculate_hash(
			v.vm,
			unsafe.Pointer(&input[0]),
			C.size_t(len(input)),
			unsafe.Pointer(&hash[0]),
		)
	}

	return hash
}

// CalculateHashFirst begins a batch hash calculation.
// Use with CalculateHashNext for efficient batch hashing.
func (v *VM) CalculateHashFirst(input []byte) {
	if len(input) == 0 {
		var zero byte
		C.randomx_calculate_hash_first(
			v.vm,
			unsafe.Pointer(&zero),
			C.size_t(0),
		)
	} else {
		C.randomx_calculate_hash_first(
			v.vm,
			unsafe.Pointer(&input[0]),
			C.size_t(len(input)),
		)
	}
}

// CalculateHashNext continues batch hash calculation.
// Returns the hash of the previous input and prepares for the next.
func (v *VM) CalculateHashNext(nextInput []byte) [HashSize]byte {
	var hash [HashSize]byte

	if len(nextInput) == 0 {
		var zero byte
		C.randomx_calculate_hash_next(
			v.vm,
			unsafe.Pointer(&zero),
			C.size_t(0),
			unsafe.Pointer(&hash[0]),
		)
	} else {
		C.randomx_calculate_hash_next(
			v.vm,
			unsafe.Pointer(&nextInput[0]),
			C.size_t(len(nextInput)),
			unsafe.Pointer(&hash[0]),
		)
	}

	return hash
}

// CalculateHashLast finishes batch hash calculation.
// Returns the hash of the last input.
func (v *VM) CalculateHashLast() [HashSize]byte {
	var hash [HashSize]byte

	C.randomx_calculate_hash_last(
		v.vm,
		unsafe.Pointer(&hash[0]),
	)

	return hash
}

// Close releases the VM resources.
func (v *VM) Close() {
	if v.vm != nil {
		C.randomx_destroy_vm(v.vm)
		v.vm = nil
	}
}

// Utility functions for OpenSY

// GetKeyBlockHeight calculates the key block height for a given block height.
// OpenSY uses a 32-block interval (not 2048 like Monero).
func GetKeyBlockHeight(blockHeight int64) int64 {
	if blockHeight < KeyBlockInterval {
		return 0 // Use genesis for early blocks
	}
	return ((blockHeight / KeyBlockInterval) - 1) * KeyBlockInterval
}

// NeedsKeyUpdate returns true if the key needs to be updated when
// moving from oldHeight to newHeight.
func NeedsKeyUpdate(oldHeight, newHeight int64) bool {
	return GetKeyBlockHeight(oldHeight) != GetKeyBlockHeight(newHeight)
}
