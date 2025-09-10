import { chromium } from 'playwright';

const APP_URL = process.env.APP_URL || 'http://127.0.0.1:3001/recents';
const SCREENSHOT_PATH = process.env.SCREENSHOT_PATH || '/workspace/docs/lady-deathwhisper.png';

(async () => {
	const browser = await chromium.launch({ headless: true });
	const page = await browser.newPage({ viewport: { width: 1400, height: 900 } });
	await page.goto(APP_URL, { waitUntil: 'domcontentloaded' });
	await page.waitForSelector('canvas', { timeout: 120000 });
	const canvas = page.locator('canvas').first();
	await canvas.scrollIntoViewIfNeeded();
	await canvas.screenshot({ path: SCREENSHOT_PATH });
	await browser.close();
	console.log('saved', SCREENSHOT_PATH);
})();