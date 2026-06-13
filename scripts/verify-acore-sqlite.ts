import { PrismaClient } from '@prisma/client';

import { getCreaturesInTile } from '@/lib/azerothcore-client/creatures';

const prisma = new PrismaClient();

const creatureCount = await prisma.creature.count();
const templateCount = await prisma.creature_template.count();
console.log('sqlite creature rows', creatureCount);
console.log('sqlite template rows', templateCount);

const creatures = await getCreaturesInTile(0, [31, 59]);
console.log('getCreaturesInTile(0, [31, 59])', creatures.length);
if (creatures[0]) {
  console.log('sample', creatures[0].template.name, 'display', creatures[0].model.CreatureDisplayID);
}

await prisma.$disconnect();
process.exit(0);
