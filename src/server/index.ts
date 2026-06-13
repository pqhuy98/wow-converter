import 'dotenv/config';

import esMain from 'es-main';

import { startConverterServer } from './start';

if (esMain(import.meta)) {
  startConverterServer().catch(console.error);
}
