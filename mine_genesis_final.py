#!/usr/bin/env python3
"""
OpenSyria Genesis Block Miner - Final Version
Uses 0x1e00ffff for easier mining while staying within overflow limits
"""

import hashlib
import struct
import time

def sha256d(data):
    return hashlib.sha256(hashlib.sha256(data).digest()).digest()

def bits_to_target(bits):
    exponent = bits >> 24
    mantissa = bits & 0x00ffffff
    if exponent <= 3:
        target = mantissa >> (8 * (3 - exponent))
    else:
        target = mantissa << (8 * (exponent - 3))
    return target

def create_coinbase_tx(message, reward_qirsh, pubkey_hex):
    message_bytes = message.encode('utf-8')
    scriptsig = bytes([
        0x04, 0xff, 0xff, 0x00, 0x1d,  # push 4 bytes of nBits
        0x01, 0x04,  # push 1 byte value 4
        len(message_bytes),
    ]) + message_bytes
    
    pubkey_bytes = bytes.fromhex(pubkey_hex)
    scriptpubkey = bytes([len(pubkey_bytes)]) + pubkey_bytes + bytes([0xac])
    
    tx = b''
    tx += struct.pack('<I', 1)
    tx += bytes([1])
    tx += b'\x00' * 32 + struct.pack('<I', 0xffffffff)
    tx += bytes([len(scriptsig)]) + scriptsig
    tx += struct.pack('<I', 0xffffffff)
    tx += bytes([1])
    tx += struct.pack('<Q', reward_qirsh)
    tx += bytes([len(scriptpubkey)]) + scriptpubkey
    tx += struct.pack('<I', 0)
    return tx

def compute_merkle_root(tx_bytes):
    return sha256d(tx_bytes)

def mine_block(version, prev_hash, merkle_root, timestamp, bits, start_nonce=0):
    target = bits_to_target(bits)
    nonce = start_nonce
    start_time = time.time()
    
    while True:
        header = struct.pack('<I', version)
        header += bytes.fromhex(prev_hash)[::-1]
        header += merkle_root
        header += struct.pack('<I', timestamp)
        header += struct.pack('<I', bits)
        header += struct.pack('<I', nonce)
        
        hash_result = sha256d(header)
        hash_int = int.from_bytes(hash_result, 'little')
        
        if hash_int <= target:
            return nonce, hash_result[::-1].hex()
        
        nonce += 1
        if nonce % 1000000 == 0:
            elapsed = time.time() - start_time
            print(f"  {nonce/1000000:.1f}M nonces, {nonce/elapsed/1000:.1f}K/s")
        
        if nonce >= 0xffffffff:
            return None, None

# Config
MESSAGE = "Dec 8 2024 - Syria Liberated from Assad / سوريا حرة"
PUBKEY = "04678afdb0fe5548271967f1a67130b7105cd6a828e03909a67962e0ea1f61deb649f6bc3f4cef38c4f35504e51ec112de5c384df7ba0b8d578a4c702b6bf11d5f"
REWARD = 10000 * 100_000_000
BITS_MAIN = 0x1e00ffff
BITS_REGTEST = 0x207fffff
BASE_TIMESTAMP = 1733616000

print("OpenSyria Genesis Block Miner")
print("=" * 50)
print(f"Bits: 0x{BITS_MAIN:08x}")

coinbase_tx = create_coinbase_tx(MESSAGE, REWARD, PUBKEY)
merkle_root = compute_merkle_root(coinbase_tx)
print(f"Merkle Root: {merkle_root[::-1].hex()}\n")

networks = [
    ("Mainnet", BASE_TIMESTAMP, BITS_MAIN),
    ("Testnet", BASE_TIMESTAMP + 1, BITS_MAIN),
    ("Signet", BASE_TIMESTAMP + 2, 0x1e0377ae),
    ("Regtest", BASE_TIMESTAMP + 3, BITS_REGTEST),
    ("Testnet4", BASE_TIMESTAMP + 4, BITS_MAIN),
]

for name, timestamp, bits in networks:
    print(f"{name} (ts={timestamp}, bits=0x{bits:08x}):")
    nonce, block_hash = mine_block(1, "0"*64, merkle_root, timestamp, bits)
    if nonce:
        print(f"  Nonce: {nonce}, Hash: {block_hash}\n")
