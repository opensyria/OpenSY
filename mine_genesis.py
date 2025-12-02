#!/usr/bin/env python3
"""
OpenSyria Genesis Block Miner
Commemorating Syria's Liberation - December 8, 2024
"""

import hashlib
import struct
import time

def sha256d(data):
    """Double SHA256 hash"""
    return hashlib.sha256(hashlib.sha256(data).digest()).digest()

def create_coinbase_script(message):
    """Create the coinbase scriptSig with the embedded message"""
    # Bitcoin-style: height push + message
    msg_bytes = message.encode('utf-8')
    # 486604799 = 0x1d00ffff in little endian push, then 4, then message
    script = bytes([0x04, 0xff, 0xff, 0x00, 0x1d])  # push 486604799
    script += bytes([0x01, 0x04])  # push 4
    script += bytes([len(msg_bytes)]) + msg_bytes
    return script

def create_coinbase_tx(message, reward_qirsh):
    """Create the coinbase transaction"""
    # Genesis pubkey (same as Bitcoin's - can be changed)
    pubkey = bytes.fromhex("04678afdb0fe5548271967f1a67130b7105cd6a828e03909a67962e0ea1f61deb649f6bc3f4cef38c4f35504e51ec112de5c384df7ba0b8d578a4c702b6bf11d5f")
    
    # Build scriptPubKey: <pubkey> OP_CHECKSIG
    script_pubkey = bytes([len(pubkey)]) + pubkey + bytes([0xac])  # OP_CHECKSIG
    
    # Coinbase scriptSig
    script_sig = create_coinbase_script(message)
    
    # Build transaction
    tx = b''
    tx += struct.pack('<I', 1)  # version
    tx += bytes([1])  # input count
    # Input (coinbase)
    tx += bytes(32)  # prev tx hash (null)
    tx += struct.pack('<I', 0xffffffff)  # prev output index
    tx += bytes([len(script_sig)]) + script_sig  # scriptSig
    tx += struct.pack('<I', 0xffffffff)  # sequence
    # Output
    tx += bytes([1])  # output count
    tx += struct.pack('<Q', reward_qirsh)  # value in qirsh
    tx += bytes([len(script_pubkey)]) + script_pubkey
    tx += struct.pack('<I', 0)  # locktime
    
    return tx

def calc_merkle_root(tx):
    """Calculate merkle root for single transaction"""
    return sha256d(tx)

def create_block_header(version, prev_hash, merkle_root, timestamp, bits, nonce):
    """Create block header"""
    header = b''
    header += struct.pack('<I', version)
    header += prev_hash  # 32 bytes, all zeros for genesis
    header += merkle_root  # 32 bytes
    header += struct.pack('<I', timestamp)
    header += struct.pack('<I', bits)
    header += struct.pack('<I', nonce)
    return header

def check_pow(header_hash, bits):
    """Check if hash meets difficulty target"""
    # Extract target from compact bits
    exp = bits >> 24
    mant = bits & 0x7fffff
    target = mant * (256 ** (exp - 3))
    
    # Convert hash to integer (little endian)
    hash_int = int.from_bytes(header_hash, 'little')
    
    return hash_int < target

def mine_genesis(message, timestamp, bits, reward_syl):
    """Mine the genesis block"""
    print("=" * 70)
    print("OpenSyria Genesis Block Miner")
    print("=" * 70)
    print(f"Message: {message}")
    print(f"Timestamp: {timestamp} (Dec 8, 2024 00:00:00 UTC)")
    print(f"Bits: 0x{bits:08x}")
    print(f"Reward: {reward_syl} SYL ({reward_syl * 100000000} QIRSH)")
    print("=" * 70)
    
    # Create coinbase transaction
    reward_qirsh = reward_syl * 100000000  # Convert SYL to QIRSH
    coinbase_tx = create_coinbase_tx(message, reward_qirsh)
    
    # Calculate merkle root
    merkle_root = calc_merkle_root(coinbase_tx)
    print(f"Merkle Root: {merkle_root[::-1].hex()}")
    
    # Previous block hash (all zeros for genesis)
    prev_hash = bytes(32)
    
    # Version
    version = 1
    
    print("\nMining genesis block...")
    print("This may take a while depending on difficulty...\n")
    
    nonce = 0
    start_time = time.time()
    last_update = start_time
    
    while True:
        header = create_block_header(version, prev_hash, merkle_root, timestamp, bits, nonce)
        header_hash = sha256d(header)
        
        if check_pow(header_hash, bits):
            elapsed = time.time() - start_time
            print("\n" + "=" * 70)
            print("GENESIS BLOCK FOUND!")
            print("=" * 70)
            print(f"Nonce: {nonce}")
            print(f"Hash: {header_hash[::-1].hex()}")
            print(f"Merkle Root: {merkle_root[::-1].hex()}")
            print(f"Time elapsed: {elapsed:.2f} seconds")
            print("=" * 70)
            
            print("\n// Copy these values to chainparams.cpp:")
            print(f'// Genesis block for OpenSyria mainnet')
            print(f'// Message: "{message}"')
            print(f'genesis = CreateGenesisBlock({timestamp}, {nonce}, 0x{bits:08x}, 1, {reward_syl} * COIN);')
            print(f'assert(consensus.hashGenesisBlock == uint256{{"{header_hash[::-1].hex()}"}});')
            print(f'assert(genesis.hashMerkleRoot == uint256{{"{merkle_root[::-1].hex()}"}});')
            
            return nonce, header_hash[::-1].hex(), merkle_root[::-1].hex()
        
        nonce += 1
        
        # Progress update every 5 seconds
        if time.time() - last_update > 5:
            elapsed = time.time() - start_time
            rate = nonce / elapsed
            print(f"Tried {nonce:,} nonces... ({rate:,.0f} H/s)")
            last_update = time.time()
        
        # Safety limit
        if nonce >= 0xffffffff:
            print("Nonce space exhausted! Try different timestamp.")
            return None, None, None

if __name__ == "__main__":
    # OpenSyria Genesis Configuration
    # =================================
    
    # December 8, 2024 00:00:00 UTC - Syria Liberation Day
    # Unix timestamp: 1733616000
    TIMESTAMP = 1733616000
    
    # Genesis message commemorating Syria's liberation
    MESSAGE = "Dec 8 2024 - Syria Liberated from Assad / سوريا حرة"
    
    # Difficulty - using easier target for faster mining
    # 0x1d00ffff = Bitcoin mainnet difficulty (very hard)
    # 0x1e0ffff0 = Easier (good for testing)
    # 0x1f00ffff = Very easy (for quick testing)
    BITS = 0x1e0ffff0  # Moderate difficulty
    
    # Block reward: 10,000 SYL
    REWARD_SYL = 10000
    
    # Mine!
    mine_genesis(MESSAGE, TIMESTAMP, BITS, REWARD_SYL)
