// Copyright (c) 2025 The OpenSY developers
// Distributed under the MIT software license, see the accompanying
// file COPYING or http://www.opensource.org/licenses/mit-license.php.

#include <crypto/randomx_pool.h>
#include <logging.h>
#include <util/check.h>

#include <algorithm>

RandomXContextPool g_randomx_pool;

RandomXContextPool::RandomXContextPool() = default;

RandomXContextPool::~RandomXContextPool()
{
    LOCK(m_mutex);
    m_pool.clear();
}

bool RandomXContextPool::ShouldYieldToHigherPriority(AcquisitionPriority my_priority) const
{
    AssertLockHeld(m_mutex);
    
    switch (my_priority) {
        case AcquisitionPriority::NORMAL:
            // Normal priority yields to both HIGH and CONSENSUS_CRITICAL
            return (m_waiting_high > 0 || m_waiting_consensus_critical > 0);
        case AcquisitionPriority::HIGH:
            // High priority only yields to CONSENSUS_CRITICAL
            return (m_waiting_consensus_critical > 0);
        case AcquisitionPriority::CONSENSUS_CRITICAL:
            // Never yields
            return false;
    }
    return false;
}

std::chrono::seconds RandomXContextPool::GetTimeoutForPriority(AcquisitionPriority priority) const
{
    switch (priority) {
        case AcquisitionPriority::NORMAL:
            return ACQUIRE_TIMEOUT;
        case AcquisitionPriority::HIGH:
            return HIGH_PRIORITY_TIMEOUT;
        case AcquisitionPriority::CONSENSUS_CRITICAL:
            // Return a very long timeout - effectively infinite for practical purposes
            // Using max would cause overflow issues, so use 24 hours
            return std::chrono::seconds{86400};
    }
    return ACQUIRE_TIMEOUT;
}

std::optional<RandomXContextPool::ContextGuard> RandomXContextPool::Acquire(
    const uint256& keyBlockHash, AcquisitionPriority priority)
{
    WAIT_LOCK(m_mutex, lock);

    const bool is_consensus_critical = (priority == AcquisitionPriority::CONSENSUS_CRITICAL);
    const bool is_high = (priority == AcquisitionPriority::HIGH);
    
    auto timeout = GetTimeoutForPriority(priority);
    auto deadline = std::chrono::steady_clock::now() + timeout;

    // Track waiting threads by priority
    if (is_consensus_critical) {
        m_waiting_consensus_critical++;
    } else if (is_high) {
        m_waiting_high++;
    } else {
        m_waiting_normal++;
    }

    // RAII cleanup for waiting count
    struct WaitGuard {
        RandomXContextPool& pool;
        AcquisitionPriority priority;
        bool decremented{false};
        
        WaitGuard(RandomXContextPool& p, AcquisitionPriority prio) : pool(p), priority(prio) {}
        ~WaitGuard() { decrement(); }
        
        void decrement() {
            if (decremented) return;
            decremented = true;
            switch (priority) {
                case AcquisitionPriority::CONSENSUS_CRITICAL:
                    pool.m_waiting_consensus_critical--;
                    break;
                case AcquisitionPriority::HIGH:
                    pool.m_waiting_high--;
                    break;
                case AcquisitionPriority::NORMAL:
                    pool.m_waiting_normal--;
                    break;
            }
        }
    } wait_guard(*this, priority);

    while (true) {
        // Check if we should yield to higher priority waiters
        // (only if there's no context immediately available)
        size_t index = FindOrCreateContext(keyBlockHash);
        
        if (index != SIZE_MAX && !ShouldYieldToHigherPriority(priority)) {
            // Found or created a context
            m_pool[index].in_use = true;
            m_pool[index].last_used = std::chrono::steady_clock::now();
            m_total_acquisitions++;
            
            if (is_consensus_critical) {
                m_consensus_critical_acquisitions++;
            } else if (is_high) {
                m_high_priority_acquisitions++;
            }

            // Initialize or reinitialize if key changed
            if (m_pool[index].key_hash != keyBlockHash) {
                if (!m_pool[index].context->Initialize(keyBlockHash)) {
                    // Initialization failed - mark as not in use and return error
                    m_pool[index].in_use = false;
                    m_cv.notify_all();  // Wake everyone to retry
                    return std::nullopt;
                }
                m_pool[index].key_hash = keyBlockHash;
                m_key_reinitializations++;
            }

            // Decrement wait counter before returning
            wait_guard.decrement();
            
            return ContextGuard(m_pool[index].context.get(), *this, index);
        }

        // Need to wait - track if we're being preempted
        if (index != SIZE_MAX && ShouldYieldToHigherPriority(priority)) {
            m_priority_preemptions++;
            LogPrintf("RandomXContextPool: %s priority request yielding to higher priority\n",
                priority == AcquisitionPriority::NORMAL ? "NORMAL" : "HIGH");
        }

        m_total_waits++;

        // For CONSENSUS_CRITICAL, we never timeout - keep waiting
        if (is_consensus_critical) {
            // Wait indefinitely but check periodically for context availability
            m_cv.wait_for(lock, std::chrono::seconds{5});
            // Always retry - consensus critical never gives up
            continue;
        }

        // For other priorities, respect timeout
        if (m_cv.wait_until(lock, deadline) == std::cv_status::timeout) {
            m_total_timeouts++;
            LogPrintf("RandomXContextPool: Timeout waiting for context (priority=%s, active=%zu, waiting_cc=%zu)\n",
                is_high ? "HIGH" : "NORMAL",
                std::count_if(m_pool.begin(), m_pool.end(), [](const PoolEntry& e) { return e.in_use; }),
                m_waiting_consensus_critical);
            return std::nullopt;
        }
    }
}

size_t RandomXContextPool::FindOrCreateContext(const uint256& keyBlockHash)
{
    AssertLockHeld(m_mutex);

    // First, look for an available context with matching key (best case)
    for (size_t i = 0; i < m_pool.size(); ++i) {
        if (!m_pool[i].in_use && m_pool[i].key_hash == keyBlockHash) {
            return i;
        }
    }

    // Second, look for any available context
    for (size_t i = 0; i < m_pool.size(); ++i) {
        if (!m_pool[i].in_use) {
            return i;
        }
    }

    // Third, if pool isn't full, create a new context
    if (m_pool.size() < m_max_contexts) {
        PoolEntry entry;
        entry.context = std::make_unique<RandomXContext>();
        entry.in_use = false;
        m_pool.push_back(std::move(entry));
        return m_pool.size() - 1;
    }

    // Pool is full and all contexts are in use
    return SIZE_MAX;
}

void RandomXContextPool::Return(size_t index)
{
    if (index == SIZE_MAX) return;

    {
        LOCK(m_mutex);
        if (index < m_pool.size()) {
            m_pool[index].in_use = false;
            m_pool[index].last_used = std::chrono::steady_clock::now();
        }
    }
    // Notify all waiters - priority is handled in Acquire()
    m_cv.notify_all();
}

RandomXContextPool::PoolStats RandomXContextPool::GetStats() const
{
    LOCK(m_mutex);

    PoolStats stats;
    stats.total_contexts = m_pool.size();
    stats.active_contexts = std::count_if(m_pool.begin(), m_pool.end(),
        [](const PoolEntry& e) { return e.in_use; });
    stats.available_contexts = stats.total_contexts - stats.active_contexts;
    stats.total_acquisitions = m_total_acquisitions;
    stats.total_waits = m_total_waits;
    stats.total_timeouts = m_total_timeouts;
    stats.key_reinitializations = m_key_reinitializations;
    stats.consensus_critical_acquisitions = m_consensus_critical_acquisitions;
    stats.high_priority_acquisitions = m_high_priority_acquisitions;
    stats.priority_preemptions = m_priority_preemptions;

    return stats;
}

bool RandomXContextPool::SetMaxContexts(size_t max_contexts)
{
    LOCK(m_mutex);

    // Can only change before any contexts are created
    if (!m_pool.empty()) {
        return false;
    }

    if (max_contexts == 0 || max_contexts > 64) {
        return false;  // Sanity bounds
    }

    m_max_contexts = max_contexts;
    return true;
}
