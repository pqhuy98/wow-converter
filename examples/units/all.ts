import { main as alliance } from './alliance';
import { main as horde } from './horde';
import { main as lightHammerNpcs } from './light-hammer-npcs';
import { main as neutral } from './neutral';
import { main as scourge } from './scourge';
import { main as scourgeWowhead } from './scourge-wowhead';

async function main() {
  console.log('Starting alliance');
  await alliance();
  console.log('Alliance done');
  console.log('--------------------------------');
  console.log('Starting horde');
  await horde();
  console.log('Starting lightHammerNpcs');
  await lightHammerNpcs();
  console.log('Starting neutral');
  await neutral();
  console.log('Starting scourgeWowhead');
  await scourgeWowhead();
  console.log('Starting scourge');
  scourge();
}

if (require.main === module) {
  void main();
}
