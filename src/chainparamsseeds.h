#ifndef OPENSY_CHAINPARAMSSEEDS_H
#define OPENSY_CHAINPARAMSSEEDS_H
/**
 * List of fixed seed nodes for the OpenSY network
 * Generated: December 8, 2025 - Network Launch Day! 🇸🇾
 *
 * IMPORTANT: Only include IPs of nodes that are actually running!
 * Non-existent IPs cause connection timeouts and waste resources.
 *
 * Each entry is BIP155 serialized: networkID (1 byte) + COMPACTSIZE(addr_len) + addr + port (2 bytes BE)
 * For IPv4: 0x01 + 0x04 + 4 bytes IP + 2 bytes port
 * For IPv6: 0x02 + 0x10 + 16 bytes IP + 2 bytes port
 *
 * To add new seeds:
 * 1. Ensure node has 99%+ uptime and port 9633 open
 * 2. Convert IP to hex bytes (e.g., 157.175.40.131 -> 0x9d, 0xaf, 0x28, 0x83)
 * 3. Port 9633 in big-endian is 0x25, 0xa1
 * 4. Update chainparams_seed_main_size count
 */

// Mainnet seeds - Only include actually running nodes!
static const uint8_t chainparams_seed_main[] = {
    // node1.opensy.net (AWS Bahrain me-south-1) - 157.175.40.131:9633
    0x01,                         // BIP155 network ID for IPv4
    0x04,                         // COMPACTSIZE: address length = 4 bytes
    0x9d, 0xaf, 0x28, 0x83,       // 157.175.40.131
    0x25, 0xa1                    // Port 9633 (big-endian: 0x25a1)
    
    // TODO: Add more seeds when deployed. Example format:
    // , // comma to separate from previous entry
    // 0x01,                       // BIP155 network ID for IPv4
    // 0x04,                       // COMPACTSIZE: address length = 4 bytes
    // 0xXX, 0xXX, 0xXX, 0xXX,     // IP address bytes
    // 0x25, 0xa1                  // Port 9633
};
constexpr size_t chainparams_seed_main_size = 1;  // Update when adding more seeds

// Signet seeds - empty until OpenSY signet nodes are established
static const uint8_t chainparams_seed_signet[] = {0x00};
constexpr size_t chainparams_seed_signet_size = 0;

// Testnet seeds - empty until OpenSY testnet nodes are established
static const uint8_t chainparams_seed_test[] = {0x00};
constexpr size_t chainparams_seed_test_size = 0;

// Testnet4 seeds - empty until OpenSY testnet4 nodes are established
static const uint8_t chainparams_seed_testnet4[] = {0x00};
constexpr size_t chainparams_seed_testnet4_size = 0;

#endif // OPENSY_CHAINPARAMSSEEDS_H
