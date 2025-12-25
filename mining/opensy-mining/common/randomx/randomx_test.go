package randomx

import (
	"bytes"
	"encoding/hex"
	"runtime"
	"sync"
	"testing"
)

// Test vectors from RandomX reference implementation
var testVectors = []struct {
	key   string
	input string
	hash  string
}{
	{
		key:   "test key 000",
		input: "This is a test",
		hash:  "639183aae1bf4c9a35884cb46b09cad9175f04efd7684e7262a0ac1c2f0b4e3f",
	},
	{
		key:   "test key 000",
		input: "Lorem ipsum dolor sit amet",
		hash:  "300a0adb47603dedb42228ccb2b211104f4da45af709cd7547cd049e9489c969",
	},
	{
		key:   "test key 000",
		input: "sed do eiusmod tempor incididunt ut labore et dolore magna aliqua",
		hash:  "c36d4ed4191e617309867ed66a443be4075014e2b061bcdaf9ce7b721d2b77a8",
	},
}

func TestGetFlags(t *testing.T) {
	flags := GetFlags()
	t.Logf("Detected CPU flags: 0x%x", flags)

	// Should always have some flags set on modern CPUs
	if flags == 0 {
		t.Log("Warning: No CPU flags detected, using default mode")
	}
}

func TestNewContext(t *testing.T) {
	ctx, err := NewContext(FlagDefault)
	if err != nil {
		t.Fatalf("NewContext failed: %v", err)
	}
	defer ctx.Close()

	if ctx.HasDataset() {
		t.Error("Context should not have dataset before init")
	}
}

func TestInitCache(t *testing.T) {
	ctx, err := NewContext(FlagDefault)
	if err != nil {
		t.Fatalf("NewContext failed: %v", err)
	}
	defer ctx.Close()

	key := []byte("test key 000")
	if err := ctx.InitCache(key); err != nil {
		t.Fatalf("InitCache failed: %v", err)
	}

	gotKey := ctx.GetKey()
	if !bytes.Equal(gotKey, key) {
		t.Errorf("Key mismatch: got %x, want %x", gotKey, key)
	}
}

func TestInitCacheEmptyKey(t *testing.T) {
	ctx, err := NewContext(FlagDefault)
	if err != nil {
		t.Fatalf("NewContext failed: %v", err)
	}
	defer ctx.Close()

	err = ctx.InitCache([]byte{})
	if err != ErrInvalidKey {
		t.Errorf("Expected ErrInvalidKey, got %v", err)
	}
}

func TestCalculateHash(t *testing.T) {
	ctx, err := NewContext(FlagDefault)
	if err != nil {
		t.Fatalf("NewContext failed: %v", err)
	}
	defer ctx.Close()

	for _, tv := range testVectors {
		if err := ctx.InitCache([]byte(tv.key)); err != nil {
			t.Fatalf("InitCache failed: %v", err)
		}

		hash, err := ctx.CalculateHash([]byte(tv.input))
		if err != nil {
			t.Fatalf("CalculateHash failed: %v", err)
		}

		gotHex := hex.EncodeToString(hash[:])
		if gotHex != tv.hash {
			t.Errorf("Hash mismatch for input %q:\n  got:  %s\n  want: %s",
				tv.input, gotHex, tv.hash)
		}
	}
}

func TestCreateVM(t *testing.T) {
	ctx, err := NewContext(FlagDefault)
	if err != nil {
		t.Fatalf("NewContext failed: %v", err)
	}
	defer ctx.Close()

	// Should fail before InitCache
	_, err = ctx.CreateVM()
	if err != ErrNotInitialized {
		t.Errorf("Expected ErrNotInitialized, got %v", err)
	}

	// Should succeed after InitCache
	if err := ctx.InitCache([]byte("test key 000")); err != nil {
		t.Fatalf("InitCache failed: %v", err)
	}

	vm, err := ctx.CreateVM()
	if err != nil {
		t.Fatalf("CreateVM failed: %v", err)
	}
	defer vm.Close()
}

func TestVMCalculateHash(t *testing.T) {
	ctx, err := NewContext(FlagDefault)
	if err != nil {
		t.Fatalf("NewContext failed: %v", err)
	}
	defer ctx.Close()

	if err := ctx.InitCache([]byte("test key 000")); err != nil {
		t.Fatalf("InitCache failed: %v", err)
	}

	vm, err := ctx.CreateVM()
	if err != nil {
		t.Fatalf("CreateVM failed: %v", err)
	}
	defer vm.Close()

	for _, tv := range testVectors {
		hash := vm.CalculateHash([]byte(tv.input))
		gotHex := hex.EncodeToString(hash[:])
		if gotHex != tv.hash {
			t.Errorf("Hash mismatch for input %q:\n  got:  %s\n  want: %s",
				tv.input, gotHex, tv.hash)
		}
	}
}

func TestBatchHashing(t *testing.T) {
	ctx, err := NewContext(FlagDefault)
	if err != nil {
		t.Fatalf("NewContext failed: %v", err)
	}
	defer ctx.Close()

	if err := ctx.InitCache([]byte("test key 000")); err != nil {
		t.Fatalf("InitCache failed: %v", err)
	}

	vm, err := ctx.CreateVM()
	if err != nil {
		t.Fatalf("CreateVM failed: %v", err)
	}
	defer vm.Close()

	// Test batch hashing
	inputs := [][]byte{
		[]byte("input1"),
		[]byte("input2"),
		[]byte("input3"),
	}

	// Calculate expected hashes individually
	expected := make([][HashSize]byte, len(inputs))
	for i, input := range inputs {
		expected[i] = vm.CalculateHash(input)
	}

	// Calculate using batch API
	vm.CalculateHashFirst(inputs[0])
	hash1 := vm.CalculateHashNext(inputs[1])
	hash2 := vm.CalculateHashNext(inputs[2])
	hash3 := vm.CalculateHashLast()

	// Note: batch hashing returns hash of PREVIOUS input
	if hash1 != expected[0] {
		t.Errorf("Batch hash 1 mismatch")
	}
	if hash2 != expected[1] {
		t.Errorf("Batch hash 2 mismatch")
	}
	if hash3 != expected[2] {
		t.Errorf("Batch hash 3 mismatch")
	}
}

func TestConcurrentVMs(t *testing.T) {
	ctx, err := NewContext(FlagDefault)
	if err != nil {
		t.Fatalf("NewContext failed: %v", err)
	}
	defer ctx.Close()

	if err := ctx.InitCache([]byte("test key 000")); err != nil {
		t.Fatalf("InitCache failed: %v", err)
	}

	numGoroutines := runtime.NumCPU()
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			vm, err := ctx.CreateVM()
			if err != nil {
				t.Errorf("Goroutine %d: CreateVM failed: %v", id, err)
				return
			}
			defer vm.Close()

			// Each goroutine calculates some hashes
			for j := 0; j < 10; j++ {
				input := []byte("This is a test")
				hash := vm.CalculateHash(input)

				expected := "639183aae1bf4c9a35884cb46b09cad9175f04efd7684e7262a0ac1c2f0b4e3f"
				gotHex := hex.EncodeToString(hash[:])
				if gotHex != expected {
					t.Errorf("Goroutine %d, iteration %d: hash mismatch", id, j)
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestGetKeyBlockHeight(t *testing.T) {
	tests := []struct {
		height   int64
		expected int64
	}{
		{0, 0},
		{1, 0},
		{31, 0},
		{32, 0}, // First full interval, key is from height 0
		{33, 0},
		{63, 0},
		{64, 32}, // Second interval, key is from height 32
		{65, 32},
		{95, 32},
		{96, 64}, // Third interval, key is from height 64
		{100, 64},
		{1000, 960}, // 1000/32 = 31.25 -> 31, (31-1)*32 = 960
	}

	for _, tt := range tests {
		got := GetKeyBlockHeight(tt.height)
		if got != tt.expected {
			t.Errorf("GetKeyBlockHeight(%d) = %d, want %d", tt.height, got, tt.expected)
		}
	}
}

func TestNeedsKeyUpdate(t *testing.T) {
	tests := []struct {
		oldHeight int64
		newHeight int64
		expected  bool
	}{
		{0, 1, false},
		{31, 32, false}, // Both use key from 0
		{32, 64, true},  // 32 uses 0, 64 uses 32
		{63, 64, true},  // 63 uses 0, 64 uses 32
		{64, 65, false}, // Both use key from 32
		{95, 96, true},  // 95 uses 32, 96 uses 64
	}

	for _, tt := range tests {
		got := NeedsKeyUpdate(tt.oldHeight, tt.newHeight)
		if got != tt.expected {
			t.Errorf("NeedsKeyUpdate(%d, %d) = %v, want %v",
				tt.oldHeight, tt.newHeight, got, tt.expected)
		}
	}
}

// Benchmarks

func BenchmarkCalculateHash(b *testing.B) {
	ctx, err := NewContext(FlagDefault)
	if err != nil {
		b.Fatalf("NewContext failed: %v", err)
	}
	defer ctx.Close()

	if err := ctx.InitCache([]byte("benchmark key")); err != nil {
		b.Fatalf("InitCache failed: %v", err)
	}

	vm, err := ctx.CreateVM()
	if err != nil {
		b.Fatalf("CreateVM failed: %v", err)
	}
	defer vm.Close()

	input := []byte("benchmark input data for randomx hashing")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vm.CalculateHash(input)
	}
}

func BenchmarkCalculateHashParallel(b *testing.B) {
	ctx, err := NewContext(FlagDefault)
	if err != nil {
		b.Fatalf("NewContext failed: %v", err)
	}
	defer ctx.Close()

	if err := ctx.InitCache([]byte("benchmark key")); err != nil {
		b.Fatalf("InitCache failed: %v", err)
	}

	input := []byte("benchmark input data for randomx hashing")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		vm, err := ctx.CreateVM()
		if err != nil {
			b.Fatalf("CreateVM failed: %v", err)
		}
		defer vm.Close()

		for pb.Next() {
			vm.CalculateHash(input)
		}
	})
}

func BenchmarkInitCache(b *testing.B) {
	key := []byte("benchmark key for cache init")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, _ := NewContext(FlagDefault)
		ctx.InitCache(key)
		ctx.Close()
	}
}
