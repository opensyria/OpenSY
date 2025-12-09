const axios = require('axios');

const rpcConfig = {
    host: process.env.RPC_HOST || '127.0.0.1',
    port: process.env.RPC_PORT || 9632,
    user: process.env.RPC_USER || 'opensyria',
    password: process.env.RPC_PASSWORD || ''
};

async function call(method, params = []) {
    const url = `http://${rpcConfig.host}:${rpcConfig.port}`;
    
    const response = await axios.post(url, {
        jsonrpc: '1.0',
        id: Date.now(),
        method,
        params
    }, {
        auth: {
            username: rpcConfig.user,
            password: rpcConfig.password
        },
        headers: {
            'Content-Type': 'application/json'
        }
    });
    
    if (response.data.error) {
        throw new Error(response.data.error.message);
    }
    
    return response.data.result;
}

module.exports = { call };
