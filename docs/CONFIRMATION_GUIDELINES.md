# OpenSY Transaction Confirmation Guidelines

**Version:** 1.0  
**Date:** December 20, 2025

---

## Overview

This document provides recommendations for how many block confirmations to wait before considering a transaction final. These recommendations balance security against user experience.

---

## Quick Reference

| Transaction Value | Recommended Confirmations | Wait Time | Risk Level |
|-------------------|---------------------------|-----------|------------|
| Micro (<100 SYL) | 1 confirmation | ~2 min | Accept some risk |
| Small (<10,000 SYL) | 6 confirmations | ~12 min | Low risk |
| Medium (<100,000 SYL) | 12 confirmations | ~24 min | Very low risk |
| Large (<1,000,000 SYL) | 30 confirmations | ~1 hour | Minimal risk |
| Very Large (>1M SYL) | 60+ confirmations | ~2 hours | Ultra-secure |
| Exchange Deposits | 30-100 confirmations | 1-3 hours | Industry standard |

---

## Understanding Confirmations

### What is a Confirmation?

When your transaction is included in a block, it has **1 confirmation**. Each subsequent block adds another confirmation.

```
Your TX → Block 100 → Block 101 → Block 102 → Block 103
          ↑           ↑           ↑           ↑
          1 conf      2 conf      3 conf      4 conf
```

### Why Confirmations Matter

With more confirmations:
- ✅ Higher cost for attacker to reverse your transaction
- ✅ Lower probability of natural reorganization
- ✅ Greater certainty of finality

---

## Risk Analysis

### Reorganization Probability

| Confirmations | Reorganization Probability | Notes |
|---------------|---------------------------|-------|
| 0 (unconfirmed) | High | In mempool only |
| 1 | ~2-5% | Natural variance |
| 2 | ~0.5% | Much safer |
| 3 | ~0.1% | Recommended minimum |
| 6 | ~0.001% | Bitcoin standard |
| 12 | ~0.00001% | Very safe |
| 30+ | Negligible | Attack-resistant |

### Attack Cost Estimation

To reverse a transaction with N confirmations, an attacker must:
1. Control >50% of network hashrate
2. Mine N+1 blocks secretly
3. Publish alternate chain

**Cost increases exponentially with confirmations.**

---

## Recommendations by Use Case

### Point-of-Sale (Physical Goods)

| Scenario | Confirmations | Reasoning |
|----------|---------------|-----------|
| Coffee shop (<50 SYL) | 0-1 | Speed matters; fraud risk absorbed |
| Retail (<5,000 SYL) | 3 | Balance of speed and security |
| High-value electronics | 6+ | Worth the wait |

### Online Services

| Scenario | Confirmations | Reasoning |
|----------|---------------|-----------|
| Digital downloads | 1-3 | Reversible service |
| SaaS subscription | 3-6 | Monthly relationship |
| One-time purchase | 6 | Standard caution |

### Financial Services

| Scenario | Confirmations | Reasoning |
|----------|---------------|-----------|
| Exchange deposit | 30-100 | High-value targets |
| Remittance/Transfer | 12-30 | Balance of speed |
| Institutional settlement | 60+ | Maximum security |

### Miner Coinbase Rewards

| Scenario | Confirmations | Reasoning |
|----------|---------------|-----------|
| Coinbase maturity | 100 | **Consensus rule** (not optional) |

> **Note:** Coinbase transactions (mining rewards) require exactly 100 confirmations before they can be spent. This is enforced by consensus, not a recommendation.

---

## Implementation Guidance

### For Wallet Developers

```python
def get_recommended_confirmations(amount_syl: float) -> int:
    """Return recommended confirmations based on transaction value."""
    if amount_syl < 100:
        return 1
    elif amount_syl < 10_000:
        return 6
    elif amount_syl < 100_000:
        return 12
    elif amount_syl < 1_000_000:
        return 30
    else:
        return 60

def get_confirmation_status(confirmations: int, amount: float) -> str:
    """Return human-readable status."""
    recommended = get_recommended_confirmations(amount)
    
    if confirmations == 0:
        return "⏳ Unconfirmed - Pending"
    elif confirmations < recommended:
        return f"⚠️ {confirmations}/{recommended} - Confirming"
    else:
        return f"✅ {confirmations} - Confirmed"
```

### For Exchange Operators

```python
# Conservative exchange deposit policy
DEPOSIT_CONFIRMATIONS = {
    "SYL": 30,  # Base requirement
}

# Consider dynamic adjustment based on:
# - Network hashrate
# - Deposit amount
# - User history/KYC level
def get_deposit_confirmations(amount: float, user_tier: str) -> int:
    base = DEPOSIT_CONFIRMATIONS["SYL"]
    
    # Increase for large amounts
    if amount > 1_000_000:
        base += 30
    elif amount > 100_000:
        base += 15
    
    # Trusted users may get faster processing
    if user_tier == "verified":
        base = max(15, base - 10)
    
    return base
```

### For Merchants

```javascript
// Example: WooCommerce-style integration
const CONFIRMATION_THRESHOLDS = {
  low_risk: 1,      // Digital goods, returnable
  medium_risk: 3,   // Physical goods, standard
  high_risk: 6,     // High-value, non-returnable
  exchange: 30      // Financial services
};

function shouldConfirmOrder(txConfirmations, riskLevel) {
  return txConfirmations >= CONFIRMATION_THRESHOLDS[riskLevel];
}
```

---

## Monitoring for Anomalies

### Red Flags

| Signal | Meaning | Action |
|--------|---------|--------|
| Deep reorg (>3 blocks) | Possible attack | Pause high-value processing |
| Hashrate drop >30% | Reduced security | Increase confirmation requirements |
| Double-spend attempt | Active attack | Reject unconfirmed, increase confirms |

### RPC Commands for Monitoring

```bash
# Check chain tips for forks
opensy-cli getchaintips

# Monitor for reorgs
opensy-cli getblockchaininfo | jq '.blocks, .headers'

# Check transaction confirmations
opensy-cli gettransaction <txid> | jq '.confirmations'
```

---

## Special Considerations for OpenSY

### 2-Minute Blocks

OpenSY's 2-minute block time means:
- 6 confirmations = 12 minutes (vs. Bitcoin's 60 minutes)
- Faster confirmation but same security per block
- Consider slightly higher confirmation counts for very large amounts

### RandomX & 51% Attacks

RandomX's ASIC-resistance means:
- No specialized hardware dominates
- Hashrate is more distributed
- 51% attacks require controlling many CPUs
- Cloud computing is main attack vector

### Bootstrap Phase Considerations

During the network's early phase (current):
- Hashrate is relatively low
- Consider higher confirmation requirements
- Monitor network health closely

---

## Document History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2025-12-20 | Initial confirmation guidelines |
