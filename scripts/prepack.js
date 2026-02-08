#!/usr/bin/env node

/**
 * Prepack script - runs before npm pack/publish
 * Ensures bin directory exists with a placeholder
 */

const fs = require('fs');
const path = require('path');

const binDir = path.join(__dirname, '..', 'bin');
fs.mkdirSync(binDir, { recursive: true });

// Create a placeholder script that tells users to run npm install
const placeholder = `#!/usr/bin/env node
console.error('Ugudu binary not installed. Run: npm install @arcslash/ugudu');
process.exit(1);
`;

fs.writeFileSync(path.join(binDir, 'ugudu'), placeholder, { mode: 0o755 });
console.log('Created placeholder binary for npm package');
