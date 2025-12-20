# OpenSY Supply Schedule & Emission Curve

**Version:** 1.0  
**Date:** December 20, 2025

---

## Overview

OpenSY follows a Bitcoin-style geometric emission schedule with parameters adjusted for a 21 billion SYL maximum supply.

---

## Key Parameters

| Parameter | Value |
|-----------|-------|
| **Initial Block Reward** | 10,000 SYL |
| **Block Time** | 2 minutes (120 seconds) |
| **Halving Interval** | 1,050,000 blocks (~4 years) |
| **Maximum Supply** | 21,000,000,000 SYL (21 billion) |
| **Smallest Unit** | 1 qirsh = 0.00000001 SYL |

---

## Emission Schedule

### Halving Timeline

| Era | Block Range | Block Reward | Era Supply | Cumulative Supply | % of Max |
|-----|-------------|--------------|------------|-------------------|----------|
| 1 | 0 - 1,049,999 | 10,000 SYL | 10,500,000,000 | 10,500,000,000 | 50.00% |
| 2 | 1,050,000 - 2,099,999 | 5,000 SYL | 5,250,000,000 | 15,750,000,000 | 75.00% |
| 3 | 2,100,000 - 3,149,999 | 2,500 SYL | 2,625,000,000 | 18,375,000,000 | 87.50% |
| 4 | 3,150,000 - 4,199,999 | 1,250 SYL | 1,312,500,000 | 19,687,500,000 | 93.75% |
| 5 | 4,200,000 - 5,249,999 | 625 SYL | 656,250,000 | 20,343,750,000 | 96.88% |
| 6 | 5,250,000 - 6,299,999 | 312.5 SYL | 328,125,000 | 20,671,875,000 | 98.44% |
| 7 | 6,300,000 - 7,349,999 | 156.25 SYL | 164,062,500 | 20,835,937,500 | 99.22% |
| 8 | 7,350,000 - 8,399,999 | 78.125 SYL | 82,031,250 | 20,917,968,750 | 99.61% |
| ... | ... | ... | ... | ... | ... |
| 64 | ~66,150,000+ | 0 SYL | 0 | ~21,000,000,000 | 100% |

### Approximate Dates (at target block time)

| Era | Start Block | Approximate Date | Reward |
|-----|-------------|------------------|--------|
| 1 | 0 | December 2024 | 10,000 SYL |
| 2 | 1,050,000 | December 2028 | 5,000 SYL |
| 3 | 2,100,000 | December 2032 | 2,500 SYL |
| 4 | 3,150,000 | December 2036 | 1,250 SYL |
| 5 | 4,200,000 | December 2040 | 625 SYL |
| 6 | 5,250,000 | December 2044 | 312.5 SYL |
| 7 | 6,300,000 | December 2048 | 156.25 SYL |
| 8 | 7,350,000 | December 2052 | 78.125 SYL |

---

## Visual Emission Curve

```
Supply (Billions SYL)
21 ─────────────────────────────────────────────────────────── MAX
   │                                                    ▄▄▄▄▄▄▄
20 │                                              ▄▄▄▄▄▀
   │                                        ▄▄▄▄▄▀
19 │                                   ▄▄▄▀▀
   │                              ▄▄▄▀▀
18 │                         ▄▄▄▀▀
   │                    ▄▄▄▀▀
17 │               ▄▄▄▀▀
   │          ▄▄▄▀▀
16 │      ▄▄▀▀
   │   ▄▄▀
15 │ ▄▀
   │▄▀
14 ▄▀
   │
   │
10 ├──────┐
   │      │ First halving
   │      │ (50% of supply mined)
 5 │      └──────┐
   │             │
   │             └──────┐
   │                    └───────────────────────────────────────
 0 └────────────────────────────────────────────────────────────►
   0      4      8      12     16     20     24     28     32   Years
         │      │       │
    1st Halving │   3rd Halving
           2nd Halving
```

---

## Annual Inflation Rate

| Year | Block Range (approx) | New Supply | Existing Supply | Inflation Rate |
|------|---------------------|------------|-----------------|----------------|
| 1 | 0 - 262,800 | 2,628,000,000 | 2,628,000,000 | N/A (genesis) |
| 2 | 262,800 - 525,600 | 2,628,000,000 | 5,256,000,000 | 100.0% |
| 3 | 525,600 - 788,400 | 2,628,000,000 | 7,884,000,000 | 50.0% |
| 4 | 788,400 - 1,051,200 | 2,628,000,000 | 10,512,000,000 | 33.3% |
| 5 | 1,051,200 - 1,314,000 | 1,314,000,000 | 11,826,000,000 | 12.5% |
| 6 | 1,314,000 - 1,576,800 | 1,314,000,000 | 13,140,000,000 | 11.1% |
| 7 | 1,576,800 - 1,839,600 | 1,314,000,000 | 14,454,000,000 | 10.0% |
| 8 | 1,839,600 - 2,102,400 | 1,314,000,000 | 15,768,000,000 | 9.1% |
| 9 | 2,102,400 - 2,365,200 | 657,000,000 | 16,425,000,000 | 4.2% |
| 10 | 2,365,200 - 2,628,000 | 657,000,000 | 17,082,000,000 | 4.0% |

---

## Block Reward Lookup

### Formula

```python
def get_block_reward(height: int) -> int:
    """Returns block reward in qirsh (smallest unit)."""
    HALVING_INTERVAL = 1_050_000
    INITIAL_REWARD = 10_000 * 100_000_000  # 10,000 SYL in qirsh
    
    halvings = height // HALVING_INTERVAL
    
    if halvings >= 64:
        return 0
    
    return INITIAL_REWARD >> halvings
```

### Quick Reference Table

| Height | Block Reward |
|--------|--------------|
| 0 | 10,000 SYL |
| 10,000 | 10,000 SYL |
| 100,000 | 10,000 SYL |
| 500,000 | 10,000 SYL |
| 1,000,000 | 10,000 SYL |
| 1,050,000 | 5,000 SYL (first halving) |
| 2,000,000 | 5,000 SYL |
| 2,100,000 | 2,500 SYL (second halving) |
| 5,000,000 | 312.5 SYL |
| 10,000,000 | 9.765625 SYL |

---

## Comparison with Bitcoin

| Metric | OpenSY | Bitcoin |
|--------|--------|---------|
| Max Supply | 21,000,000,000 SYL | 21,000,000 BTC |
| Initial Reward | 10,000 SYL | 50 BTC |
| Block Time | 2 minutes | 10 minutes |
| Halving Interval | 1,050,000 blocks | 210,000 blocks |
| Halving Time | ~4 years | ~4 years |
| Smallest Unit | 1 qirsh | 1 satoshi |

**Key Difference:** OpenSY has 1000x the supply of Bitcoin with 200x the block reward, creating a more accessible unit denomination for everyday transactions.

---

## Circulating Supply Query

To check current circulating supply, use:

```bash
# Calculate from block height
opensy-cli getblockchaininfo | jq '.blocks'

# Then calculate:
# For blocks 0-1,049,999: supply = blocks × 10,000
# For blocks 1,050,000+: account for halvings
```

---

## Economic Considerations

### Why 21 Billion?

1. **Accessibility:** Larger numbers feel more accessible for daily transactions
2. **Psychological:** "100 SYL" is easier to understand than "0.0001 BTC"
3. **Syrian Context:** Historically, large denomination currencies are common
4. **Future-Proofing:** Allows for significant economic activity without decimals

### Supply Predictability

- Every SYL that will ever exist follows this exact schedule
- No premine, no founder rewards, no inflation surprises
- Genesis coinbase is provably unspendable (Bitcoin's original key)

### Long-Term Security

As block rewards decrease, transaction fees must eventually cover miner costs. This transition will occur gradually over 50+ years, allowing the fee market to develop organically.

---

## Document History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2025-12-20 | Initial supply schedule documentation |
