# Copyright (c) 2025 The OpenSyria Core developers
# Distributed under the MIT software license, see the accompanying
# file COPYING or https://opensource.org/license/mit/.

# RandomX library integration for ASIC-resistant Proof-of-Work
# RandomX is used by Monero and provides CPU-optimized mining
# Documentation: https://github.com/tevador/RandomX

include(FetchContent)

message(STATUS "Configuring RandomX library...")

FetchContent_Declare(
    randomx
    GIT_REPOSITORY https://github.com/tevador/RandomX.git
    GIT_TAG        v1.2.1
    GIT_SHALLOW    TRUE
)

# Disable building RandomX tests and benchmarks
set(RANDOMX_BUILD_TESTS OFF CACHE BOOL "" FORCE)
set(RANDOMX_BUILD_TOOLS OFF CACHE BOOL "" FORCE)
set(RANDOMX_BUILD_SHARED OFF CACHE BOOL "" FORCE)

# Fetch and build RandomX
FetchContent_MakeAvailable(randomx)

# Verify RandomX target is available
if(NOT TARGET randomx)
    message(FATAL_ERROR "RandomX target not found after FetchContent")
endif()

# Make RandomX headers available as SYSTEM includes to suppress warnings in downstream code
FetchContent_GetProperties(randomx)
# Remove any existing include directories and re-add as SYSTEM
get_target_property(_randomx_includes randomx INTERFACE_INCLUDE_DIRECTORIES)
if(_randomx_includes)
    set_target_properties(randomx PROPERTIES INTERFACE_INCLUDE_DIRECTORIES "")
    target_include_directories(randomx SYSTEM INTERFACE ${_randomx_includes})
endif()
target_include_directories(randomx SYSTEM PUBLIC ${randomx_SOURCE_DIR}/src)

# Suppress documentation warnings from RandomX (upstream issue with @param comments)
if(CMAKE_CXX_COMPILER_ID MATCHES "Clang|AppleClang")
    target_compile_options(randomx PRIVATE -Wno-documentation)
endif()

message(STATUS "RandomX library configured successfully")
