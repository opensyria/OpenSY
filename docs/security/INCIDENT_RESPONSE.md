# OpenSY Incident Response Plan

**Version:** 1.0  
**Date:** December 20, 2025  
**Status:** Active  
**Classification:** Public

---

## Purpose

This document defines procedures for responding to security incidents affecting the OpenSY network, infrastructure, or users.

---

## Table of Contents

1. [Incident Severity Levels](#1-incident-severity-levels)
2. [Response Team](#2-response-team)
3. [Incident Response Phases](#3-incident-response-phases)
4. [Specific Playbooks](#4-specific-playbooks)
5. [Communication Templates](#5-communication-templates)
6. [Post-Incident Procedures](#6-post-incident-procedures)

---

## 1. Incident Severity Levels

### SEV-1: Critical (Network Consensus at Risk)

| Criteria | Examples |
|----------|----------|
| Network consensus failure | Chain split, invalid blocks accepted |
| Active 51% attack | Deep reorg in progress, double-spends |
| Critical vulnerability being exploited | RCE, funds at immediate risk |
| Complete network partition | All nodes isolated |

**Response Time:** IMMEDIATE (within minutes)  
**Escalation:** All hands, public communication within 1 hour

### SEV-2: High (Significant Security Impact)

| Criteria | Examples |
|----------|----------|
| Unexploited critical vulnerability | RCE discovered, patch needed |
| Significant hashrate anomaly | >30% sudden change |
| DNS seed compromise | New nodes receiving bad peers |
| Major dependency vulnerability | RandomX, OpenSSL critical CVE |

**Response Time:** <4 hours  
**Escalation:** Core team, coordinated disclosure

### SEV-3: Moderate (Limited Security Impact)

| Criteria | Examples |
|----------|----------|
| DoS vulnerability | Node crash on malformed input |
| Minor reorg (< 6 blocks) | Normal variance vs attack unclear |
| Single seed node failure | Degraded but functional |
| Wallet-only vulnerability | Requires local access |

**Response Time:** <24 hours  
**Escalation:** Security team

### SEV-4: Low (Minimal Security Impact)

| Criteria | Examples |
|----------|----------|
| Information disclosure | Version fingerprinting |
| Documentation error | Misleading security guidance |
| Minor UI/UX security issue | Confusing warning messages |

**Response Time:** <7 days  
**Escalation:** Standard development process

---

## 2. Response Team

### 2.1 Roles

| Role | Responsibilities |
|------|------------------|
| **Incident Commander (IC)** | Overall coordination, decisions, external communication |
| **Technical Lead** | Root cause analysis, fix development |
| **Communications Lead** | User notifications, public statements |
| **Operations Lead** | Infrastructure actions, monitoring |

### 2.2 Contact List

| Role | Primary | Backup |
|------|---------|--------|
| Incident Commander | TBD | TBD |
| Technical Lead | TBD | TBD |
| Communications | TBD | TBD |
| Operations | TBD | TBD |

### 2.3 Escalation Path

```
┌─────────────────┐
│  Incident       │
│  Detected       │
└────────┬────────┘
         │
         ▼
┌─────────────────┐     ┌─────────────────┐
│  SEV-3/4?       │────►│  Security Team  │
│  (Low/Moderate) │ Yes │  handles        │
└────────┬────────┘     └─────────────────┘
         │ No
         ▼
┌─────────────────┐     ┌─────────────────┐
│  SEV-2?         │────►│  Core Team      │
│  (High)         │ Yes │  + Security     │
└────────┬────────┘     └─────────────────┘
         │ No
         ▼
┌─────────────────┐
│  SEV-1          │
│  ALL HANDS      │
│  + Public Comms │
└─────────────────┘
```

---

## 3. Incident Response Phases

### Phase 1: Detection & Triage (0-15 minutes)

```
□ Confirm incident is real (not false positive)
□ Assign initial severity level
□ Identify Incident Commander
□ Create incident channel/thread
□ Begin incident log with timestamps
```

### Phase 2: Containment (15-60 minutes)

```
□ Stop ongoing damage if possible
□ Isolate affected systems
□ Preserve evidence (logs, blocks, network captures)
□ Assess blast radius
□ Decide on public communication timing
```

### Phase 3: Eradication (1-24 hours)

```
□ Identify root cause
□ Develop and test fix
□ Prepare release if needed
□ Coordinate with exchanges/services if needed
□ Draft user guidance
```

### Phase 4: Recovery (Hours to Days)

```
□ Deploy fix
□ Verify normal operation
□ Monitor for recurrence
□ Restore any disabled services
□ Confirm with community
```

### Phase 5: Post-Incident (Within 7 days)

```
□ Complete incident timeline
□ Conduct blameless post-mortem
□ Document lessons learned
□ Update threat model if needed
□ Implement preventive measures
```

---

## 4. Specific Playbooks

### 4.1 Playbook: 51% Attack / Deep Reorg

**Detection Signals:**
- Reorg depth > 6 blocks
- Sudden hashrate spike followed by drop
- Double-spend reports from exchanges

**Immediate Actions:**
```
1. ALERT: Notify all known exchanges to pause deposits
2. MONITOR: Track attacker's chain progress
3. DOCUMENT: Record all reorged transactions
4. ASSESS: Determine if attack is ongoing or complete
```

**If Attack is Ongoing:**
```
5. COORDINATE: Rally honest miners to defend
6. COMMUNICATE: Public statement within 1 hour
7. CONSIDER: Emergency checkpoint (last resort)
```

**If Attack Completed:**
```
5. ANALYZE: Identify double-spent transactions
6. COORDINATE: Work with victims on remediation
7. HARDEN: Implement additional protections
```

### 4.2 Playbook: Critical Vulnerability Disclosed

**If Privately Disclosed:**
```
1. ACKNOWLEDGE: Thank reporter within 24 hours
2. VERIFY: Reproduce and confirm vulnerability
3. DEVELOP: Create patch in private branch
4. TEST: Thorough testing including regression
5. COORDINATE: Notify major infrastructure operators
6. RELEASE: Push update with coordinated disclosure
7. REWARD: Process bug bounty if applicable
```

**If Publicly Disclosed (0-day):**
```
1. ASSESS: Determine if being actively exploited
2. MITIGATE: Deploy any possible mitigations
3. COMMUNICATE: Immediate public acknowledgment
4. FAST-TRACK: Emergency patch development
5. RELEASE: Push update as fast as safely possible
```

### 4.3 Playbook: DNS Seed Compromise

**Detection Signals:**
- New nodes connecting only to suspicious IPs
- Seed returning incorrect addresses
- Community reports of connection issues

**Actions:**
```
1. VERIFY: Confirm seed is compromised
2. DISABLE: Remove from rotation if possible
3. ALERT: Public warning about affected seed
4. REDIRECT: Guide users to manual addnode
5. RESTORE: Deploy replacement seed
6. INVESTIGATE: Determine compromise vector
```

### 4.4 Playbook: RandomX Vulnerability

**Detection Signals:**
- Monero security disclosure
- Academic paper publication
- Anomalous block hashes

**Actions:**
```
1. ASSESS: Applicability to OpenSY
2. COORDINATE: Contact Monero team if needed
3. PATCH: Apply upstream fix or develop custom
4. CONSIDER: If critical, evaluate algorithm change
5. COMMUNICATE: Technical disclosure after patch
```

---

## 5. Communication Templates

### 5.1 Initial Acknowledgment (SEV-1/2)

```
TITLE: [Security] Investigating Network Anomaly

We are aware of [brief description] and are actively investigating.

Status: Under Investigation
Impact: Being assessed
Action Required: [None yet / Pause transactions / etc.]

We will provide updates as we learn more.

- OpenSY Security Team
[Timestamp UTC]
```

### 5.2 Confirmed Incident

```
TITLE: [Security] Confirmed: [Incident Type]

We have confirmed [description of incident].

What happened: [Brief explanation]
Impact: [Who/what is affected]
Status: [Contained / Ongoing / Resolved]

Recommended Actions:
- [Action 1]
- [Action 2]

Timeline:
- [Time]: [Event]
- [Time]: [Event]

Next update: [Time]

- OpenSY Security Team
```

### 5.3 Resolution Notice

```
TITLE: [Security] Resolved: [Incident Type]

The [incident] has been resolved.

Summary: [What happened]
Resolution: [How it was fixed]
Impact: [Final assessment]

Action Required:
- [Update to version X.Y.Z]
- [Other user actions]

A detailed post-mortem will be published within [timeframe].

Thank you to [acknowledgments].

- OpenSY Security Team
```

---

## 6. Post-Incident Procedures

### 6.1 Post-Mortem Template

```markdown
# Incident Post-Mortem: [Title]

**Date:** [Incident date]
**Duration:** [Start to resolution]
**Severity:** [SEV level]
**Author:** [Name]

## Summary
[2-3 sentence summary]

## Timeline (All times UTC)
| Time | Event |
|------|-------|
| HH:MM | [Event] |

## Root Cause
[Technical explanation of what went wrong]

## Impact
- Users affected: [Number/scope]
- Funds at risk: [Amount if any]
- Duration: [Downtime/exposure window]

## What Went Well
- [Positive 1]
- [Positive 2]

## What Went Poorly
- [Issue 1]
- [Issue 2]

## Action Items
| Action | Owner | Due Date | Status |
|--------|-------|----------|--------|
| [Task] | [Name] | [Date] | [Status] |

## Lessons Learned
[Key takeaways for future prevention]
```

### 6.2 Metrics to Track

| Metric | Target |
|--------|--------|
| Time to detect | < 15 minutes for SEV-1 |
| Time to acknowledge | < 1 hour for SEV-1/2 |
| Time to contain | < 4 hours for SEV-1 |
| Time to resolve | < 24 hours for SEV-2 |
| Post-mortem completion | < 7 days |

---

## Appendix A: Emergency Contacts

| Service | Contact | Purpose |
|---------|---------|---------|
| GitHub Security | security@github.com | Repository compromise |
| Cloudflare | [Dashboard] | DDoS mitigation |
| AWS Support | [Console] | Infrastructure issues |

---

## Appendix B: Useful Commands

```bash
# Check for unusual reorgs
opensy-cli getchaintips

# Get peer information
opensy-cli getpeerinfo

# Check for banned peers
opensy-cli listbanned

# Network hash rate estimate
opensy-cli getnetworkhashps

# Get block at specific height
opensy-cli getblockhash <height>
opensy-cli getblock <hash>
```

---

## Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-12-20 | Audit Team | Initial incident response plan |

---

*This document should be tested via tabletop exercises quarterly.*
