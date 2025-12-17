#!/usr/bin/env python3
"""
Simple Genesis Block Miner for OpenSY
Mines SHA256d genesis block with timestamp 1733631480 (Dec 8, 2024 06:18:00 Syria)
"""
import hashlib
import struct
import time

def sha256d(data):
    """Double SHA256 hash"""
    return hashlib.sha256(hashlib.sha256(data).digest()).digest()

def int_to_little_endian(n, length):
    """Convert integer to little-endian bytes"""
    return n.to_bytes(length, byteorder='little')

def compact_to_uint256(compact):
    """Convert compact target format to uint256 bytes"""
    size = compact >> 24
    word = compact & 0x007fffff
    if size <= 3:
        word >>= 8 * (3 - size)
        result = word.to_bytes(32, byteorder='little')
    else:
        result = (word << (8 * (size - 3))).to_bytes(32, byteorder='little')
    return result

def mine_genesis(timestamp, bits, version=1, start_nonce=0, max_nonce=2**32):
    """Mine genesis block"""
    # Block header structure: version(4) + prev_block(32) + merkle_root(32) + time(4) + bits(4) + nonce(4) = 80 bytes
    # We need to compute merkle_root from the coinbase tx first
    
    # Coinbase transaction for OpenSY genesis
    pszTimestamp = b"Dec 8 2024 - Syria Liberated from Assad / \xd8\xb3\xd9\x88\xd8\xb1\xd9\x8a\xd8\xa7 \xd8\xad\xd8\xb1\xd8\xa9"
    
    # Build coinbase tx
    tx_version = struct.pack('<I', 1)  # version
    tx_in_count = bytes([1])
    # Previous output (null for coinbase)
    prev_out = bytes(32) + struct.pack('<I', 0xffffffff)
    # Script sig: <bits> <4> <timestamp>
    script_sig = bytes([4, 0xff, 0xff, 0x00, 0x1d, 1, 4]) + bytes([len(pszTimestamp)]) + pszTimestamp
    script_sig_len = bytes([len(script_sig)])
    sequence = struct.pack('<I', 0xffffffff)
    tx_out_count = bytes([1])
    # Value: 10000 * 100000000 satoshis = 1000000000000000
    value = struct.pack('<Q', 10000 * 100000000)
    # Output script: OP_PUSH(65) <pubkey> OP_CHECKSIG
    pubkey = bytes.fromhex("04678afdb0fe5548271967f1a67130b7105cd6a828e03909a67962e0ea1f61deb649f6bc3f4cef38c4f35504e51ec112de5c384df7ba0b8d578a4c702b6bf11d5f")
    script_pubkey = bytes([65]) + pubkey + bytes([0xac])  # OP_CHECKSIG = 0xac
    script_pubkey_len = bytes([len(script_pubkey)])
    locktime = struct.pack('<I', 0)
    
    coinbase_tx = tx_version + tx_in_count + prev_out + script_sig_len + script_sig + sequence + tx_out_count + value + script_pubkey_len + script_pubkey + locktime
    
    # Merkle root = hash of single tx (genesis has only coinbase)
    merkle_root = sha256d(coinbase_tx)
    
    print(f"Coinbase tx hash (merkle root): {merkle_root[::-1].hex()}")
    
    # Previous block hash (all zeros for genesis)
    prev_block = bytes(32)
    
    # Target from bits
    target = compact_to_uint256(bits)
    target_int = int.from_bytes(target, byteorder='little')
    
    print(f"Target: {target[::-1].hex()}")
    print(f"Mining genesis block with timestamp {timestamp}...")
    print(f"Starting from nonce {start_nonce}")
    
    # Build header template (without nonce)
    header_template = (
        struct.pack('<I', version) +      # version
        prev_block +                       # prev_block
        merkle_root +                      # merkle_root
        struct.pack('<I', timestamp) +    # time
        struct.pack('<I', bits)           # bits
    )
    
    start_time = time.time()
    nonce = start_nonce
    checked = 0
    
    while nonce < max_nonce:
        header = header_template + struct.pack('<I', nonce)
        hash_result = sha256d(header)
        hash_int = int.from_bytes(hash_result, byteorder='little')
        
        if hash_int <= target_int:
            elapsed = time.time() - start_time
            print(f"\n{'='*60}")
            print(f"FOUND VALID NONCE!")
            print(f"{'='*60}")
            print(f"Nonce: {nonce}")
            print(f"Genesis hash: {hash_result[::-1].hex()}")
            print(f"Merkle root: {merkle_root[::-1].hex()}")
            print(f"Time elapsed: {elapsed:.2f} seconds")
            print(f"Hash rate: {checked/elapsed:.2f} H/s")
            return nonce, hash_result[::-1].hex(), merkle_root[::-1].hex()
        
        nonce += 1
        checked += 1
        
        if checked % 1000000 == 0:
            elapsed = time.time() - start_time
            print(f"Checked {checked/1e6:.1f}M nonces ({checked/elapsed:.0f} H/s) - current: {nonce}")
    
    print(f"No valid nonce found in range")
    return None, None, None

if __name__ == "__main__":
    # OpenSY mainnet genesis parameters
    TIMESTAMP = 1733631480  # Dec 8, 2024 06:18:00 Syria (04:18 UTC)
    BITS = 0x1e00ffff       # Difficulty
    VERSION = 1
    
    nonce, hash_result, merkle = mine_genesis(TIMESTAMP, BITS, VERSION)
    
    if nonce:
        print(f"\n{'='*60}")
        print("UPDATE chainparams.cpp with:")
        print(f"{'='*60}")
        print(f'genesis = CreateGenesisBlock({TIMESTAMP}, {nonce}, 0x1e00ffff, 1, 10000 * COIN);')
        print(f'assert(consensus.hashGenesisBlock == uint256{{"{hash_result}"}});')
        print(f'assert(genesis.hashMerkleRoot == uint256{{"{merkle}"}});')
