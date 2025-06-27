// Main controller for the EO Bridge web interface
class BridgeController {
    constructor() {
        this.initElements();
        this.bindEvents();
        this.loadInitialData();
        this.startStatusUpdates();
    }

    initElements() {
        this.status = document.getElementById('status');
        this.portInput = document.getElementById('port');
        this.discoveryCheck = document.getElementById('discovery');
        this.serverSelect = document.getElementById('serverSelect');
        this.manualServer = document.getElementById('manualServer');
        this.startBtn = document.getElementById('startBtn');
        this.stopBtn = document.getElementById('stopBtn');
        this.refreshBtn = document.getElementById('refreshBtn');
    }

    bindEvents() {
        this.discoveryCheck.addEventListener('change', () => this.toggleDiscovery());
        this.serverSelect.addEventListener('change', () => this.selectServer());
        this.startBtn.addEventListener('click', () => this.startBridge());
        this.stopBtn.addEventListener('click', () => this.stopBridge());
        this.refreshBtn.addEventListener('click', () => this.refreshServers());
    }

    async loadInitialData() {
        await this.updateStatus();
        await this.loadServers();
    }

    // Poll bridge status every 2 seconds
    startStatusUpdates() {
        setInterval(() => this.updateStatus(), 2000);
    }

    async updateStatus() {
        try {
            const response = await fetch('/api/status');
            const data = await response.json();
            
            this.status.textContent = data.status;
            this.status.className = `status ${data.running ? 'running' : 'stopped'}`;
            
            this.updateUIState(data.running);
        } catch (error) {
            console.error('Failed to update status:', error);
        }
    }

    updateUIState(isRunning) {
        const controls = [this.portInput, this.discoveryCheck];
        
        if (isRunning) {
            // Disable all controls when running
            controls.forEach(el => el.disabled = true);
            this.serverSelect.disabled = true;
            this.manualServer.disabled = true;
            this.startBtn.style.display = 'none';
            this.startBtn.disabled = true;
            this.stopBtn.style.display = 'inline-block';
            this.stopBtn.disabled = false;
        } else {
            // Re-enable controls when stopped
            controls.forEach(el => el.disabled = false);
            
            // Set server input states based on discovery checkbox
            if (this.discoveryCheck.checked) {
                this.serverSelect.disabled = false;
                this.manualServer.disabled = true;
            } else {
                this.serverSelect.disabled = true;
                this.manualServer.disabled = false;
            }
            
            // Ensure buttons are properly enabled/disabled when stopped
            this.startBtn.style.display = 'inline-block';
            this.startBtn.disabled = false;
            this.stopBtn.style.display = 'none';
            this.stopBtn.disabled = true;
        }
    }

    async loadServers() {
        try {
            const response = await fetch('/api/servers');
            const servers = await response.json();
            
            this.serverSelect.innerHTML = '<option value="">Select a server...</option>';
            
            servers.forEach(server => {
                const option = document.createElement('option');
                option.value = `${server.host}:${server.port}`;
                option.textContent = `${server.name} (${server.players} players) - ${server.host}:${server.port}`;
                this.serverSelect.appendChild(option);
            });
        } catch (error) {
            console.error('Failed to load servers:', error);
        }
    }

    // Toggle between server discovery and manual input
    toggleDiscovery() {
        if (this.discoveryCheck.checked) {
            this.serverSelect.style.display = 'block';
            this.manualServer.style.display = 'none';
            this.serverSelect.disabled = false;
            this.manualServer.disabled = true;
        } else {
            this.serverSelect.style.display = 'none';
            this.manualServer.style.display = 'block';
            this.serverSelect.disabled = true;
            this.manualServer.disabled = false;
        }
    }

    selectServer() {
        if (this.serverSelect.value) {
            this.manualServer.value = this.serverSelect.value;
        }
    }

    // Start the bridge with validation
    async startBridge() {
        const port = parseInt(this.portInput.value);
        const server = this.manualServer.value;
        
        if (!port || port < 1 || port > 65535) {
            this.showError('Please enter a valid port number (1-65535)');
            return;
        }
        
        if (!server) {
            this.showError('Please specify a game server');
            return;
        }
        
        this.setButtonLoading(this.startBtn, 'Starting...');
        
        try {
            const response = await fetch('/api/start', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({port, server})
            });
            
            const data = await response.json();
            
            if (data.success) {
                await this.updateStatus();
            } else {
                this.showError(`Failed to start: ${data.error}`);
                this.resetButton(this.startBtn, 'Start Bridge');
            }
        } catch (error) {
            this.showError('Network error occurred');
            this.resetButton(this.startBtn, 'Start Bridge');
        }
    }

    async stopBridge() {
        this.setButtonLoading(this.stopBtn, 'Stopping...');
        
        try {
            const response = await fetch('/api/stop', {method: 'POST'});
            const data = await response.json();
            
            if (data.success) {
                await this.updateStatus();
            } else {
                this.showError(`Failed to stop: ${data.error}`);
                this.resetButton(this.stopBtn, 'Stop Bridge');
            }
        } catch (error) {
            this.showError('Network error occurred');
            this.resetButton(this.stopBtn, 'Stop Bridge');
        }
    }

    async refreshServers() {
        this.setButtonLoading(this.refreshBtn, 'Loading...');
        
        try {
            const response = await fetch('/api/refresh-servers', {method: 'POST'});
            const data = await response.json();
            
            if (data.success) {
                await this.loadServers();
                this.showSuccess('Servers refreshed successfully');
            } else {
                this.showError(`Failed to refresh: ${data.error}`);
            }
        } catch (error) {
            this.showError('Failed to fetch servers');
        } finally {
            this.resetButton(this.refreshBtn, 'Refresh Servers');
        }
    }

    setButtonLoading(button, text) {
        button.textContent = text;
        button.disabled = true;
    }

    resetButton(button, text) {
        button.textContent = text;
        button.disabled = false;
    }

    showError(message) {
        // Simple alert for now - could be replaced with toast notifications
        alert(`Error: ${message}`);
    }

    showSuccess(message) {
        // Simple alert for now - could be replaced with toast notifications
        console.log(`Success: ${message}`);
    }
}

// Initialize the app when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    new BridgeController();
});