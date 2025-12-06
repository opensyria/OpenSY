OpenSyria Core integration/staging tree
=====================================

https://opensyria.net

For an immediately usable, binary version of the OpenSyria Core software, see
https://opensyria.net/en/download/.

What is OpenSyria?
------------------

OpenSyria (SYL) is a cryptocurrency forked from Bitcoin Core, created to support
the Syrian community with a modern, decentralized digital currency. It maintains
Bitcoin's proven security model while introducing Syria-specific customizations.

**Key Features:**
- **Address Prefix:** Mainnet addresses start with 'F' (Freedom)
- **Network Ports:** 9633 (mainnet), based on Syria's country code 963
- **Block Reward:** 10,000 SYL initial reward
- **Genesis Date:** December 8, 2024 (Syria Liberation Day)
- **Currency Unit:** SYL (Syrian Lira digital)

What is OpenSyria Core?
-----------------------

OpenSyria Core connects to the OpenSyria peer-to-peer network to download and fully
validate blocks and transactions. It also includes a wallet and graphical user
interface, which can be optionally built.

Further information about OpenSyria Core is available in the [doc folder](/doc).

Network Parameters
------------------

| Parameter | Mainnet | Testnet | Regtest |
|-----------|---------|---------|---------|  
| Address Prefix | F | f | f |
| Default Port | 9633 | 19633 | 19634 |
| Bech32 Prefix | syl | tsyl | rsyl |
| Block Time | ~2 minutes | ~2 minutes | instant |

License
-------

OpenSyria Core is released under the terms of the MIT license. See [COPYING](COPYING) for more
information or see https://opensource.org/license/MIT.

Development Process
-------------------

The `main` branch is regularly built (see `doc/build-*.md` for instructions) and tested, but it is not guaranteed to be
completely stable. [Tags](https://github.com/opensyria/OpenSyria/tags) are created
regularly from release branches to indicate new official, stable release versions of OpenSyria Core.

<!-- TODO [INFRASTRUCTURE - HIGH PRIORITY]:
1. Block Explorer: Deploy a block explorer (recommend mempool/mempool or btc-rpc-explorer)
   - Users need to view transactions, blocks, and addresses
   - Host at explore.opensyria.net or similar

2. Network Health Dashboard: Set up monitoring for:
   - Node count and geographic distribution
   - Network hashrate
   - Block times (target: 2 minutes)
   - Mempool statistics
   - Use Grafana + Prometheus or similar

3. Public Seed Nodes: Deploy geographically distributed seed nodes
   - Update src/kernel/chainparams.cpp with DNS seeds
-->

The https://github.com/opensyria/opensyria-gui repository is used exclusively for the
development of the GUI. Its master branch is identical in all monotree
repositories. Release branches and tags do not exist, so please do not fork
that repository unless it is for development reasons.

The contribution workflow is described in [CONTRIBUTING.md](CONTRIBUTING.md)
and useful hints for developers can be found in [doc/developer-notes.md](doc/developer-notes.md).

Testing
-------

Testing and code review is the bottleneck for development; we get more pull
requests than we can review and test on short notice. Please be patient and help out by testing
other people's pull requests, and remember this is a security-critical project where any mistake might cost people
lots of money.

### Automated Testing

Developers are strongly encouraged to write [unit tests](src/test/README.md) for new code, and to
submit new unit tests for old code. Unit tests can be compiled and run
(assuming they weren't disabled during the generation of the build system) with: `ctest`. Further details on running
and extending unit tests can be found in [/src/test/README.md](/src/test/README.md).

There are also [regression and integration tests](/test), written
in Python.
These tests can be run (if the [test dependencies](/test) are installed) with: `build/test/functional/test_runner.py`
(assuming `build` is your build directory).

The CI (Continuous Integration) systems make sure that every pull request is tested on Windows, Linux, and macOS.
The CI must pass on all commits before merge to avoid unrelated CI failures on new pull requests.

### Manual Quality Assurance (QA) Testing

Changes should be tested by somebody other than the developer who wrote the
code. This is especially important for large or high-risk changes. It is useful
to add a test plan to the pull request description if testing the changes is
not straightforward.

Translations
------------

Changes to translations as well as new translations can be submitted to
[OpenSyria Core's Transifex page](https://explore.transifex.com/opensyria/opensyria/).

Translations are periodically pulled from Transifex and merged into the git repository. See the
[translation process](doc/translation_process.md) for details on how this works.

**Important**: We do not accept translation changes as GitHub pull requests because the next
pull from Transifex would automatically overwrite them again.
