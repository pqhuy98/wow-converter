import { MapManager } from '@/vendors/wc3maptranslator/extra/map-manager';

const map = new MapManager();
map.load('maps/test-64-tileset.w3x');
map.save('maps/test-64-tileset.w3x');
