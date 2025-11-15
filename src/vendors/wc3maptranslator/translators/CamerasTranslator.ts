import { type JsonResult, type WarResult } from '../CommonInterfaces';
import { type Camera } from '../data/Camera';
import { HexBuffer } from '../HexBuffer';
import { W3Buffer } from '../W3Buffer';
import { type Translator } from './Translator';

export class CamerasTranslator implements Translator<Camera[]> {
  private static instance: CamerasTranslator;

  private constructor() {}

  public static getInstance(): CamerasTranslator {
    if (this.instance == null) {
      this.instance = new this();
    }
    return this.instance;
  }

  public static jsonToWar(cameras: Camera[]): WarResult {
    return this.getInstance().jsonToWar(cameras);
  }

  public static warToJson(buffer: Buffer): JsonResult<Camera[]> {
    return this.getInstance().warToJson(buffer);
  }

  public jsonToWar(cameras: Camera[]): WarResult {
    const outBufferToWar = new HexBuffer();

    /*
         * Header
         */
    outBufferToWar.addInt(0); // file version
    outBufferToWar.addInt(cameras?.length || 0); // number of cameras

    /*
         * Body
         */
    cameras?.forEach((camera) => {
      outBufferToWar.addFloat(camera.target.x);
      outBufferToWar.addFloat(camera.target.y);
      outBufferToWar.addFloat(camera.offsetZ);
      outBufferToWar.addFloat(camera.rotation != null ? camera.rotation : 0); // optional
      outBufferToWar.addFloat(camera.aoa);
      outBufferToWar.addFloat(camera.distance);
      outBufferToWar.addFloat(camera.roll != null ? camera.roll : 0);
      outBufferToWar.addFloat(camera.fov);
      outBufferToWar.addFloat(camera.farClipping);
      outBufferToWar.addFloat(camera.nearClipping != null ? camera.nearClipping : 16);
      outBufferToWar.addFloat(camera.localPitch != null ? camera.localPitch : 0);
      outBufferToWar.addFloat(camera.localYaw != null ? camera.localYaw : 0);
      outBufferToWar.addFloat(camera.localRoll != null ? camera.localRoll : 0);
      // Camera name - null-terminated string
      outBufferToWar.addString(camera.name ?? '');
    });

    return {
      errors: [],
      buffer: outBufferToWar.getBuffer(),
    };
  }

  public warToJson(buffer: Buffer): JsonResult<Camera[]> {
    const result: Camera[] = [];
    const outBufferToJSON = new W3Buffer(buffer);

    const _fileVersion = outBufferToJSON.readInt(); // File version
    const numCameras = outBufferToJSON.readInt(); // # of cameras

    for (let i = 0; i < numCameras; i++) {
      const camera: Camera = {
        target: {
          x: 0,
          y: 0,
        },
        offsetZ: 0,
        rotation: 0,
        aoa: 0,
        distance: 0,
        roll: 0,
        fov: 0,
        farClipping: 0,
        nearClipping: 16,
        localPitch: 0,
        localYaw: 0,
        localRoll: 0,
        name: '',
      };

      camera.target.x = outBufferToJSON.readFloat();
      camera.target.y = outBufferToJSON.readFloat();
      camera.offsetZ = outBufferToJSON.readFloat();
      camera.rotation = outBufferToJSON.readFloat();
      camera.aoa = outBufferToJSON.readFloat(); // angle of attack
      camera.distance = outBufferToJSON.readFloat();
      camera.roll = outBufferToJSON.readFloat();
      camera.fov = outBufferToJSON.readFloat(); // field of view
      camera.farClipping = outBufferToJSON.readFloat();
      camera.nearClipping = outBufferToJSON.readFloat();
      camera.localPitch = outBufferToJSON.readFloat();
      camera.localYaw = outBufferToJSON.readFloat();
      camera.localRoll = outBufferToJSON.readFloat();
      // Camera name: null-terminated string
      camera.name = outBufferToJSON.readString();

      result.push(camera);
    }

    return {
      errors: [],
      json: result,
    };
  }
}
