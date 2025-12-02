#!/usr/bin/env python3
"""
OpenSyria Genesis Block Miner
Mines genesis blocks with correct difficulty (0x1d00ffff) for Bitcoin-compatible chains
"""

import hashlib
import struct
import time

def sha256d(data):
    return hashlib.sha256(hashlib.sha256(data).digest()).digest()

def bits_to_target(bits):
    """Convert compact bits to full target"""
    exponent = bits >> 24
    mantissa = bits & 0x00ffffff
    if exponent <= 3:
        target = mantissa >> (8 * (3 - exponent))
    else:
        target = mantissa << (8 * (exponent - 3))
    return target

def create_coinbase_tx(message, reward_qirsh, pubkey_hex):
    """Create a coinbase transaction"""
    # scriptSig: OP_PUSH(4) nBits OP_PUSH(1) nHeight=0 OP_PUSH(len) message
    # For genesis, we use: 0x04 ffff001d 0x01 0x04 <len> <message>
    message_bytes = message.encode('utf-8')
    scriptsig = bytes([
        0x04,  # push 4 bytes
        0xff, 0xff, 0x00, 0x1d,  # nBits in LE (0x1d00ffff)
        0x01,  # push 1 byte
        0x04,  # the value 4
        len(message_bytes),  # push message length
    ]) + message_bytes
    
    # scriptPubKey: <pubkey> OP_CHECKSIG
    pubkey_bytes = bytes.fromhex(pubkey_hex)
    scriptpubkey = bytes([len(pubkey_bytes)]) + pubkey_bytes + bytes([0xac])  # OP_CHECKSIG
    
    # Build transaction
    tx = b''
    tx += struct.pack('<I', 1)  # version
    tx += bytes([1])  # number of inputs
    # Input
    tx += b'\x00' * 32  # prev txid (null)
    tx += struct.pack('<I', 0xffffffff)  # prev index
    tx += bytes([len(scriptsig)]) + scriptsig  # scriptSig
    tx += struct.pack('<I', 0xffffffff)  # sequence
    # Outputs
    tx += bytes([1])  # number of outputs
    tx += struct.pack('<Q', reward_qirsh)  # value in qirsh
    tx += bytes([len(scriptpubkey)]) + scriptpubkey  # scriptPubKey
    tx += struct.pack('<I', 0)  # locktime
    
    return tx

def compute_merkle_root(tx_bytes):
    """Compute merkle root for a single transaction"""
    return sha256d(tx_bytes)

def mine_block(version, prev_hash, merkle_root, timestamp, bits, start_nonce=0):
    """Mine a block header"""
    target = bits_to_target(bits)
    print(f"  Target: {target:064x}")
    
    nonce = start_nonce
    start_time = time.time()
    last_report = start_time
    
    while True:
        # Build header
        header = struct.pack('<I', version)
        header += bytes.fromhex(prev_hash)[::-1]  # prev hash in LE
        header += merkle_root  # already in LE
        header += struct.pack('<I', timestamp)
        header += struct.pack('<I', bits)
        header += struct.pack('<I', nonce)
        
        # Hash
        hash_result = sha256d(header)
        hash_int = int.from_bytes(hash_result, 'little')
        
        if hash_int <= target:
            hash_hex = hash_result[::-1].hex()
            return nonce, hash_hex
        
        nonce += 1
        if nonce % 1000000 == 0:
            elapsed = time.time() - start_time
            rate = nonce / elapsed
            print(f"  Tried {nonce/1000000:.1f}M nonces, {rate/1000:.1f}K/s")
        
        if nonce >= 0xffffffff:
            return None, None

# Configuration
MESSAGE = "Dec 8 2024 - Syria Liberated from Assad / سوريا حرة"
PUBKEY = "04678afdb0fe5548271967f1a67130b7105cd6a828e03909a67962e0ea1f61deb649f6bc3f4cef38c4f35504e51ec112de5c384df7ba0b8d578a4c702b6bf11d5f"
REWARD = 10000 * 100_000_000  # 10000 SYL in qirsh
BITS = 0x1d00ffff  # Standard Bitcoin difficulty - requires ~32 leading zero bits
PREV_HASH = "0000000000000000000000000000000000000000000000000000000000000000"

# Syria Liberation timestamps
BASE_TIMESTAMP = 1733616000  # Dec 8, 2024 00:00:00 UTC

print("OpenSyria Genesis Block Miner")
print("=" * 50)
print(f"Message: {MESSAGE}")
print(f"Bits: 0x{BITS:08x}")
print(f"Target requires ~32 leading zero bits")
print()

# Create coinbase transaction
coinbase_tx = create_coinbase_tx(MESSAGE, REWARD, PUBKEY)
merkle_root = compute_merkle_root(coinbase_tx)

print(f"Coinbase TX: {coinbase_tx.hex()}")
print(f"Merkle Root: {merkle_root[::-1].hex()}")
print()

networks = [
    ("Mainnet", BASE_TIMESTAMP),
    ("Testnet", BASE_TIMESTAMP + 1),
    ("Signet", BASE_TIMESTAMP + 2),
    ("Regtest", BASE_TIMESTAMP + 3),
    ("Testnet4", BASE_TIMESTAMP + 4),
]

print("WARNING: Mining with 0x1d00ffff will take MUCH longer than 0x1e0ffff0")
print("Expected time: Could be hours or days per block")
print()

for name, timestamp in networks:
    print(f"Mining {name} genesis (timestamp={timestamp})...")
    
    # For regtest, use easier difficulty
    if name == "Regtest":
        bits = 0x207fffff
        print(f"  Using regtest difficulty: 0x{bits:08x}")
    else:
        bits = BITS
    
    nonce, block_hash = mine_block(1, PREV_HASH, merkle_root, timestamp, bits)
    
    if nonce is not None:
        print(f"  SUCCESS!")
        print(f"  Nonce: {nonce}")
        print(f"  Hash: {block_hash}")
    else:
        print(f"  FAILED - nonce overflow")
    print()
