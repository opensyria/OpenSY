// Copyright (c) 2009-2010 Satoshi Nakamoto
// Copyright (c) 2009-2023 The OpenSyria Core developers
// Distributed under the MIT software license, see the accompanying
// file COPYING or http://www.opensource.org/licenses/mit-license.php.

#ifndef OPENSYRIA_UTIL_EXCEPTION_H
#define OPENSYRIA_UTIL_EXCEPTION_H

#include <exception>
#include <string_view>

void PrintExceptionContinue(const std::exception* pex, std::string_view thread_name);

#endif // OPENSYRIA_UTIL_EXCEPTION_H
