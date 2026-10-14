import { readFileSync } from 'node:fs';
import { createContext, runInContext } from 'node:vm';

import { expect, test } from 'bun:test';

test('WMO camera uses world eye and resets first-person angles', () => {
  const host = readFileSync(new URL('../../../../golang/internal/server/wowsource/assets/capture.js', import.meta.url), 'utf8');
  const context = createContext();
  runInContext(`
    const window={};const info={kind:'wmo'};
    const positions=[],horizontal=[],vertical=[],ticks=[];
    const Module={_setScenePos:(...p)=>positions.push(p),_addHorizontalViewDir:a=>horizontal.push(a),_addVerticalViewDir:a=>vertical.push(a),_gameloop:t=>ticks.push(t)};
  `, context);
  runInContext(host.slice(host.indexOf('function setCamera(')), context);
  runInContext('setCamera({eye:[10,0,0],target:[0,0,0],up:[0,0,1]},false)', context);
  expect(runInContext('JSON.stringify(positions)', context)).toBe('[[-10.0078125,2.00390625,0]]');
  expect(runInContext('horizontal[0]', context)).toBe(0);
  expect(runInContext('ticks.length', context)).toBe(0);
  runInContext('setCamera({eye:[10,0,0],target:[0,0,0],up:[0,0,1]},false)', context);
  expect(runInContext('horizontal[1]', context)).toBe(0);
  runInContext('setCamera({eye:[0,10,0],target:[0,0,0],up:[0,0,1]})', context);
  expect(runInContext('horizontal[2]', context)).toBe(-90);
  expect(runInContext('ticks[0]', context)).toBe(0.0002);
  runInContext('setCamera({eye:[0,0,10],target:[0,0,0],up:[0,1,0]},false)', context);
  expect(runInContext('vertical[3]', context)).toBe(89);
  runInContext('setCamera({eye:[-1030,-1266,9],target:[-1040,-1266,3],up:[0,0,1]},false)', context);
  expect(runInContext('JSON.stringify(positions[4])', context)).toBe('[1029.9921875,1268.00390625,9]');
});
