require('dotenv').config();
const express = require('express');
const path = require('path');
const rpc = require('./lib/rpc');

const app = express();
const PORT = process.env.PORT || 3000;
const HOST = process.env.HOST || '0.0.0.0';

// View engine
app.set('view engine', 'ejs');
app.set('views', path.join(__dirname, 'views'));

// Static files
app.use(express.static(path.join(__dirname, 'public')));
app.use(express.json());

// Language middleware
app.use((req, res, next) => {
    // Get language from query, cookie, or default
    const lang = req.query.lang || req.cookies?.lang || process.env.DEFAULT_LANG || 'ar';
    req.lang = lang;
    res.locals.lang = lang;
    res.locals.dir = lang === 'ar' ? 'rtl' : 'ltr';
    res.locals.t = require('./locales/' + lang);
    next();
});

// Routes
app.get('/', async (req, res) => {
    try {
        const [info, mining, peers, mempool] = await Promise.all([
            rpc.call('getblockchaininfo'),
            rpc.call('getmininginfo'),
            rpc.call('getconnectioncount'),
            rpc.call('getmempoolinfo')
        ]);
        
        // Get latest blocks
        const latestBlocks = [];
        let height = info.blocks;
        for (let i = 0; i < 10 && height >= 0; i++, height--) {
            const hash = await rpc.call('getblockhash', [height]);
            const block = await rpc.call('getblock', [hash]);
            latestBlocks.push(block);
        }
        
        res.render('index', {
            info,
            mining,
            peers,
            mempool,
            latestBlocks,
            coinName: process.env.COIN_NAME || 'OpenSyria',
            coinSymbol: process.env.COIN_SYMBOL || 'SYL'
        });
    } catch (err) {
        console.error('Error:', err);
        res.render('error', { error: err.message });
    }
});

// Block page
app.get('/block/:hash', async (req, res) => {
    try {
        const block = await rpc.call('getblock', [req.params.hash, 2]);
        res.render('block', { 
            block,
            coinName: process.env.COIN_NAME || 'OpenSyria',
            coinSymbol: process.env.COIN_SYMBOL || 'SYL'
        });
    } catch (err) {
        res.render('error', { error: err.message });
    }
});

// Block by height
app.get('/block-height/:height', async (req, res) => {
    try {
        const hash = await rpc.call('getblockhash', [parseInt(req.params.height)]);
        res.redirect('/block/' + hash);
    } catch (err) {
        res.render('error', { error: err.message });
    }
});

// Transaction page
app.get('/tx/:txid', async (req, res) => {
    try {
        const tx = await rpc.call('getrawtransaction', [req.params.txid, true]);
        res.render('transaction', { 
            tx,
            coinName: process.env.COIN_NAME || 'OpenSyria',
            coinSymbol: process.env.COIN_SYMBOL || 'SYL'
        });
    } catch (err) {
        res.render('error', { error: err.message });
    }
});

// Address page (basic - shows message since we need address indexing)
app.get('/address/:address', async (req, res) => {
    res.render('address', { 
        address: req.params.address,
        coinName: process.env.COIN_NAME || 'OpenSyria',
        coinSymbol: process.env.COIN_SYMBOL || 'SYL'
    });
});

// Search
app.get('/search', async (req, res) => {
    const q = req.query.q?.trim();
    
    if (!q) {
        return res.redirect('/');
    }
    
    // Check if it's a block height
    if (/^\d+$/.test(q)) {
        try {
            const hash = await rpc.call('getblockhash', [parseInt(q)]);
            return res.redirect('/block/' + hash);
        } catch (e) {}
    }
    
    // Check if it's a block hash (64 hex chars)
    if (/^[a-fA-F0-9]{64}$/.test(q)) {
        try {
            await rpc.call('getblock', [q]);
            return res.redirect('/block/' + q);
        } catch (e) {
            // Try as txid
            try {
                await rpc.call('getrawtransaction', [q, true]);
                return res.redirect('/tx/' + q);
            } catch (e2) {}
        }
    }
    
    // Check if it's an address
    if (q.startsWith('syl1') || q.startsWith('F') || q.startsWith('3')) {
        return res.redirect('/address/' + q);
    }
    
    res.render('error', { error: res.locals.t.notFound });
});

// API endpoints
app.get('/api/status', async (req, res) => {
    try {
        const [info, mining, peers] = await Promise.all([
            rpc.call('getblockchaininfo'),
            rpc.call('getmininginfo'),
            rpc.call('getconnectioncount')
        ]);
        res.json({ info, mining, peers });
    } catch (err) {
        res.status(500).json({ error: err.message });
    }
});

app.get('/api/block/:hash', async (req, res) => {
    try {
        const block = await rpc.call('getblock', [req.params.hash, 2]);
        res.json(block);
    } catch (err) {
        res.status(500).json({ error: err.message });
    }
});

app.get('/api/tx/:txid', async (req, res) => {
    try {
        const tx = await rpc.call('getrawtransaction', [req.params.txid, true]);
        res.json(tx);
    } catch (err) {
        res.status(500).json({ error: err.message });
    }
});

// Start server
app.listen(PORT, HOST, () => {
    console.log(`\n🔍 OpenSyria Explorer | مستكشف سوريا المفتوحة`);
    console.log(`   Running at http://${HOST}:${PORT}`);
    console.log(`   Connected to RPC at ${process.env.RPC_HOST}:${process.env.RPC_PORT}\n`);
});
