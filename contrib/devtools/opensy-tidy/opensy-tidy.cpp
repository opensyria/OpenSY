// Copyright (c) 2023 Bitcoin Developers
// Distributed under the MIT software license, see the accompanying
// file COPYING or http://www.opensource.org/licenses/mit-license.php.

#include "nontrivial-threadlocal.h"

#include <clang-tidy/ClangTidyModule.h>
#include <clang-tidy/ClangTidyModuleRegistry.h>

class OpenSYModule final : public clang::tidy::ClangTidyModule
{
public:
    void addCheckFactories(clang::tidy::ClangTidyCheckFactories& CheckFactories) override
    {
        CheckFactories.registerCheck<opensy::NonTrivialThreadLocal>("opensy-nontrivial-threadlocal");
    }
};

static clang::tidy::ClangTidyModuleRegistry::Add<OpenSYModule>
    X("opensy-module", "Adds opensy checks.");

volatile int OpenSYModuleAnchorSource = 0;
