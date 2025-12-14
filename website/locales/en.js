module.exports = {
  // Site
  siteName: 'OpenSY',
  tagline: "Syria's First Cryptocurrency",
  genesisQuote: '"Dec 8 2024 - Syria Liberated from Assad"',
  genesisCaption: '— Genesis Block Message',
  
  // Navigation
  nav: {
    home: 'Home',
    download: 'Download',
    explorer: 'Explorer',
    community: 'Community',
    docs: 'Documentation',
    github: 'GitHub'
  },
  
  // Hero Section
  hero: {
    title: 'OpenSY',
    subtitle: "Syria's First Blockchain",
    description: 'A decentralized cryptocurrency built for the Syrian community. Forked from Bitcoin Core with Syria-specific customizations.',
    launchDate: 'Launched December 8, 2025',
    getStarted: 'Get Started',
    viewExplorer: 'View Explorer',
    downloadWallet: 'Download Wallet'
  },
  
  // Stats Section
  stats: {
    title: 'Network Statistics',
    blocks: 'Blocks',
    hashrate: 'Network Hashrate',
    peers: 'Connected Peers',
    supply: 'Circulating Supply'
  },
  
  // Features Section
  features: {
    title: 'Why OpenSY?',
    subtitle: 'Built on Bitcoin\'s proven technology with Syria-specific enhancements',
    items: [
      {
        icon: '🔒',
        title: 'Secure',
        description: 'SHA-256 Proof of Work, the same battle-tested algorithm securing Bitcoin for over 15 years.'
      },
      {
        icon: '⚡',
        title: 'Fast',
        description: '2-minute block times for quicker confirmations compared to Bitcoin\'s 10 minutes.'
      },
      {
        icon: '🌍',
        title: 'Decentralized',
        description: 'No central authority. The network is maintained by miners and node operators worldwide.'
      },
      {
        icon: '🇸🇾',
        title: 'Syrian Identity',
        description: 'Address prefix "F" for Freedom, port 9633 from country code +963, symbol SYL.'
      },
      {
        icon: '💎',
        title: 'Fair Launch',
        description: 'No premine, no ICO. All coins are mined through Proof of Work.'
      },
      {
        icon: '📖',
        title: 'Open Source',
        description: 'Fully open source code. Anyone can audit, contribute, or fork.'
      }
    ]
  },
  
  // Specs Section
  specs: {
    title: 'Technical Specifications',
    items: [
      { label: 'Algorithm', value: 'SHA-256 (Proof of Work)' },
      { label: 'Block Time', value: '~2 minutes' },
      { label: 'Block Reward', value: '10,000 SYL' },
      { label: 'Max Supply', value: '21 Billion SYL' },
      { label: 'Halving Interval', value: '~4 years (1,050,000 blocks)' },
      { label: 'Address Prefix', value: 'F (Mainnet), f (Testnet)' },
      { label: 'Bech32 Prefix', value: 'syl (Mainnet), tsyl (Testnet)' },
      { label: 'P2P Port', value: '9633 (Mainnet), 19633 (Testnet)' },
      { label: 'RPC Port', value: '9632 (Mainnet), 19632 (Testnet)' },
      { label: 'Genesis Date', value: 'December 8, 2024 (Syria Liberation Day)' }
    ]
  },
  
  // Quick Start Section
  quickStart: {
    title: 'Quick Start',
    subtitle: 'Get your node running in minutes',
    steps: [
      { title: 'Clone', code: 'git clone https://github.com/opensyria/OpenSY.git' },
      { title: 'Build', code: 'cd OpenSY && cmake -B build && cmake --build build -j$(nproc)' },
      { title: 'Run', code: './build/bin/opensyd -daemon -addnode=node1.opensy.net' },
      { title: 'Check', code: './build/bin/opensy-cli getblockchaininfo' }
    ]
  },
  
  // Footer
  footer: {
    copyright: '© 2025 OpenSY. Open source under MIT License.',
    freesyria: 'سوريا حرة',
    links: {
      title: 'Links',
      github: 'GitHub',
      explorer: 'Block Explorer',
      docs: 'Documentation'
    },
    resources: {
      title: 'Resources',
      whitepaper: 'Whitepaper',
      walletGuide: 'Wallet Guide',
      nodeGuide: 'Node Guide'
    },
    community: {
      title: 'Community',
      twitter: 'Twitter',
      telegram: 'Telegram',
      discord: 'Discord'
    }
  },
  
  // Download Page
  download: {
    title: 'Download OpenSY',
    subtitle: 'Choose your platform',
    buildFromSource: 'Build from Source',
    buildInstructions: 'For maximum security, we recommend building from source:',
    platforms: {
      windows: 'Windows',
      macos: 'macOS',
      linux: 'Linux',
      source: 'Source Code'
    },
    comingSoon: 'Pre-built binaries coming soon. For now, build from source.'
  },
  
  // Docs Page
  docs: {
    title: 'Documentation',
    subtitle: 'Everything you need to know about OpenSY',
    guides: [
      { title: 'Wallet Backup & Restore', description: 'Learn how to safely backup and restore your wallet.', link: 'https://github.com/opensyria/OpenSY/blob/main/docs/WALLET_RESTORE_GUIDE.md' },
      { title: 'Node Operator Guide', description: 'Complete guide to running a full node.', link: 'https://github.com/opensyria/OpenSY/blob/main/docs/NODE_OPERATOR_GUIDE.md' },
      { title: 'Mining Guide', description: 'Start mining OpenSY with your hardware.', link: 'https://github.com/opensyria/OpenSY#mining' }
    ]
  },
  
  // Community Page
  community: {
    title: 'Join the Community',
    subtitle: 'Connect with OpenSY supporters worldwide',
    channels: [
      { name: 'GitHub', description: 'Contribute to development', icon: '💻', link: 'https://github.com/opensyria/OpenSY' },
      { name: 'Twitter/X', description: 'Follow for updates', icon: '🐦', link: '#' },
      { name: 'Telegram', description: 'Chat with the community', icon: '💬', link: '#' },
      { name: 'Discord', description: 'Developer discussions', icon: '🎮', link: '#' }
    ]
  }
};
