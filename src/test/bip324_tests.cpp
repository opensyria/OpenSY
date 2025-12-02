// Copyright (c) 2023 The Bitcoin Core developers
// Copyright (c) 2024 The OpenSyria Core developers
// Distributed under the MIT software license, see the accompanying
// file COPYING or http://www.opensource.org/licenses/mit-license.php.

#include <bip324.h>
#include <chainparams.h>
#include <key.h>
#include <pubkey.h>
#include <span.h>
#include <test/util/random.h>
#include <test/util/setup_common.h>
#include <util/strencodings.h>

#include <array>
#include <cstddef>
#include <cstdint>
#include <ranges>
#include <vector>

#include <boost/test/unit_test.hpp>

BOOST_FIXTURE_TEST_SUITE(bip324_tests, BasicTestingSetup)

BOOST_AUTO_TEST_CASE(packet_test_vectors) {
    // BIP324 key derivation uses network magic in the HKDF process. We use mainnet params here
    // as that is what the test vectors are written for.
    SelectParams(ChainType::MAIN);

    // OpenSyria BIP324 test vector
    // Generated using OpenSyria's HKDF salt: "opensyria_v2_shared_secret" + 0x53594c4d
    // Input keys are from Bitcoin's BIP324 test vectors (first vector)
    // Output values (session_id, garbage terminators, ciphertext) are OpenSyria-specific
    
    // Input parameters
    const uint32_t in_idx = 1;
    const auto in_priv_ours = ParseHex("61062ea5071d800bbfd59e2e8b53d47d194b095ae5a4df04936b49772ef0d4d7");
    const auto in_ellswift_ours = ParseHex<std::byte>("ec0adff257bbfe500c188c80b4fdd640f6b45a482bbc15fc7cef5931deff0aa186f6eb9bba7b85dc4dcc28b28722de1e3d9108b985e2967045668f66098e475b");
    const auto in_ellswift_theirs = ParseHex<std::byte>("a4a94dfce69b4a2a0a099313d10f9f7e7d649d60501c9e1d274c300e0d89aafaffffffffffffffffffffffffffffffffffffffffffffffffffffffff8faf88d5");
    const bool in_initiating = true;
    const auto in_contents = ParseHex<std::byte>("8e");
    
    // Expected outputs (OpenSyria-specific)
    const auto expected_send_garbage = ParseHex<std::byte>("1b4f4037feef79e9389c7f6646451d92");
    const auto expected_recv_garbage = ParseHex<std::byte>("85004b90f2543551b40f0b78016b4f91");
    const auto expected_session_id = ParseHex<std::byte>("fea954a1c326a6648a898908bf0c3d7a8c6e489e8937864cc880f58c0b9cf707");
    const auto expected_ciphertext = ParseHex<std::byte>("50d5e4a59c2d517d276a562b209758325121e54c49");

    // Load keys
    CKey key;
    key.Set(in_priv_ours.begin(), in_priv_ours.end(), true);
    EllSwiftPubKey ellswift_ours(in_ellswift_ours);
    EllSwiftPubKey ellswift_theirs(in_ellswift_theirs);

    // Instantiate encryption BIP324 cipher
    BIP324Cipher cipher(key, ellswift_ours);
    BOOST_CHECK(!cipher);
    BOOST_CHECK(cipher.GetOurPubKey() == ellswift_ours);
    cipher.Initialize(ellswift_theirs, in_initiating);
    BOOST_CHECK(cipher);

    // Compare session variables
    BOOST_CHECK(std::ranges::equal(expected_session_id, cipher.GetSessionID()));
    BOOST_CHECK(std::ranges::equal(expected_send_garbage, cipher.GetSendGarbageTerminator()));
    BOOST_CHECK(std::ranges::equal(expected_recv_garbage, cipher.GetReceiveGarbageTerminator()));

    // Seek to the numbered packet (idx=1, so encrypt one dummy first)
    for (uint32_t i = 0; i < in_idx; ++i) {
        std::vector<std::byte> dummy(cipher.EXPANSION);
        cipher.Encrypt({}, {}, true, dummy);
    }

    // Encrypt contents
    std::vector<std::byte> ciphertext(in_contents.size() + cipher.EXPANSION);
    cipher.Encrypt(in_contents, {}, false, ciphertext);

    // Verify ciphertext matches expected
    BOOST_CHECK(ciphertext == expected_ciphertext);
}

BOOST_AUTO_TEST_CASE(bip324_cipher_roundtrip)
{
    // Test that encrypt/decrypt round-trips work correctly
    SelectParams(ChainType::MAIN);
    
    // Generate random keys for initiator
    CKey init_key = GenerateRandomKey();
    EllSwiftPubKey init_ellswift(init_key.EllSwiftCreate(MakeByteSpan(GetRandHash())));
    
    // Generate random keys for responder
    CKey resp_key = GenerateRandomKey();
    EllSwiftPubKey resp_ellswift(resp_key.EllSwiftCreate(MakeByteSpan(GetRandHash())));
    
    // Create ciphers for both sides
    BIP324Cipher init_cipher(init_key, init_ellswift);
    BIP324Cipher resp_cipher(resp_key, resp_ellswift);
    
    // Initialize ciphers (initiator sends to responder's ellswift, responder receives from initiator's)
    init_cipher.Initialize(resp_ellswift, true);  // initiator
    resp_cipher.Initialize(init_ellswift, false); // responder
    
    // Both should derive the same session ID
    BOOST_CHECK(std::ranges::equal(init_cipher.GetSessionID(), resp_cipher.GetSessionID()));
    
    // Test encryption/decryption round-trip
    std::vector<std::byte> plaintext = {std::byte{0x01}, std::byte{0x02}, std::byte{0x03}, std::byte{0x04}};
    std::vector<std::byte> ciphertext(plaintext.size() + init_cipher.EXPANSION);
    
    // Initiator encrypts
    init_cipher.Encrypt(plaintext, {}, false, ciphertext);
    
    // Responder decrypts
    uint32_t len = resp_cipher.DecryptLength(std::span{ciphertext}.first(resp_cipher.LENGTH_LEN));
    // DecryptLength returns content size (includes 1-byte header in BIP324 format)
    BOOST_CHECK_EQUAL(len, plaintext.size());
    
    std::vector<std::byte> decrypted(len);
    bool ignore = false;
    bool success = resp_cipher.Decrypt(std::span{ciphertext}.subspan(resp_cipher.LENGTH_LEN), {}, ignore, decrypted);
    BOOST_CHECK(success);
    BOOST_CHECK(!ignore);
    BOOST_CHECK(std::ranges::equal(plaintext, decrypted));
}

BOOST_AUTO_TEST_SUITE_END()
