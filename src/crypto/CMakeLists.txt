# Copyright (c) 2023-present The Bitcoin Core developers
# Distributed under the MIT software license, see the accompanying
# file COPYING or https://opensource.org/license/mit/.

add_library(opensy_crypto STATIC EXCLUDE_FROM_ALL
  aes.cpp
  argon2_context.cpp
  blake2b.cpp
  chacha20.cpp
  chacha20poly1305.cpp
  hex_base.cpp
  hkdf_sha256_32.cpp
  hmac_sha256.cpp
  hmac_sha512.cpp
  muhash.cpp
  poly1305.cpp
  randomx_context.cpp
  randomx_pool.cpp
  ripemd160.cpp
  sha1.cpp
  sha256.cpp
  sha256_sse4.cpp
  sha3.cpp
  sha512.cpp
  siphash.cpp
  ../support/cleanse.cpp
)

target_link_libraries(opensy_crypto
  PRIVATE
    core_interface
  PUBLIC
    randomx
)

# Optional: Link libsodium for optimized Argon2id (recommended for production)
# If libsodium is found, use it; otherwise fall back to reference implementation
find_package(PkgConfig QUIET)
if(PkgConfig_FOUND)
  pkg_check_modules(LIBSODIUM QUIET libsodium)
  if(LIBSODIUM_FOUND)
    target_compile_definitions(opensy_crypto PRIVATE HAVE_LIBSODIUM)
    target_include_directories(opensy_crypto PRIVATE ${LIBSODIUM_INCLUDE_DIRS})
    target_link_directories(opensy_crypto PUBLIC ${LIBSODIUM_LIBRARY_DIRS})
    target_link_libraries(opensy_crypto PUBLIC ${LIBSODIUM_LINK_LIBRARIES})
    message(STATUS "Using libsodium for Argon2id PoW fallback")
  else()
    message(STATUS "libsodium not found - Argon2id fallback will use reference implementation")
  endif()
endif()

if(HAVE_SSE41)
  target_compile_definitions(opensy_crypto PRIVATE ENABLE_SSE41)
  target_sources(opensy_crypto PRIVATE sha256_sse41.cpp)
  set_property(SOURCE sha256_sse41.cpp PROPERTY
    COMPILE_OPTIONS ${SSE41_CXXFLAGS}
  )
endif()

if(HAVE_AVX2)
  target_compile_definitions(opensy_crypto PRIVATE ENABLE_AVX2)
  target_sources(opensy_crypto PRIVATE sha256_avx2.cpp)
  set_property(SOURCE sha256_avx2.cpp PROPERTY
    COMPILE_OPTIONS ${AVX2_CXXFLAGS}
  )
endif()

if(HAVE_SSE41 AND HAVE_X86_SHANI)
  target_compile_definitions(opensy_crypto PRIVATE ENABLE_SSE41 ENABLE_X86_SHANI)
  target_sources(opensy_crypto PRIVATE sha256_x86_shani.cpp)
  set_property(SOURCE sha256_x86_shani.cpp PROPERTY
    COMPILE_OPTIONS ${SSE41_CXXFLAGS} ${X86_SHANI_CXXFLAGS}
  )
endif()

if(HAVE_ARM_SHANI)
  target_compile_definitions(opensy_crypto PRIVATE ENABLE_ARM_SHANI)
  target_sources(opensy_crypto PRIVATE sha256_arm_shani.cpp)
  set_property(SOURCE sha256_arm_shani.cpp PROPERTY
    COMPILE_OPTIONS ${ARM_SHANI_CXXFLAGS}
  )
endif()
