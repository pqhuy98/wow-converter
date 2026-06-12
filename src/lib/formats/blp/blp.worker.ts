import { parentPort } from 'worker_threads';

import { convertTextureToBlp, png2BlpJs } from './blp.convert';

export { png2BlpJs };

type MessageTask = {
  type: 'task',
  id: number,
  arrayBuffer: ArrayBuffer,
  byteOffset: number,
  byteLength: number,
  kind: 'png' | 'blp2',
  resizeTo?: { width: number, height: number },
  blpPath: string,
};
type MessageShutdown = {type: 'shutdown'};
type Message = MessageTask | MessageShutdown;

const isValidMessage = (msg: unknown): msg is Message => {
  if (!msg || typeof msg !== 'object' || !('type' in msg)) {
    return false;
  }
  return true;
};

function run() {
  if (!parentPort) {
    return;
  }

  parentPort.on('message', (msg: unknown) => {
    if (!isValidMessage(msg)) return;

    if (msg.type === 'shutdown') {
      parentPort!.postMessage({ type: 'shutdown-ack' });
      process.exit(0);
    }

    if (msg.type !== 'task') return;

    const id: number = msg.id;
    const data = Buffer.from(new Uint8Array(msg.arrayBuffer, msg.byteOffset, msg.byteLength));
    const blpPath: string = msg.blpPath;

    void (async () => {
      try {
        await convertTextureToBlp({ [msg.kind]: data, resizeTo: msg.resizeTo }, blpPath);
        parentPort!.postMessage({ type: 'done', id, success: true });
      } catch (error: unknown) {
        parentPort!.postMessage({
          type: 'done', id, success: false, error: error instanceof Error ? error.message : String(error),
        });
      }
    })();
  });
}

void run();
