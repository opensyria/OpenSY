// CoopMine Dashboard JavaScript
class CoopMineDashboard {
    constructor() {
        this.ws = null;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 10;
        this.reconnectDelay = 2000;
        
        this.elements = {
            hashrate: document.getElementById('hashrate'),
            hashrateUnit: document.getElementById('hashrate-unit'),
            workers: document.getElementById('workers'),
            shares: document.getElementById('shares'),
            blocks: document.getElementById('blocks'),
            workersTbody: document.getElementById('workers-tbody'),
            workerCount: document.getElementById('worker-count'),
            clusterId: document.getElementById('cluster-id'),
            clusterName: document.getElementById('cluster-name'),
            uptime: document.getElementById('uptime'),
            poolStatus: document.getElementById('pool-status'),
            connectionStatus: document.getElementById('connection-status'),
        };
        
        this.connect();
    }
    
    connect() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;
        
        console.log('Connecting to WebSocket:', wsUrl);
        
        this.ws = new WebSocket(wsUrl);
        
        this.ws.onopen = () => {
            console.log('WebSocket connected');
            this.reconnectAttempts = 0;
            this.updateConnectionStatus('connected');
        };
        
        this.ws.onclose = () => {
            console.log('WebSocket disconnected');
            this.updateConnectionStatus('disconnected');
            this.scheduleReconnect();
        };
        
        this.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
        };
        
        this.ws.onmessage = (event) => {
            try {
                const message = JSON.parse(event.data);
                this.handleMessage(message);
            } catch (e) {
                console.error('Failed to parse message:', e);
            }
        };
    }
    
    scheduleReconnect() {
        if (this.reconnectAttempts < this.maxReconnectAttempts) {
            this.reconnectAttempts++;
            const delay = this.reconnectDelay * Math.min(this.reconnectAttempts, 5);
            console.log(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);
            setTimeout(() => this.connect(), delay);
        }
    }
    
    updateConnectionStatus(status) {
        const el = this.elements.connectionStatus;
        el.className = 'connection-status ' + status;
        el.querySelector('.status-text').textContent = 
            status === 'connected' ? 'Connected' : 
            status === 'disconnected' ? 'Disconnected' : 'Connecting...';
    }
    
    handleMessage(message) {
        if (message.type === 'stats') {
            this.updateStats(message.data);
        }
    }
    
    updateStats(stats) {
        // Update main stats
        this.elements.hashrate.textContent = this.formatNumber(stats.total_hashrate);
        this.elements.hashrateUnit.textContent = stats.hashrate_unit || 'H/s';
        this.elements.workers.textContent = stats.workers_online || 0;
        this.elements.shares.textContent = this.formatNumber(stats.shares_valid);
        this.elements.blocks.textContent = stats.blocks_found || 0;
        
        // Update cluster info
        this.elements.clusterId.textContent = stats.cluster_id || '--';
        this.elements.clusterName.textContent = stats.cluster_name || '--';
        this.elements.uptime.textContent = this.formatUptime(stats.uptime_seconds);
        this.elements.poolStatus.textContent = stats.pool_connected ? '✓ Connected' : '✗ Disconnected';
        this.elements.poolStatus.style.color = stats.pool_connected ? '#3fb950' : '#f85149';
        
        // Update workers table
        this.updateWorkersTable(stats.workers || []);
    }
    
    updateWorkersTable(workers) {
        const tbody = this.elements.workersTbody;
        this.elements.workerCount.textContent = `${workers.length} worker${workers.length !== 1 ? 's' : ''}`;
        
        if (workers.length === 0) {
            tbody.innerHTML = `
                <tr class="empty-row">
                    <td colspan="5">No workers connected</td>
                </tr>
            `;
            return;
        }
        
        tbody.innerHTML = workers.map(worker => `
            <tr>
                <td>
                    <strong>${this.escapeHtml(worker.name)}</strong>
                    <br><small style="color: var(--text-muted)">${worker.id}</small>
                </td>
                <td>${this.getStatusBadge(worker.status)}</td>
                <td>${this.formatHashrate(worker.hashrate)}</td>
                <td>${this.formatNumber(worker.shares)}</td>
                <td>${this.formatLastSeen(worker.last_seen)}</td>
            </tr>
        `).join('');
    }
    
    getStatusBadge(status) {
        const statusMap = {
            'mining': { class: 'online', text: '● Mining' },
            'idle': { class: 'idle', text: '○ Idle' },
            'offline': { class: 'offline', text: '● Offline' },
        };
        const s = statusMap[status?.toLowerCase()] || { class: 'idle', text: status };
        return `<span class="status-badge ${s.class}">${s.text}</span>`;
    }
    
    formatNumber(num) {
        if (num === undefined || num === null) return '--';
        if (num >= 1e9) return (num / 1e9).toFixed(2) + 'B';
        if (num >= 1e6) return (num / 1e6).toFixed(2) + 'M';
        if (num >= 1e3) return (num / 1e3).toFixed(2) + 'K';
        return num.toFixed(2);
    }
    
    formatHashrate(h) {
        if (!h) return '--';
        if (h >= 1e12) return (h / 1e12).toFixed(2) + ' TH/s';
        if (h >= 1e9) return (h / 1e9).toFixed(2) + ' GH/s';
        if (h >= 1e6) return (h / 1e6).toFixed(2) + ' MH/s';
        if (h >= 1e3) return (h / 1e3).toFixed(2) + ' KH/s';
        return h.toFixed(2) + ' H/s';
    }
    
    formatUptime(seconds) {
        if (!seconds) return '--';
        const days = Math.floor(seconds / 86400);
        const hours = Math.floor((seconds % 86400) / 3600);
        const mins = Math.floor((seconds % 3600) / 60);
        
        if (days > 0) return `${days}d ${hours}h ${mins}m`;
        if (hours > 0) return `${hours}h ${mins}m`;
        return `${mins}m`;
    }
    
    formatLastSeen(timestamp) {
        if (!timestamp) return '--';
        const now = Date.now() / 1000;
        const diff = now - timestamp;
        
        if (diff < 60) return 'Just now';
        if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
        if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
        return `${Math.floor(diff / 86400)}d ago`;
    }
    
    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}

// Initialize dashboard when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    window.dashboard = new CoopMineDashboard();
});
