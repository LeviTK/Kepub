import { writeFileSync, appendFileSync } from 'node:fs';

writeFileSync('writer.pid', `${process.pid}`);
process.on('SIGTERM', () => appendFileSync('writer-term', 'TERM\n'));
setInterval(() => appendFileSync('heartbeat', '.'), 25);
setTimeout(() => writeFileSync('late-write', 'must not appear after terminal'), 700);
